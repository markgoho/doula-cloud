// Package authn verifies GCP Identity Platform credentials -- ID tokens
// and the session cookies minted from them -- and manages their
// revocation. It defines a small Verifier interface so HTTP middleware
// can be tested against a fake implementation instead of a live Identity
// Platform project.
package authn

import (
	"context"
	"database/sql"
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
// depend on session, which already depends on authn for Verifier and
// BearerToken.
const SessionCookieName = "__session"

// SessionLifetime is how long a newly minted or renewed session cookie is
// valid for. #138 fixes this at 12 hours for both populations. Defined
// here, alongside SessionCookieName, so Begin's renewal path (#147) and
// package session's minting path build identical cookies without session
// importing back into authn's credential seam.
const SessionLifetime = 12 * time.Hour

// NewSessionCookie builds the *http.Cookie every session cookie -- newly
// minted or renewed -- is sent as, wrapping cookieValue with the shared
// name, attributes, and SessionLifetime.
func NewSessionCookie(cookieValue string) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    cookieValue,
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

// Begin extracts the caller's credential from r, verifies it with
// verifier, and opens a transaction on db. It reads the session cookie
// first, checking revocation on every request since a session cookie is
// long-lived; a request with no cookie falls back to the Bearer ID
// token, which is not revocation-checked, so the app keeps working
// while it migrates off it (removing the fallback is a separate
// ticket). If the session cookie is past half its life, it re-mints and
// sets a fresh one on w (#147). It writes the appropriate HTTP error
// (401 for a missing, invalid, or revoked credential; 500 for DB
// connection failure) and returns ok=false if any step fails. On
// success it returns the open *sql.Tx and the verified caller's UID.
// Callers must ensure the returned transaction is rolled back or
// committed.
func Begin(w http.ResponseWriter, r *http.Request, verifier Verifier, db *sql.DB) (*sql.Tx, string, bool) {
	verified, ok := credential(w, r, verifier)
	if !ok {
		return nil, "", false
	}

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}

	return tx, verified.UID, true
}

// credential resolves the caller's identity from r: the session cookie
// if one is present, otherwise the Bearer ID token. It writes a 401 and
// returns ok=false if neither credential is present or the one present
// fails verification. A verified session cookie past half its life is
// re-minted onto w before this returns (see renewIfStale); a rejected
// credential never is.
func credential(w http.ResponseWriter, r *http.Request, verifier Verifier) (*VerifiedToken, bool) {
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		verified, err := verifier.VerifySessionCookie(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, "invalid session", http.StatusUnauthorized)
			return nil, false
		}
		renewIfStale(w, r, verifier, verified)
		return verified, true
	}

	idToken, ok := BearerToken(r)
	if !ok {
		http.Error(w, "missing credential", http.StatusUnauthorized)
		return nil, false
	}

	verified, err := verifier.VerifyIDToken(r.Context(), idToken)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return nil, false
	}
	return verified, true
}

// renewIfStale re-mints the session cookie on w when verified's session
// cookie is already past half its SessionLifetime (#147): a session
// cookie cannot be exchanged for a fresh one on its own, so this reuses
// the Bearer ID token the frontend still attaches to every request while
// the app migrates off it (#149, #150). A request carrying no Bearer
// token, or a mint failure, leaves the existing cookie in place -- the
// request still proceeds on the cookie already verified, and the next
// request past half life tries again.
func renewIfStale(w http.ResponseWriter, r *http.Request, verifier Verifier, verified *VerifiedToken) {
	if !pastHalfLife(verified.IssuedAt, verified.Expires, time.Now()) {
		return
	}

	idToken, ok := BearerToken(r)
	if !ok {
		return
	}

	cookieValue, err := verifier.MintSessionCookie(r.Context(), idToken, SessionLifetime)
	if err != nil {
		return
	}
	http.SetCookie(w, NewSessionCookie(cookieValue))
}

// pastHalfLife reports whether now has reached the midpoint between
// issuedAt and expires. A zero issuedAt or expires means the Verifier
// did not report the cookie's lifetime, so nothing is renewed.
func pastHalfLife(issuedAt, expires, now time.Time) bool {
	if issuedAt.IsZero() || expires.IsZero() {
		return false
	}
	midpoint := issuedAt.Add(expires.Sub(issuedAt) / 2)
	return !now.Before(midpoint)
}

