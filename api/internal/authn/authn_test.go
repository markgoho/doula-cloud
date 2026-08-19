package authn_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
)

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{name: "missing header", header: "", wantToken: "", wantOK: false},
		{name: "wrong scheme", header: "Basic abc123", wantToken: "", wantOK: false},
		{name: "empty token", header: "Bearer ", wantToken: "", wantOK: false},
		{name: "valid token", header: "Bearer abc123", wantToken: "abc123", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			token, ok := authn.BearerToken(req)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if token != tt.wantToken {
				t.Fatalf("token = %q, want %q", token, tt.wantToken)
			}
		})
	}
}

// testUID is the caller identity a fake Verifier reports on success,
// shared across this file's success-path tests.
const testUID = "test-uid"

func TestBegin_MissingCredential(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	tx, uid, ok := authn.Begin(rec, req, authntest.Verifier{}, nil)
	if ok {
		t.Fatalf("expected ok=false, got true (tx=%v, uid=%q)", tx, uid)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Body.String(); got != "missing credential\n" {
		t.Fatalf("body = %q, want %q", got, "missing credential\n")
	}
}

func TestBegin_InvalidToken(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	tx, uid, ok := authn.Begin(rec, req, authntest.Verifier{Err: errors.New("bad token")}, nil)
	if ok {
		t.Fatalf("expected ok=false, got true (tx=%v, uid=%q)", tx, uid)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Body.String(); got != "invalid token\n" {
		t.Fatalf("body = %q, want %q", got, "invalid token\n")
	}
}

func TestBegin_Success(t *testing.T) {
	db := testdb.New(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	tx, uid, ok := authn.Begin(rec, req, authntest.Verifier{UID: testUID}, db.App)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	defer func() { _ = tx.Rollback() }()
	if uid != testUID {
		t.Fatalf("uid = %q, want %q", uid, testUID)
	}
}

func TestBegin_SessionCookie_Success(t *testing.T) {
	db := testdb.New(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	addSessionCookie(req, "a-session-cookie")

	tx, uid, ok := authn.Begin(rec, req, authntest.Verifier{UID: testUID}, db.App)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	defer func() { _ = tx.Rollback() }()
	if uid != testUID {
		t.Fatalf("uid = %q, want %q", uid, testUID)
	}
}

func TestBegin_SessionCookie_Revoked(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	addSessionCookie(req, "a-revoked-cookie")
	req.Header.Set("Authorization", "Bearer a-fresh-id-token")

	tx, uid, ok := authn.Begin(rec, req, authntest.Verifier{Err: authntest.ErrRevoked}, nil)
	if ok {
		t.Fatalf("expected ok=false, got true (tx=%v, uid=%q)", tx, uid)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Body.String(); got != "invalid session\n" {
		t.Fatalf("body = %q, want %q", got, "invalid session\n")
	}
	// AC: a rejected request never re-mints the cookie, even though the
	// Authorization header carries an ID token that renewal could
	// otherwise have used.
	if got := rec.Result().Cookies(); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want none", got)
	}
}

// halfLifeCookie returns the IssuedAt/Expires a Verifier reports so its
// session cookie sits pastHalf (true: past half its 12-hour life, false:
// still within the first half) relative to time.Now().
func halfLifeCookie(pastHalf bool) (issuedAt, expires time.Time) {
	now := time.Now()
	if pastHalf {
		return now.Add(-7 * time.Hour), now.Add(5 * time.Hour)
	}
	return now.Add(-1 * time.Hour), now.Add(11 * time.Hour)
}

func TestBegin_SessionCookie_RenewsPastHalfLife(t *testing.T) {
	db := testdb.New(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	addSessionCookie(req, "a-stale-cookie")
	req.Header.Set("Authorization", "Bearer a-fresh-id-token")

	issuedAt, expires := halfLifeCookie(true)
	tx, uid, ok := authn.Begin(rec, req, authntest.Verifier{UID: testUID, IssuedAt: issuedAt, Expires: expires}, db.App)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	defer func() { _ = tx.Rollback() }()
	if uid != testUID {
		t.Fatalf("uid = %q, want %q", uid, testUID)
	}

	renewed := findSessionCookie(t, rec.Result().Cookies())
	if renewed.MaxAge != int(authn.SessionLifetime.Seconds()) {
		t.Errorf("MaxAge = %d, want %d", renewed.MaxAge, int(authn.SessionLifetime.Seconds()))
	}
	if renewed.Path != "/" || !renewed.HttpOnly || !renewed.Secure || renewed.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie attributes = %+v, want Path=/, HttpOnly, Secure, SameSite=Lax", renewed)
	}
}

func TestBegin_SessionCookie_NoRenewalBeforeHalfLife(t *testing.T) {
	db := testdb.New(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	addSessionCookie(req, "a-fresh-cookie")
	req.Header.Set("Authorization", "Bearer a-fresh-id-token")

	issuedAt, expires := halfLifeCookie(false)
	tx, _, ok := authn.Begin(rec, req, authntest.Verifier{UID: testUID, IssuedAt: issuedAt, Expires: expires}, db.App)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	defer func() { _ = tx.Rollback() }()

	if got := rec.Result().Cookies(); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want none", got)
	}
}

func TestBegin_SessionCookie_NoRenewalWithoutBearerToken(t *testing.T) {
	db := testdb.New(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	addSessionCookie(req, "a-stale-cookie")

	issuedAt, expires := halfLifeCookie(true)
	tx, _, ok := authn.Begin(rec, req, authntest.Verifier{UID: testUID, IssuedAt: issuedAt, Expires: expires}, db.App)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	defer func() { _ = tx.Rollback() }()

	if got := rec.Result().Cookies(); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want none", got)
	}
}

func TestBegin_SessionCookie_NoRenewalOnMintFailure(t *testing.T) {
	db := testdb.New(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	addSessionCookie(req, "a-stale-cookie")
	req.Header.Set("Authorization", "Bearer a-fresh-id-token")

	issuedAt, expires := halfLifeCookie(true)
	tx, uid, ok := authn.Begin(rec, req, authntest.Verifier{UID: testUID, IssuedAt: issuedAt, Expires: expires, MintErr: errors.New("mint failed")}, db.App)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	defer func() { _ = tx.Rollback() }()
	if uid != testUID {
		t.Fatalf("uid = %q, want %q", uid, testUID)
	}

	if got := rec.Result().Cookies(); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want none", got)
	}
}

// findSessionCookie returns the __session cookie among cookies, failing
// the test if none is present.
func findSessionCookie(t *testing.T, cookies []*http.Cookie) *http.Cookie {
	t.Helper()
	for _, c := range cookies {
		if c.Name == authn.SessionCookieName {
			return c
		}
	}
	t.Fatalf("no %s cookie in %v", authn.SessionCookieName, cookies)
	return nil
}

func TestBegin_SessionCookiePreferredOverBearer(t *testing.T) {
	db := testdb.New(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	addSessionCookie(req, "a-session-cookie")

	// A Verifier that only succeeds via VerifySessionCookie proves Begin
	// checked the cookie rather than falling back to the Bearer header
	// even though both are present.
	verifier := cookieOnlyVerifier{uid: testUID}

	tx, uid, ok := authn.Begin(rec, req, verifier, db.App)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	defer func() { _ = tx.Rollback() }()
	if uid != testUID {
		t.Fatalf("uid = %q, want %q", uid, testUID)
	}
}

// addSessionCookie sets req's Cookie header directly rather than
// req.AddCookie(&http.Cookie{...}), which gosec's G124 flags for lacking
// response-only attributes (Secure, HttpOnly, SameSite) that a request's
// Cookie header never carries in the first place.
func addSessionCookie(req *http.Request, value string) {
	req.Header.Set("Cookie", authn.SessionCookieName+"="+value)
}

// cookieOnlyVerifier is a Verifier whose VerifyIDToken always fails, so a
// test using it can prove a code path never reached the Bearer fallback.
type cookieOnlyVerifier struct {
	uid string
}

func (v cookieOnlyVerifier) VerifyIDToken(context.Context, string) (*authn.VerifiedToken, error) {
	return nil, errors.New("cookieOnlyVerifier: VerifyIDToken must not be called")
}

func (v cookieOnlyVerifier) MintSessionCookie(context.Context, string, time.Duration) (string, error) {
	return "", errors.New("cookieOnlyVerifier: MintSessionCookie must not be called")
}

func (v cookieOnlyVerifier) VerifySessionCookie(context.Context, string) (*authn.VerifiedToken, error) {
	return &authn.VerifiedToken{UID: v.uid}, nil
}

func (v cookieOnlyVerifier) RevokeRefreshTokens(context.Context, string) error {
	return errors.New("cookieOnlyVerifier: RevokeRefreshTokens must not be called")
}
