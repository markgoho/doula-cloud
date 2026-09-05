package authn_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
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

// beginRequest runs authn.Begin against db for a request carrying
// whatever cookie and header the caller set up, rolling back whatever
// transaction Begin opens.
func beginRequest(t *testing.T, db *testdb.DB, setup func(*http.Request)) (*httptest.ResponseRecorder, string, bool) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	setup(req)

	tx, uid, _, ok := authn.Begin(rec, req, db.App)
	if ok {
		t.Cleanup(func() { _ = tx.Rollback() })
	}
	return rec, uid, ok
}

// assertUnauthorized fails the test unless rec carries a 401 with message,
// and no Set-Cookie -- a rejected request must never renew a session.
func assertUnauthorized(t *testing.T, rec *httptest.ResponseRecorder, message string) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var out apierr.APIError
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Message != message {
		t.Fatalf("message = %q, want %q", out.Message, message)
	}
	if got := rec.Result().Cookies(); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want none", got)
	}
}

func TestBegin_MissingCredential(t *testing.T) {
	db := testdb.New(t)

	rec, _, ok := beginRequest(t, db, func(*http.Request) {})
	if ok {
		t.Fatal("expected ok=false, got true")
	}
	assertUnauthorized(t, rec, "missing credential")
}

func TestBegin_SessionCookie_Success(t *testing.T) {
	db := testdb.New(t)
	token := authntest.SeedSession(t, db.App, testUID)

	rec, uid, ok := beginRequest(t, db, func(req *http.Request) {
		authntest.AddSessionCookie(req, token)
	})
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if uid != testUID {
		t.Fatalf("uid = %q, want %q", uid, testUID)
	}
	// AC: a fresh session is not renewed, so nothing is re-set.
	if got := rec.Result().Cookies(); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want none", got)
	}
}

// TestBegin_SessionCookie_Rejected covers the three ways a cookie can
// fail to name a live session. All three are deliberately the same
// outcome to the caller: 401, and no renewal.
func TestBegin_SessionCookie_Rejected(t *testing.T) {
	tests := []struct {
		name  string
		token func(t *testing.T, db *testdb.DB) string
	}{
		{
			name:  "unknown token",
			token: func(*testing.T, *testdb.DB) string { return "never-issued" },
		},
		{
			name: "ended session",
			token: func(t *testing.T, db *testdb.DB) string {
				token := authntest.SeedSession(t, db.App, testUID)
				authntest.EndSession(t, db.App, token)
				return token
			},
		},
		{
			name: "expired session",
			token: func(t *testing.T, db *testdb.DB) string {
				return authntest.SeedSessionAt(t, db.App, testUID, time.Now().Add(-authn.SessionLifetime-time.Minute))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testdb.New(t)
			token := tt.token(t, db)

			rec, _, ok := beginRequest(t, db, func(req *http.Request) {
				authntest.AddSessionCookie(req, token)
			})
			if ok {
				t.Fatal("expected ok=false, got true")
			}
			assertUnauthorized(t, rec, "invalid session")
		})
	}
}

func TestBegin_SessionCookie_RenewsPastHalfLife(t *testing.T) {
	db := testdb.New(t)
	// Minted seven hours ago, so five of its twelve hours remain.
	token := authntest.SeedSessionAt(t, db.App, testUID, time.Now().Add(-7*time.Hour))

	rec, uid, ok := beginRequest(t, db, func(req *http.Request) {
		authntest.AddSessionCookie(req, token)
	})
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
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
	// AC: renewal extends the one session rather than minting a second
	// one. Asserted on the store, not on the cookie's value -- a re-mint
	// would leave the original row live alongside a new one.
	if got := authntest.CountFor(t, db.App, testUID); got != 1 {
		t.Errorf("session rows for %s = %d, want 1 -- renewal minted a new session instead of extending", testUID, got)
	}
}

// TestBegin_SessionCookie_RenewalSurvivesRollback proves renewal is a
// database write that outlives the request transaction. The session was
// minted with one hour left, and the transaction Begin opened is rolled
// back -- exactly what staffauth.SessionHandler and
// clientauth.SessionHandler do on every request, since they only read.
// If the UPDATE rode that transaction, the browser would get a cookie
// with a fresh 12-hour MaxAge over a row still expiring within the hour.
func TestBegin_SessionCookie_RenewalSurvivesRollback(t *testing.T) {
	db := testdb.New(t)
	token := authntest.SeedSessionAt(t, db.App, testUID, time.Now().Add(-11*time.Hour))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	authntest.AddSessionCookie(req, token)

	tx, _, _, ok := authn.Begin(rec, req, db.App)
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	var expiresAt time.Time
	if err := db.App.QueryRowContext(t.Context(),
		`SELECT expires_at FROM sessions WHERE identity_uid = $1`, testUID,
	).Scan(&expiresAt); err != nil {
		t.Fatalf("read expires_at: %v", err)
	}
	if remaining := time.Until(expiresAt); remaining < authn.SessionLifetime-time.Minute {
		t.Fatalf("expires_at leaves %v, want a full %v -- the renewal was rolled back", remaining, authn.SessionLifetime)
	}
}

