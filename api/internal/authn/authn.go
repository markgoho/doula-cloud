// Package authn owns the browser session: it reads the __session cookie
// off a request, verifies it against Postgres, renews it, and opens the
// request-scoped transaction the rest of the BFF runs in (see store.go
// and ADR-0004). GCP Identity Platform remains the identity provider but
// is reduced to one method, VerifyIDToken, behind a small Verifier
// interface so HTTP middleware can be tested against a fake
// implementation instead of a live Identity Platform project.
package authn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

// SessionCookieName is the name of the session cookie Begin reads. It
// must be exactly this value: since #139 the deployed app reaches the
// BFF through a Firebase Hosting rewrite of /api/** to Cloud Run, and
// Hosting strips every incoming Cookie header on that hop except one
// named exactly "__session". Any other name fails only in production --
// local `vite dev` and the Playwright preview server both proxy /api
// without stripping anything. Defined here, rather than in package
// session, so this package's own credential-reading seam does not
// depend on session, which already depends on authn for the session
// store, Verifier, and BearerToken.
const SessionCookieName = "__session"

// SessionLifetime is how long a newly minted or renewed session is valid
// for. #138 fixes this at 12 hours for both populations. It is both the
// session row's expiry window and the cookie's MaxAge, so the browser
// and Postgres agree on when a session ends.
const SessionLifetime = 12 * time.Hour

// msgInternalError is the body a caller sees when the failure is the
// BFF's own, not theirs -- an unreachable database, not a bad
// credential.
const msgInternalError = "internal error"

// NewSessionCookie builds the *http.Cookie every session cookie -- newly
// minted or renewed -- is sent as, wrapping the session's token with the
// shared name, attributes, and SessionLifetime.
func NewSessionCookie(token string) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionLifetime.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

// BearerToken extracts the token from an HTTP Authorization header of the
// form "Bearer <token>". It returns ("", false) if the header is missing,
// malformed, or contains an empty token.
func BearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, prefix)
	if token == "" {
		return "", false
	}
	return token, true
}

// Begin opens a transaction on db, resolves the caller's credential
// inside it, and returns the transaction and the caller's UID. The
// transaction comes first because the session cookie is verified by
// reading the sessions table, so the credential check and everything the
// handler does afterwards share one connection and one round trip
// budget. It reads the session cookie first, renewing it on the way
// through if it is past half its life (#147); a request with no cookie
// falls back to the Bearer ID token, which costs a network verify
// against Identity Platform while the transaction is held open -- #151
// removes that fallback. It writes the appropriate HTTP error (401 for a
// missing, invalid, ended, or expired credential; 500 for DB connection
// failure), rolls the transaction back, and returns ok=false if any step
// fails. Callers must ensure a returned transaction is rolled back or
// committed.
func Begin(w http.ResponseWriter, r *http.Request, verifier Verifier, db *sql.DB) (*sql.Tx, string, bool) {
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		http.Error(w, msgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}

	uid, ok := credential(w, r, verifier, tx, db)
	if !ok {
		_ = tx.Rollback()
		return nil, "", false
	}

	return tx, uid, true
}

// credential resolves the caller's identity from r: the session cookie
// if one is present, otherwise the Bearer ID token. It writes a 401 and
// returns ok=false if neither credential is present or the one present
// fails verification -- but a session the database could not be asked
// about is a 500, not a 401: an unreachable database must not read as
// "you are signed out". A verified session past half its life is renewed
// before this returns (see renewIfStale); a rejected credential never
// is.
func credential(w http.ResponseWriter, r *http.Request, verifier Verifier, tx *sql.Tx, db *sql.DB) (string, bool) {
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		now := time.Now()
		uid, expiresAt, err := lookupSession(r.Context(), tx, cookie.Value, now)
		if errors.Is(err, errNoSession) {
			http.Error(w, "invalid session", http.StatusUnauthorized)
			return "", false
		}
		if err != nil {
			http.Error(w, msgInternalError, http.StatusInternalServerError)
			return "", false
		}
		renewIfStale(w, r, db, cookie.Value, expiresAt, now)
		return uid, true
	}

	idToken, ok := BearerToken(r)
	if !ok {
		http.Error(w, "missing credential", http.StatusUnauthorized)
		return "", false
	}

	verified, err := verifier.VerifyIDToken(r.Context(), idToken)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return "", false
	}
	return verified.UID, true
}

