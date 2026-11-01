package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/clock"
)

const (
	idleLimit     = time.Hour
	absoluteLimit = 8 * time.Hour
)

const factorTOTP = "totp"

const (
	msgNotAnOperatorSignIn = "Sign in with an operator login."
	msgSecondFactorNeeded  = "Set up an authenticator app to sign in."
	msgSessionEnded        = "Your session has ended. Sign in again."
)

// Identity is what a verified operator token says.
type Identity struct {
	UID          string
	Email        string
	Name         string
	SecondFactor string
	AuthTime     time.Time
}

// Verifier verifies an ID token against the operator tenant only, so a Staff token is refused.
type Verifier interface {
	Verify(ctx context.Context, idToken string) (Identity, error)
}

type operator struct {
	ID          string
	Name        string
	Email       string
	StepUpAt    *time.Time
	identityUID string
	token       string
}

type operatorKey struct{}

type sessionView struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// The emulator has no TOTP, so its phone factor stands in while it is the identity provider.
func passedSecondFactor(provider string, acceptEmulatorFactor bool) bool {
	return provider == factorTOTP || (acceptEmulatorFactor && provider == "phone")
}

func signIn(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idToken, ok := authn.BearerToken(r)
		if !ok {
			apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, msgNotAnOperatorSignIn, nil)
			return
		}
		identity, err := d.Verifier.Verify(r.Context(), idToken)
		if err != nil {
			apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, msgNotAnOperatorSignIn, nil)
			return
		}
		if !passedSecondFactor(identity.SecondFactor, d.AcceptEmulatorFactor) {
			apierr.Write(w, http.StatusForbidden, apierr.CodeMFARequired, msgSecondFactorNeeded, nil)
			return
		}
		name := identity.Name
		if name == "" {
			name = identity.Email
		}

		token := newSessionToken()
		now := clock.Now(r.Context())
		view, err := mintSession(r.Context(), d.DB, identity, name, token, now)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		http.SetCookie(w, sessionCookie(token, int(absoluteLimit.Seconds())))
		apierr.WriteJSON(w, http.StatusOK, view)
	}
}

func mintSession(ctx context.Context, db *sql.DB, identity Identity, name, token string, now time.Time) (sessionView, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return sessionView{}, fmt.Errorf("mint operator session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO operators (identity_uid, name, email, created_at) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (identity_uid) DO NOTHING`,
		identity.UID, name, identity.Email, now); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return sessionView{}, fmt.Errorf("mint operator session: %w", err)
	}
	var operatorID string
	var view sessionView
	if err := tx.QueryRowContext(ctx,
		`SELECT id, name, email FROM operators WHERE identity_uid = $1`, identity.UID).
		Scan(&operatorID, &view.Name, &view.Email); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return sessionView{}, fmt.Errorf("mint operator session: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO operator_sessions (token_hash, operator_id, idle_expires_at, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		tokenHash(token), operatorID, now.Add(idleLimit), now.Add(absoluteLimit), now); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return sessionView{}, fmt.Errorf("mint operator session: %w", err)
	}
	if _, err := recordAct(ctx, tx, operator{ID: operatorID}, actSignIn, nil, nil, nil, now); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return sessionView{}, fmt.Errorf("mint operator session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return sessionView{}, fmt.Errorf("mint operator session: %w", err)
	}
	return view, nil
}

// One statement on the pool, so no request holds a second connection (#2432).
func requireSession(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(authn.SessionCookieName)
		if err != nil {
			apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, msgSessionEnded, nil)
			return
		}
		now := clock.Now(r.Context())
		op := operator{token: cookie.Value}
		err = db.QueryRowContext(r.Context(),
			`WITH renewed AS (
			     UPDATE operator_sessions
			        SET idle_expires_at = LEAST($3, expires_at)
			      WHERE token_hash = $1 AND idle_expires_at > $2 AND expires_at > $2
			  RETURNING operator_id, step_up_at)
			 SELECT o.id, o.name, o.email, o.identity_uid, renewed.step_up_at
			   FROM renewed JOIN operators o ON o.id = renewed.operator_id`,
			tokenHash(cookie.Value), now, now.Add(idleLimit)).Scan(&op.ID, &op.Name, &op.Email, &op.identityUID, &op.StepUpAt)
		if errors.Is(err, sql.ErrNoRows) {
			http.SetCookie(w, sessionCookie("", -1))
			apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, msgSessionEnded, nil)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), operatorKey{}, op)))
	})
}

func signedIn(r *http.Request) operator {
	op, _ := r.Context().Value(operatorKey{}).(operator)
	return op
}

func session() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		op := signedIn(r)
		apierr.WriteJSON(w, http.StatusOK, sessionView{Name: op.Name, Email: op.Email})
	}
}

func signOut(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		op := signedIn(r)
		now := clock.Now(r.Context())
		if err := endSession(r.Context(), db, op, now); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		http.SetCookie(w, sessionCookie("", -1))
		w.WriteHeader(http.StatusNoContent)
	}
}

func endSession(ctx context.Context, db *sql.DB, op operator, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("end operator session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM operator_sessions WHERE token_hash = $1`, tokenHash(op.token)); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("end operator session: %w", err)
	}
	if _, err := recordAct(ctx, tx, op, actSignOut, nil, nil, nil, now); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("end operator session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("end operator session: %w", err)
	}
	return nil
}

// Firebase Hosting strips every cookie but __session on the /api/** rewrite.
func sessionCookie(token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     authn.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func newSessionToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