func TestBegin_SessionCookie_NoRenewalBeforeHalfLife(t *testing.T) {
	db := testdb.New(t)
	// Minted one hour ago, so eleven of its twelve hours remain.
	token := authntest.SeedSessionAt(t, db.App, testUID, time.Now().Add(-time.Hour))

	rec, _, ok := beginRequest(t, db, func(req *http.Request) {
		authntest.AddSessionCookie(req, token)
	})
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if got := rec.Result().Cookies(); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want none", got)
	}
}

// TestBegin_SessionCookie_RenewsWithNoBearerTokenAnywhere is the point
// of ADR-0004 and the AC of #151 that guards it: renewal past half life
// still works with no Bearer ID token present anywhere. It asserts on
// expires_at in the store rather than on the Set-Cookie header, so it
// fails if renewal silently stops -- a browser handed a fresh MaxAge
// over a row still expiring on the old schedule is exactly the
// divergence that would otherwise go unnoticed until a session died
// mid-shift.
func TestBegin_SessionCookie_RenewsWithNoBearerTokenAnywhere(t *testing.T) {
	db := testdb.New(t)
	token := authntest.SeedSessionAt(t, db.App, testUID, time.Now().Add(-7*time.Hour))

	rec, _, ok := beginRequest(t, db, func(req *http.Request) {
		authntest.AddSessionCookie(req, token)
	})
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	findSessionCookie(t, rec.Result().Cookies())

	var expiresAt time.Time
	if err := db.App.QueryRowContext(t.Context(),
		`SELECT expires_at FROM sessions WHERE identity_uid = $1`, testUID,
	).Scan(&expiresAt); err != nil {
		t.Fatalf("read expires_at: %v", err)
	}
	if remaining := time.Until(expiresAt); remaining < authn.SessionLifetime-time.Minute {
		t.Fatalf("expires_at leaves %v, want a full %v -- renewal stopped", remaining, authn.SessionLifetime)
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

// TestBegin_BearerTokenAloneIsRejected is the AC of #151: a request
// carrying only a Bearer ID token gets a 401 on a cookie-authenticated
// route. The Verifier here would happily verify the token, so the 401
// can only come from Begin no longer looking at the header at all --
// with a failing Verifier this test would pass for the wrong reason.
func TestBegin_BearerTokenAloneIsRejected(t *testing.T) {
	db := testdb.New(t)

	rec, _, ok := beginRequest(t, db, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer would-verify-fine")
	})
	if ok {
		t.Fatal("expected ok=false, got true -- Begin still accepts a Bearer ID token")
	}
	assertUnauthorized(t, rec, "missing credential")
}

// bootstrapRequest runs authn.BeginBootstrap against db for a request
// the caller sets up, rolling back whatever transaction it opens.
func bootstrapRequest(t *testing.T, db *testdb.DB, verifier authn.Verifier, setup func(*http.Request)) (*httptest.ResponseRecorder, string, bool) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	setup(req)

	tx, verified, ok := authn.BeginBootstrap(rec, req, verifier, db.App)
	if ok {
		t.Cleanup(func() { _ = tx.Rollback() })
	}
	return rec, verified.UID, ok
}

// TestBeginBootstrap_BearerToken_Success covers the AC that the three
// bootstrap endpoints still accept a Bearer ID token: they run before a
// session exists, so there is nothing else for them to read.
func TestBeginBootstrap_BearerToken_Success(t *testing.T) {
	db := testdb.New(t)

	_, uid, ok := bootstrapRequest(t, db, authntest.Verifier{UID: testUID}, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer valid-token")
	})
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if uid != testUID {
		t.Fatalf("uid = %q, want %q", uid, testUID)
	}
}

func TestBeginBootstrap_MissingCredential(t *testing.T) {
	db := testdb.New(t)

	rec, _, ok := bootstrapRequest(t, db, authntest.Verifier{UID: testUID}, func(*http.Request) {})
	if ok {
		t.Fatal("expected ok=false, got true")
	}
	assertUnauthorized(t, rec, "missing credential")
}

func TestBeginBootstrap_InvalidToken(t *testing.T) {
	db := testdb.New(t)

	rec, _, ok := bootstrapRequest(t, db, authntest.Verifier{Err: errors.New("bad token")}, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer invalid-token")
	})
	if ok {
		t.Fatal("expected ok=false, got true")
	}
	assertUnauthorized(t, rec, "invalid token")
}

