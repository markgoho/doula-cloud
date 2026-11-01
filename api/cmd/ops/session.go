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

// An operator session ends an hour after its last use, and eight hours
// after it began whatever its use (ADR-0062).
const (
	idleLimit     = time.Hour
	absoluteLimit = 8 * time.Hour
)

// factorTOTP is an authenticator app's provider in a token's
// firebase.sign_in_second_factor claim.
const factorTOTP = "totp"

const (
	msgNotAnOperatorSignIn = "Sign in with an operator login."
	msgSecondFactorNeeded  = "Set up an authenticator app to sign in."
	msgSessionEnded        = "Your session has ended. Sign in again."
)

// Identity is what an operator token says once its tenant has verified
// it. SecondFactor is the provider of the second factor this sign-in
// passed, empty when it passed none. AuthTime is when that sign-in
// happened, in real wall time.
type Identity struct {
	UID          string
	Email        string
	Name         string
	SecondFactor string
	AuthTime     time.Time
}

// Verifier checks an ID token against the operator tenant, and refuses a
// token from any other tenant or from none, which every Staff token is.
type Verifier interface {
	Verify(ctx context.Context, idToken string) (Identity, error)
}

// operator is the signed-in operator, as the session middleware found
// them. StepUpAt is the session's last step-up, nil before its first.
type operator struct {
	ID          string
	Name        string
	Email       string
	StepUpAt    *time.Time
	identityUID string
	token       string
}

type operatorKey struct{}

// sessionView is what GET /api/session and a sign-in answer: the
// operator's own name and email, for the page header.
type sessionView struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// passedSecondFactor reports whether a sign-in's second factor is one
// ops accepts: an authenticator app, or the emulator's phone factor while
// the emulator is the identity provider, since the emulator has no TOTP.
func passedSecondFactor(provider string, acceptEmulatorFactor bool) bool {
	return provider == factorTOTP || (acceptEmulatorFactor && provider == "phone")
}

// signIn exchanges a verified operator token for a session. The first
// sign-in records the operator's name and email; every sign-in is an
// operator_acts row. No idempotency key: it runs before there is a
// session to key one on (docs/api-design.md section 3).
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, sessionCookie(token, int(absoluteLimit.Seconds())))
		apierr.WriteJSON(w, http.StatusOK, view)
	}
}

// mintSession records the operator if this is their first sign-in, the
// session, and the sign_in act, in one transaction on one connection.
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
	if err := recordAct(ctx, tx, operatorID, "sign_in", now); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return sessionView{}, fmt.Errorf("mint operator session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return sessionView{}, fmt.Errorf("mint operator session: %w", err)
	}
	return view, nil
}

func recordAct(ctx context.Context, tx *sql.Tx, operatorID, action string, now time.Time) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO operator_acts (operator_id, action, created_at) VALUES ($1, $2, $3)`,
		operatorID, action, now)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("record %s: %w", action, err)
	}
	return nil
}

// requireSession admits a request whose cookie names a live operator
// session and moves its idle limit on, never past its absolute one. One
// statement on the pool, so no request holds a second connection for it
// (#2432).
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
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

// signOut ends this browser's session and records the sign_out act.
// DELETE is idempotent by method, so it takes no idempotency key.
func signOut(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		op := signedIn(r)
		now := clock.Now(r.Context())
		if err := endSession(r.Context(), db, op, now); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
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
	if err := recordAct(ctx, tx, op.ID, "sign_out", now); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("end operator session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("end operator session: %w", err)
	}
	return nil
}

// sessionCookie is the session's cookie, or with maxAge -1 its removal.
// The name is __session because Firebase Hosting strips every other
// cookie on the /api/** rewrite (authn.SessionCookieName).
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