// renewIfStale extends the session's expiry and re-sets the cookie when
// it is already past half its SessionLifetime (#147). The token value
// does not change, so this is an UPDATE of one row plus a fresh MaxAge
// on the browser's copy -- no Bearer ID token is involved, which is what
// keeps renewal working once #151 removes the Bearer path.
//
// The UPDATE deliberately runs on the pool rather than the request's own
// transaction. Renewal is about the credential, not about whatever the
// handler goes on to do with it, and several handlers (the two
// SessionHandlers, which only read) never commit at all -- riding their
// transaction would send the browser a cookie with a fresh MaxAge while
// expires_at silently stayed put, which is the browser/Postgres
// divergence this design exists to remove. A failed UPDATE leaves the
// existing cookie in place: the session is still valid until its
// current expiry, and the next request past half life tries again.
func renewIfStale(w http.ResponseWriter, r *http.Request, q Querier, token string, expiresAt, now time.Time) {
	if !pastHalfLife(expiresAt, now) {
		return
	}
	if err := renewSession(r.Context(), q, token, now); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return
	}
	http.SetCookie(w, NewSessionCookie(token))
}

// pastHalfLife reports whether a session expiring at expiresAt has less
// than half of a full SessionLifetime left at now. It is written against
// the remaining time rather than a stored issue time on purpose: renewal
// moves expires_at and nothing else, so a midpoint computed from a fixed
// creation time would drift into the past and re-renew on every request.
func pastHalfLife(expiresAt, now time.Time) bool {
	return !now.Before(expiresAt.Add(-SessionLifetime / 2))
}

// VerifiedToken is the identity a Verifier extracts from a valid ID
// token. Identity Platform provides identity only -- no custom claims --
// so this is deliberately just a uid; the BFF resolves authorization
// (which Practice/Client the caller may act as) from the database on
// every request.
type VerifiedToken struct {
	UID string
}

// Verifier checks an Identity Platform ID token. That is the whole of
// what the identity provider is asked to do since ADR-0004: the session
// itself is a row in Postgres (store.go), so nothing here mints,
// verifies, or revokes a session.
type Verifier interface {
	// VerifyIDToken checks a raw ID token and returns the identity it
	// carries, or an error if the token is missing, expired, malformed,
	// or not signed by the configured Identity Platform project. Used
	// only by the session-exchange endpoint and the bootstrap endpoints,
	// which run before a session exists.
	VerifyIDToken(ctx context.Context, idToken string) (*VerifiedToken, error)
}

// FirebaseVerifier verifies tokens via the GCP Identity Platform Admin
// SDK. Identity Platform is built on the Firebase Auth backend, so the
// Firebase Admin SDK for Go is Google's supported client for verifying
// its tokens.
type FirebaseVerifier struct {
	client *auth.Client
}

// NewFirebaseVerifier creates a FirebaseVerifier scoped to the given GCP
// project, using Application Default Credentials to authenticate to the
// Admin SDK.
func NewFirebaseVerifier(ctx context.Context, projectID string) (*FirebaseVerifier, error) {
	// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
		return nil, fmt.Errorf("authn: init firebase app: %w", err)
	}

	// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
	client, err := app.Auth(ctx)
	if err != nil {
		// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
		return nil, fmt.Errorf("authn: init auth client: %w", err)
	}

	// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
	return &FirebaseVerifier{client: client}, nil
}

// VerifyIDToken verifies idToken against Identity Platform and returns
// the caller's uid.
func (v *FirebaseVerifier) VerifyIDToken(ctx context.Context, idToken string) (*VerifiedToken, error) {
	// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
		return nil, fmt.Errorf("authn: verify id token: %w", err)
	}
	// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
	return &VerifiedToken{UID: token.UID}, nil
}
