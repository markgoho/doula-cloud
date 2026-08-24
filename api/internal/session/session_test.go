package session_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/testdb"
)

// seedStaff inserts a Staff row for identityUID -- CreateHandler's
// new-sign-in notice (#345) is Platform voice, Staff only, so a test
// proving it fires needs a Staff row behind the signing-in identity.
func seedStaff(t *testing.T, db *testdb.DB, identityUID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Test Staff', 'staff@example.com')`,
		identityUID,
	); err != nil {
		t.Fatalf("seed staff %q: %v", identityUID, err)
	}
}

func countNewSignInNotices(t *testing.T, db *testdb.DB, identityUID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM session_notice_outbox WHERE identity_uid = $1 AND kind = 'new_signin'`,
		identityUID,
	).Scan(&count); err != nil {
		t.Fatalf("count session_notice_outbox rows: %v", err)
	}
	return count
}

var errBadToken = errors.New("invalid token")

func newServer(t *testing.T, verifier authntest.Verifier) (*httptest.Server, *testdb.DB) {
	t.Helper()
	db := testdb.New(t)
	mux := http.NewServeMux()
	mux.Handle("POST /session", session.CreateHandler(verifier, db.App))
	mux.Handle("DELETE /session", session.EndHandler(db.App))
	return httptest.NewServer(mux), db
}

// postCreate sends a create-session request carrying token as a Bearer
// ID token, or none at all when token is empty.
func postCreate(t *testing.T, srv *httptest.Server, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/session", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// sessionCookie returns the __session cookie from resp, or nil if none
// was set.
func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	return nil
}

func TestCreateHandler_MissingToken(t *testing.T) {
	srv, _ := newServer(t, authntest.Verifier{})
	defer srv.Close()

	resp := postCreate(t, srv, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on missing token: %+v", c)
	}
}

func TestCreateHandler_InvalidToken(t *testing.T) {
	srv, _ := newServer(t, authntest.Verifier{Err: errBadToken})
	defer srv.Close()

	resp := postCreate(t, srv, "bad-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on invalid token: %+v", c)
	}
}

// TestCreateHandler_SessionStoreFailure covers the 500 path: with the
// sessions table gone, creating the session row fails, and the caller
// must be told rather than handed a cookie naming a session that does
// not exist. Dropping the table is how a per-test cloned database forces
// a write failure that no fake Verifier can produce any more, now that
// minting is a database write rather than a call to Identity Platform.
func TestCreateHandler_SessionStoreFailure(t *testing.T) {
	srv, db := newServer(t, authntest.Verifier{UID: "uid-1"})
	defer srv.Close()
	if _, err := db.Admin.ExecContext(t.Context(), `DROP TABLE sessions`); err != nil {
		t.Fatalf("drop sessions: %v", err)
	}

	resp := postCreate(t, srv, "good-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set despite session store failure: %+v", c)
	}
}

// TestCreateHandler_Success drives #144's core case: a valid ID token
// gets a __session cookie with the right attributes, backed by a session
// row. It deliberately never asserts on the cookie's value -- that is an
// opaque random token, and only its digest is ever stored.
func TestCreateHandler_Success(t *testing.T) {
	srv, db := newServer(t, authntest.Verifier{UID: "uid-1"})
	defer srv.Close()

	resp := postCreate(t, srv, "good-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	c := sessionCookie(resp)
	if c == nil {
		t.Fatal("no __session cookie set")
	}
	if c.Value == "" {
		t.Fatal("cookie value is empty")
	}
	if !c.HttpOnly {
		t.Error("cookie is not HttpOnly")
	}
	if !c.Secure {
		t.Error("cookie is not Secure")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}
	wantMaxAge := int(session.Lifetime.Seconds())
	if c.MaxAge != wantMaxAge {
		t.Errorf("MaxAge = %d, want %d", c.MaxAge, wantMaxAge)
	}
	// The cookie is only half of it since ADR-0004: the session is the
	// row, and without one the cookie authenticates nobody.
	if got := authntest.CountFor(t, db.App, "uid-1"); got != 1 {
		t.Errorf("session rows for uid-1 = %d, want 1", got)
	}
}

// TestCreateHandler_QueuesNewSignInNoticeForStaff covers #345: a Staff
// member signing in queues a new-sign-in notice.
func TestCreateHandler_QueuesNewSignInNoticeForStaff(t *testing.T) {
	srv, db := newServer(t, authntest.Verifier{UID: "staff-uid"})
	defer srv.Close()
	seedStaff(t, db, "staff-uid")

	resp := postCreate(t, srv, "good-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := countNewSignInNotices(t, db, "staff-uid"); got != 1 {
		t.Errorf("new-sign-in notices for a signing-in Staff member = %d, want 1", got)
	}
}

// TestCreateHandler_NoNewSignInNoticeForClient covers #345's scope: a
// Client Portal sign-in (no Staff row behind the identity) never queues
// this Platform-voice notice.
func TestCreateHandler_NoNewSignInNoticeForClient(t *testing.T) {
	srv, db := newServer(t, authntest.Verifier{UID: "client-uid"})
	defer srv.Close()

	resp := postCreate(t, srv, "good-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := countNewSignInNotices(t, db, "client-uid"); got != 0 {
		t.Errorf("new-sign-in notices for a Client sign-in = %d, want 0", got)
	}
}

// TestEndHandler_ClearsCookie proves ending a session clears the cookie
// by setting an expiring one under the same name, and deletes the row
// behind it so the token is dead even if the browser kept a copy.
func TestEndHandler_ClearsCookie(t *testing.T) {
	srv, db := newServer(t, authntest.Verifier{})
	defer srv.Close()
	token := authntest.SeedSession(t, db.App, "uid-1")

	resp := deleteWithSession(t, srv, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	c := sessionCookie(resp)
	if c == nil {
		t.Fatal("no __session cookie set")
	}
	if c.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative (expire immediately)", c.MaxAge)
	}
	if got := authntest.CountFor(t, db.App, "uid-1"); got != 0 {
		t.Errorf("session rows for uid-1 = %d, want 0", got)
	}
}

// TestEndHandler_LeavesOtherBrowsersAlone covers the AC that ordinary
// sign-out ends this browser's session only -- ending every session for
// a person is a separate administrative action (#154).
func TestEndHandler_LeavesOtherBrowsersAlone(t *testing.T) {
	srv, db := newServer(t, authntest.Verifier{})
	defer srv.Close()
	laptop := authntest.SeedSession(t, db.App, "uid-1")
	authntest.SeedSession(t, db.App, "uid-1")

	resp := deleteWithSession(t, srv, laptop)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := authntest.CountFor(t, db.App, "uid-1"); got != 1 {
		t.Errorf("session rows for uid-1 = %d, want 1 (the other browser's)", got)
	}
}

// deleteWithSession sends an end-session request carrying token as the
// __session cookie.
func deleteWithSession(t *testing.T, srv *httptest.Server, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, srv.URL+"/session", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// TestEndHandler_NoSessionStillSucceeds proves end-session is idempotent:
// calling it with no session cookie at all still clears the cookie and
// reports success.
func TestEndHandler_NoSessionStillSucceeds(t *testing.T) {
	srv, _ := newServer(t, authntest.Verifier{Err: errBadToken})
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, srv.URL+"/session", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if c := sessionCookie(resp); c == nil {
		t.Fatal("no __session cookie set")
	}
}