// TestBeginBootstrap_SessionCookieIsNotACredential pins that the
// bootstrap path is Bearer-only in the other direction too: a live
// session cookie does not stand in for the ID token, because a caller
// who already holds a session has no reason to bootstrap.
func TestBeginBootstrap_SessionCookieIsNotACredential(t *testing.T) {
	db := testdb.New(t)
	token := authntest.SeedSession(t, db.App, testUID)

	rec, _, ok := bootstrapRequest(t, db, authntest.Verifier{UID: testUID}, func(req *http.Request) {
		authntest.AddSessionCookie(req, token)
	})
	if ok {
		t.Fatal("expected ok=false, got true")
	}
	assertUnauthorized(t, rec, "missing credential")
}

// TestMintSession_SweepsExpiredSessions covers the AC that expired rows
// are removed rather than accumulating: minting is where the sweep runs,
// and it takes every expired row with it, not just the caller's own.
func TestMintSession_SweepsExpiredSessions(t *testing.T) {
	db := testdb.New(t)
	authntest.SeedSessionAt(t, db.App, "still-here", time.Now().Add(-time.Hour))
	authntest.SeedSessionAt(t, db.App, "long-gone", time.Now().Add(-authn.SessionLifetime-time.Hour))
	if got := authntest.CountFor(t, db.App, "long-gone"); got != 1 {
		t.Fatalf("expired session rows before mint = %d, want 1", got)
	}

	authntest.SeedSession(t, db.App, "newcomer")

	if got := authntest.CountFor(t, db.App, "long-gone"); got != 0 {
		t.Fatalf("expired session rows after mint = %d, want 0", got)
	}
	if got := authntest.CountFor(t, db.App, "still-here"); got != 1 {
		t.Fatalf("live session rows after mint = %d, want 1 (the sweep took a live session)", got)
	}
}

// TestSessions_AreIndependentPerBrowser covers the AC that ending one
// session leaves that person's other sessions alone -- two sign-ins by
// the same person are two rows.
func TestSessions_AreIndependentPerBrowser(t *testing.T) {
	db := testdb.New(t)
	laptop := authntest.SeedSession(t, db.App, testUID)
	phone := authntest.SeedSession(t, db.App, testUID)

	authntest.EndSession(t, db.App, laptop)

	_, _, ok := beginRequest(t, db, func(req *http.Request) {
		authntest.AddSessionCookie(req, phone)
	})
	if !ok {
		t.Fatal("ending the laptop's session also ended the phone's")
	}
}

// TestEndSession_UnknownTokenSucceeds covers the AC that ending a
// session that is not there is not an error.
func TestEndSession_UnknownTokenSucceeds(t *testing.T) {
	db := testdb.New(t)

	if err := authn.EndSession(t.Context(), db.App, "never-issued"); err != nil {
		t.Fatalf("EndSession on an unknown token: %v", err)
	}
}

// TestEndAllSessions_RemovesEveryDeviceButOnlyThatPerson covers #154's
// core behavior: every session row a person holds is gone -- laptop and
// phone alike -- while a different person's session, seeded alongside
// theirs, is untouched.
func TestEndAllSessions_RemovesEveryDeviceButOnlyThatPerson(t *testing.T) {
	db := testdb.New(t)
	authntest.SeedSession(t, db.App, testUID)
	authntest.SeedSession(t, db.App, testUID)
	const otherUID = "someone-else"
	authntest.SeedSession(t, db.App, otherUID)

	if err := authn.EndAllSessions(t.Context(), db.App, testUID); err != nil {
		t.Fatalf("EndAllSessions: %v", err)
	}

	if got := authntest.CountFor(t, db.App, testUID); got != 0 {
		t.Fatalf("session rows for %q = %d, want 0", testUID, got)
	}
	if got := authntest.CountFor(t, db.App, otherUID); got != 1 {
		t.Fatalf("session rows for %q = %d, want 1 (untouched)", otherUID, got)
	}
}

// TestEndAllSessions_UnknownIdentitySucceeds mirrors EndSession's
// idempotence: ending sessions for someone with no live session is not
// an error.
func TestEndAllSessions_UnknownIdentitySucceeds(t *testing.T) {
	db := testdb.New(t)

	if err := authn.EndAllSessions(t.Context(), db.App, "never-signed-in"); err != nil {
		t.Fatalf("EndAllSessions on an identity with no sessions: %v", err)
	}
}

