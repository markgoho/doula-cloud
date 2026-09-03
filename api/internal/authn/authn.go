// Package authn owns the browser session: it reads the __session cookie
// off a request, verifies it against Postgres, renews it, and opens the
// request-scoped transaction the rest of the BFF runs in (see store.go
// and ADR-0004). GCP Identity Platform remains the identity provider,
// behind two small interfaces: Verifier (VerifyIDToken) and, since #613,
// AccountManager (reading and writing the account records Identity
// Platform still owns as credential store) -- both fakeable so HTTP
// handlers can be tested without a live Identity Platform project.
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

// Begin opens a transaction on db, resolves the caller's session cookie
// inside it, and returns the transaction and the caller's UID. The
// transaction comes first because the session cookie is verified by
// reading the sessions table, so the credential check and everything the
// handler does afterwards share one connection and one round trip
// budget. The session cookie is the only credential it reads: since #151
// no route behind Begin accepts a Bearer ID token, so no request on this
// path costs a network verify against Identity Platform. The three
// bootstrap endpoints, which run before a session exists, use
// BeginBootstrap instead. A cookie past half its life is renewed on the
// way through (#147). Begin writes the appropriate HTTP error (401 for a
// missing, invalid, ended, or expired session; 500 for a DB failure),
// rolls the transaction back, and returns ok=false if any step fails.
// Callers must ensure a returned transaction is rolled back or
// committed.
func Begin(w http.ResponseWriter, r *http.Request, db *sql.DB) (*sql.Tx, string, bool) {
	tx, ok := beginTx(w, r, db)
	if !ok {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		return nil, "", false
	}

	uid, ok := sessionCredential(w, r, tx, db)
	if !ok {
		_ = tx.Rollback()
		return nil, "", false
	}

	return tx, uid, true
}

// BeginBootstrap is Begin for the three endpoints that cannot read a
// session cookie because they run before a session exists: Staff signup,
// Staff invitation acceptance, and Client portal invitation acceptance.
// They read a Bearer ID token and verify it against Identity Platform,
// which costs a network round trip while the transaction is held open --
// acceptable at these three seams because each is a once-per-person
// event, and unacceptable everywhere else, which is why Begin no longer
// offers it. There is no cookie fallback: a caller holding a session has
// no reason to bootstrap.
func BeginBootstrap(w http.ResponseWriter, r *http.Request, verifier Verifier, db *sql.DB) (*sql.Tx, VerifiedToken, bool) {
	tx, ok := beginTx(w, r, db)
	if !ok {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		return nil, VerifiedToken{}, false
	}

	verified, ok := idTokenCredential(w, r, verifier)
	if !ok {
		_ = tx.Rollback()
		return nil, VerifiedToken{}, false
	}

	return tx, verified, true
}

// beginTx opens the request-scoped transaction both entry points run
// their credential check inside, writing a 500 if the database cannot be
// reached at all.
func beginTx(w http.ResponseWriter, r *http.Request, db *sql.DB) (*sql.Tx, bool) {
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		http.Error(w, msgInternalError, http.StatusInternalServerError)
		return nil, false
	}
	return tx, true
}

// sessionCredential resolves the caller's identity from the __session
// cookie. It writes a 401 and returns ok=false if the cookie is absent
// or names no live session -- but a session the database could not be
// asked about is a 500, not a 401: an unreachable database must not read
// as "you are signed out". A verified session past half its life is
// renewed before this returns (see renewIfStale); a rejected one never
// is.
func sessionCredential(w http.ResponseWriter, r *http.Request, tx *sql.Tx, db *sql.DB) (string, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		http.Error(w, "missing credential", http.StatusUnauthorized)
		return "", false
	}

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

// idTokenCredential resolves the caller's identity from a Bearer ID
// token, writing a 401 if the header is absent, malformed, or carries a
// token Identity Platform rejects.
func idTokenCredential(w http.ResponseWriter, r *http.Request, verifier Verifier) (VerifiedToken, bool) {
	idToken, ok := BearerToken(r)
	if !ok {
		http.Error(w, "missing credential", http.StatusUnauthorized)
		return VerifiedToken{}, false
	}

	verified, err := verifier.VerifyIDToken(r.Context(), idToken)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return VerifiedToken{}, false
	}
	return *verified, true
}

// renewIfStale extends the session's expiry and re-sets the cookie when
// it is already past half its SessionLifetime (#147). The token value
// does not change, so this is an UPDATE of one row plus a fresh MaxAge
// on the browser's copy -- no Bearer ID token is involved, which is what
// keeps renewal working now that #151 has removed the Bearer path from
// every route behind Begin.
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
// so the BFF still resolves authorization (which Practice/Client the
// caller may act as) from the database on every request. Email is the
// one reserved claim carried alongside the uid: ADR-0008 has Staff
// invitation acceptance compare the caller's *verified* address against
// the address the Owner invited, and only the identity provider can say
// what that address is. It is empty for an identity signed in through a
// provider that reports none, which accept treats as "cannot prove you
// are the invitee".
type VerifiedToken struct {
	UID   string
	Email string
	// AuthTime is when this ID token's underlying sign-in happened, not
	// when the token was issued or refreshed -- a cached token silently
	// refreshed an hour into a 12-hour session still reports the
	// original sign-in time. #615's step-up re-auth (staffauth.
	// RequireRecentAuth) is the one caller that reads it: it requires a
	// fresh Bearer token whose AuthTime is within a few minutes of now,
	// which only a real, just-completed sign-in can produce.
	AuthTime time.Time
}

// Verifier checks an Identity Platform ID token. Session minting,
// verification, and revocation stay Postgres-side (store.go, ADR-0004);
// AccountManager (account.go) is the other half of what #613 widens
// Identity Platform's role to.
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
	// Identity Platform types every reserved claim except the handful
	// auth.Token names as fields into Claims, so the email claim is a
	// map lookup with a comma-ok assertion, not a struct field. A
	// provider that reports no address leaves this empty rather than
	// failing the verify -- deciding what an address-less identity may
	// do belongs to the handler, not here.
	// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
	email, _ := token.Claims["email"].(string)
	// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
	return &VerifiedToken{UID: token.UID, Email: email, AuthTime: time.Unix(token.AuthTime, 0)}, nil
}