// VerifiedToken is the identity a Verifier extracts from a valid ID
// token. Identity Platform provides identity only -- no custom claims --
// so this is deliberately just a uid; the BFF resolves authorization
// (which Practice/Client the caller may act as) from the database on
// every request.
type VerifiedToken struct {
	UID string

	// IssuedAt and Expires are the session cookie's own mint time and
	// expiry, as VerifySessionCookie reports them. Both are the zero
	// time for a token VerifyIDToken returns, and for any VerifiedToken
	// a Verifier does not populate them for -- renewIfStale treats a
	// zero value as "unknown lifetime, do not renew".
	IssuedAt time.Time
	Expires  time.Time
}

// Verifier checks Identity Platform credentials and manages the session
// cookies minted from them.
type Verifier interface {
	// VerifyIDToken checks a raw ID token and returns the identity it
	// carries, or an error if the token is missing, expired, malformed,
	// or not signed by the configured Identity Platform project. Used
	// only by the bootstrap endpoints, which run before a session cookie
	// exists.
	VerifyIDToken(ctx context.Context, idToken string) (*VerifiedToken, error)

	// MintSessionCookie exchanges a verified ID token for an opaque
	// session cookie value that expires after expiresIn.
	MintSessionCookie(ctx context.Context, idToken string, expiresIn time.Duration) (string, error)

	// VerifySessionCookie checks a session cookie and returns the
	// identity it carries. Unlike VerifyIDToken, this always checks
	// that the credential has not been revoked, since a session cookie
	// is long-lived and revocation must take effect on the next request.
	// The returned token's IssuedAt and Expires report the cookie's own
	// mint time and expiry, which Begin uses to renew it once it is
	// past half its life (#147).
	VerifySessionCookie(ctx context.Context, sessionCookie string) (*VerifiedToken, error)

	// RevokeRefreshTokens ends every session held by uid, on every
	// device, by revoking that person's Identity Platform refresh
	// tokens. A session cookie already verified before the revocation
	// takes effect only on its next VerifySessionCookie call.
	RevokeRefreshTokens(ctx context.Context, uid string) error
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

// MintSessionCookie exchanges idToken for a session cookie via the Admin
// SDK.
func (v *FirebaseVerifier) MintSessionCookie(ctx context.Context, idToken string, expiresIn time.Duration) (string, error) {
	// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
	cookie, err := v.client.SessionCookie(ctx, idToken, expiresIn)
	if err != nil {
		// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
		return "", fmt.Errorf("authn: mint session cookie: %w", err)
	}
	// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
	return cookie, nil
}

// VerifySessionCookie verifies sessionCookie against Identity Platform,
// checking revocation, and returns the caller's uid and the cookie's own
// mint time and expiry.
func (v *FirebaseVerifier) VerifySessionCookie(ctx context.Context, sessionCookie string) (*VerifiedToken, error) {
	// coverage:ignore reason: requires a real Identity Platform session cookie, not exercised by unit tests
	token, err := v.client.VerifySessionCookieAndCheckRevoked(ctx, sessionCookie)
	if err != nil {
		// coverage:ignore reason: requires a real Identity Platform session cookie, not exercised by unit tests
		return nil, fmt.Errorf("authn: verify session cookie: %w", err)
	}
	// coverage:ignore reason: requires a real Identity Platform session cookie, not exercised by unit tests
	return &VerifiedToken{
		UID:      token.UID,
		IssuedAt: time.Unix(token.IssuedAt, 0),
		Expires:  time.Unix(token.Expires, 0),
	}, nil
}

// RevokeRefreshTokens revokes uid's Identity Platform refresh tokens,
// ending their sessions on every device.
func (v *FirebaseVerifier) RevokeRefreshTokens(ctx context.Context, uid string) error {
	// coverage:ignore reason: requires a real Identity Platform user, not exercised by unit tests
	if err := v.client.RevokeRefreshTokens(ctx, uid); err != nil {
		// coverage:ignore reason: requires a real Identity Platform user, not exercised by unit tests
		return fmt.Errorf("authn: revoke refresh tokens: %w", err)
	}
	// coverage:ignore reason: requires a real Identity Platform user, not exercised by unit tests
	return nil
}