// TestMintSession_SweepFailure and TestMintSession_InsertFailure cover
// MintSession's two write failures. Neither can be provoked with a fake
// -- minting is a database write since ADR-0004 -- so each breaks the
// sessions table in its own cloned database: dropping it fails the sweep
// before a token is ever generated, while revoking INSERT lets the sweep
// through and fails only the write that matters.
func TestMintSession_SweepFailure(t *testing.T) {
	db := testdb.New(t)
	if _, err := db.Admin.ExecContext(t.Context(), `DROP TABLE sessions`); err != nil {
		t.Fatalf("drop sessions: %v", err)
	}

	cookie, err := authn.MintSession(t.Context(), db.App, testUID, false, time.Now())
	if err == nil {
		t.Fatalf("MintSession succeeded with no sessions table, cookie = %+v", cookie)
	}
}

func TestMintSession_InsertFailure(t *testing.T) {
	db := testdb.New(t)
	if _, err := db.Admin.ExecContext(t.Context(), `REVOKE INSERT ON sessions FROM app_runtime`); err != nil {
		t.Fatalf("revoke insert: %v", err)
	}

	cookie, err := authn.MintSession(t.Context(), db.App, testUID, false, time.Now())
	if err == nil {
		t.Fatalf("MintSession succeeded without INSERT permission, cookie = %+v", cookie)
	}
}

// TestBegin_SessionCookie_LookupFailureIs500 pins the difference between
// "this session is not valid" and "the database could not be asked": an
// unreachable sessions table must not read to the caller as being
// signed out, or a database blip would sign out every browser at once.
func TestBegin_SessionCookie_LookupFailureIs500(t *testing.T) {
	db := testdb.New(t)
	token := authntest.SeedSession(t, db.App, testUID)
	if _, err := db.Admin.ExecContext(t.Context(), `DROP TABLE sessions`); err != nil {
		t.Fatalf("drop sessions: %v", err)
	}

	rec, _, ok := beginRequest(t, db, func(req *http.Request) {
		authntest.AddSessionCookie(req, token)
	})
	if ok {
		t.Fatal("expected ok=false, got true")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestBegin_SessionCookie_RenewalFailureStillServesTheRequest proves a
// failed renewal is not a failed request: the session is still valid
// until its current expiry, so the caller is served and the cookie they
// already hold is left alone rather than re-set with a MaxAge no row
// backs.
func TestBegin_SessionCookie_RenewalFailureStillServesTheRequest(t *testing.T) {
	db := testdb.New(t)
	token := authntest.SeedSessionAt(t, db.App, testUID, time.Now().Add(-7*time.Hour))
	if _, err := db.Admin.ExecContext(t.Context(), `REVOKE UPDATE ON sessions FROM app_runtime`); err != nil {
		t.Fatalf("revoke update: %v", err)
	}

	rec, uid, ok := beginRequest(t, db, func(req *http.Request) {
		authntest.AddSessionCookie(req, token)
	})
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if uid != testUID {
		t.Fatalf("uid = %q, want %q", uid, testUID)
	}
	if got := rec.Result().Cookies(); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want none (renewal failed, so nothing to promise)", got)
	}
}

// TestBegin_ConcurrentRenewalsDoNotBlock pins the one hazard in renewing
// on the pool rather than the request's transaction: the transaction has
// already SELECTed the session row and stays open for the whole handler,
// while renewal UPDATEs that same table on a second connection. A plain
// SELECT takes no row lock, so nothing should wait -- but a wait here
// would be a hung request in production, not a failing assertion, so the
// second request runs under a deadline that turns a block into a
// failure.
func TestBegin_ConcurrentRenewalsDoNotBlock(t *testing.T) {
	db := testdb.New(t)
	stale := time.Now().Add(-7 * time.Hour)
	first := authntest.SeedSessionAt(t, db.App, "first-browser", stale)
	second := authntest.SeedSessionAt(t, db.App, "second-browser", stale)

	firstTx, _, _, ok := authn.Begin(httptest.NewRecorder(), requestWithSession(t.Context(), first), db.App)
	if !ok {
		t.Fatal("first request: expected ok=true, got false")
	}
	// Deliberately left open: this is the state a real handler is in for
	// the whole of its work.
	defer func() { _ = firstTx.Rollback() }()

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	rec := httptest.NewRecorder()

	secondTx, _, _, ok := authn.Begin(rec, requestWithSession(ctx, second), db.App)
	if !ok {
		t.Fatalf("second request blocked or failed behind the first request's open transaction: status %d, body %q", rec.Code, rec.Body.String())
	}
	defer func() { _ = secondTx.Rollback() }()

	findSessionCookie(t, rec.Result().Cookies())
}

// requestWithSession builds a request carrying token as its __session
// cookie, on ctx.
func requestWithSession(ctx context.Context, token string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	authntest.AddSessionCookie(req, token)
	return req
}
