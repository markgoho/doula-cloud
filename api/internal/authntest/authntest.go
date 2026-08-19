// Package authntest provides a shared test double for authn.Verifier, so
// every package with an HTTP handler behind authn.Begin can build one the
// same way, plus helpers for seeding the Postgres session rows the
// __session cookie now resolves against (ADR-0004).
package authntest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
)

// Verifier is a test double for authn.Verifier. Real Identity Platform
// ID tokens cannot be minted in a unit test, so this reports a fixed
// identity for any token.
type Verifier struct {
	// UID is the caller identity VerifyIDToken reports on success.
	UID string

	// Err, when set, is the error VerifyIDToken returns instead of a
	// verified identity.
	Err error
}

// Verifier satisfies authn.Verifier by value, so tests can pass
// authntest.Verifier{...} directly rather than a pointer to one.
var _ authn.Verifier = Verifier{}

// VerifyIDToken returns v.Err if it is set, and a token carrying v.UID
// otherwise -- whatever idToken is.
func (v Verifier) VerifyIDToken(_ context.Context, _ string) (*authn.VerifiedToken, error) {
	if v.Err != nil {
		return nil, v.Err
	}
	return &authn.VerifiedToken{UID: v.UID}, nil
}

// SeedSession creates a live session for uid and returns the value its
// __session cookie carries, for a test that needs a request to arrive
// already signed in. Ending it -- the revoked-session case authn.Begin
// must reject -- is EndSession below, or just letting SeedSessionAt
// place the expiry in the past.
func SeedSession(t *testing.T, q authn.Querier, uid string) string {
	t.Helper()
	return SeedSessionAt(t, q, uid, time.Now())
}

// SeedSessionAt creates a session as though it had been minted at now,
// so it expires authn.SessionLifetime after that. A now in the past is
// how a test puts a session past half its life (to drive renewal) or
// past its expiry (to drive a 401) without waiting or stubbing a clock.
func SeedSessionAt(t *testing.T, q authn.Querier, uid string, now time.Time) string {
	t.Helper()
	cookie, err := authn.MintSession(t.Context(), q, uid, now)
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("authntest: seed session: %v", err)
	}
	return cookie.Value
}

// EndSession deletes the session token names, so a test can prove a
// request carrying an ended session is rejected.
func EndSession(t *testing.T, q authn.Querier, token string) {
	t.Helper()
	if err := authn.EndSession(t.Context(), q, token); err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("authntest: end session: %v", err)
	}
}

// CountFor returns how many session rows exist for uid, so a test can
// assert that a session was swept, ended, or left alone by counting
// rows rather than reaching for the cookie a request carried.
func CountFor(t *testing.T, q authn.Querier, uid string) int {
	t.Helper()
	var count int
	if err := q.QueryRowContext(t.Context(),
		`SELECT count(*) FROM sessions WHERE identity_uid = $1`, uid,
	).Scan(&count); err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("authntest: count sessions: %v", err)
	}
	return count
}

// AddSessionCookie sets req's Cookie header to carry token as the
// __session cookie. It writes the header directly rather than calling
// req.AddCookie(&http.Cookie{...}), which gosec's G124 flags for lacking
// the response-only attributes (Secure, HttpOnly, SameSite) that a
// request's Cookie header never carries in the first place.
func AddSessionCookie(req *http.Request, token string) {
	req.Header.Set("Cookie", authn.SessionCookieName+"="+token)
}

// Authenticate makes req arrive signed in as uid: it seeds a live
// session and puts that session's token on the request as the __session
// cookie. It is the one call a route test needs to get past authn.Begin,
// which since #151 reads nothing but this cookie.
func Authenticate(t *testing.T, q authn.Querier, req *http.Request, uid string) {
	t.Helper()
	AddSessionCookie(req, SeedSession(t, q, uid))
}
