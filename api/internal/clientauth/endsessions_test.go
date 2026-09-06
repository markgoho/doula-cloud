package clientauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// findSessionCookie returns the *last* __session cookie in cookies -- a
// browser applies Set-Cookie headers for one name in the order they
// arrive, so the last one is what actually ends up stored. That matters
// here specifically: a session past half its life gets a renewing
// Set-Cookie from authn.Begin itself before this handler ever runs, so a
// request that both renews and then ends the session carries two, and
// the first one is stale the instant it is written.
func findSessionCookie(t *testing.T, cookies []*http.Cookie) *http.Cookie {
	t.Helper()
	var last *http.Cookie
	for _, c := range cookies {
		if c.Name == authn.SessionCookieName {
			last = c
		}
	}
	if last == nil {
		t.Fatalf("no %s cookie in %v", authn.SessionCookieName, cookies)
	}
	return last
}

func newEndSessionsServer(db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	clientauth.Mount(g, db.App, tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux)
}

func deleteWithSession(t *testing.T, srv *httptest.Server, session string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, srv.URL+"/api/portal/sessions", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if session != "" {
		authntest.AddSessionCookie(req, session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// TestEndAllSessionsHandler_EndsEveryDeviceAndRecordsIt is #618's own
// AC: a Client can sign out of every device from inside the portal, and
// every one of her sessions is gone afterwards -- including the one this
// request itself rode in on -- and the action is recorded (who, when).
func TestEndAllSessionsHandler_EndsEveryDeviceAndRecordsIt(t *testing.T) {
	db := testdb.New(t)
	srv := newEndSessionsServer(db)
	defer srv.Close()

	identifier, clientID, laptop := seedSignedInClient(t, db, "sessions@example.com")
	// A second device signed in under the same Portal Account, so
	// CountFor below actually proves "every device", not just the one
	// this request rode in on.
	authntest.SeedSession(t, db.App, identifier)

	resp := deleteWithSession(t, srv, laptop)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	if got := authntest.CountFor(t, db.App, identifier); got != 0 {
		t.Fatalf("session rows for %s after ending everywhere = %d, want 0", identifier, got)
	}

	// The request's own session (laptop) is one of the ones just ended,
	// so the response must tell the browser to drop its cookie too.
	cleared := findSessionCookie(t, resp.Cookies())
	if cleared.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative (cleared)", cleared.MaxAge)
	}

	var actorClient string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_client_id FROM activity WHERE subject_kind = 'client' AND subject_id = $1 AND action = 'portal_sessions_ended'`,
		clientID,
	).Scan(&actorClient); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actorClient != clientID {
		t.Fatalf("actor_client_id = %q, want %q", actorClient, clientID)
	}
}

// TestEndAllSessionsHandler_ClearsTheCookieEvenWhenRenewedOnTheWayIn
// covers the interaction between #147's renewal and #618's own clear: a
// session past half its life is renewed by authn.Begin itself, with its
// own Set-Cookie, before this handler runs at all -- so the response can
// carry two Set-Cookie headers for __session, and the one that actually
// leaves the person signed out must be the one a browser applies last.
func TestEndAllSessionsHandler_ClearsTheCookieEvenWhenRenewedOnTheWayIn(t *testing.T) {
	db := testdb.New(t)
	srv := newEndSessionsServer(db)
	defer srv.Close()

	identifier := portalaccount.NewIdentifier()
	seedClientWithEngagement(t, db, identifier)
	// 20 of the portal session's 30 days gone -- well past half life.
	token := authntest.SeedSessionAt(t, db.App, identifier, time.Now().Add(-20*24*time.Hour))

	resp := deleteWithSession(t, srv, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	cleared := findSessionCookie(t, resp.Cookies())
	if cleared.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative -- the browser must end up signed out, not holding the mid-request renewal", cleared.MaxAge)
	}
	if got := authntest.CountFor(t, db.App, identifier); got != 0 {
		t.Fatalf("session rows for %s after ending everywhere = %d, want 0", identifier, got)
	}
}

// TestEndAllSessionsHandler_StaffSessionRefused proves a signed-in Staff
// member's session -- perfectly valid, but naming no Portal Account --
// is refused rather than being run through EndAllSessions for whatever
// uid the cookie happened to carry.
func TestEndAllSessionsHandler_StaffSessionRefused(t *testing.T) {
	db := testdb.New(t)
	srv := newEndSessionsServer(db)
	defer srv.Close()

	staffUID := "identity-platform-uid"
	staffSession := authntest.SeedSession(t, db.App, staffUID)

	resp := deleteWithSession(t, srv, staffSession)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	// Refused, not acted on: the Staff member's own session must survive.
	if got := authntest.CountFor(t, db.App, staffUID); got != 1 {
		t.Fatalf("session rows for %s after refusal = %d, want 1", staffUID, got)
	}
}

// TestEndAllSessionsHandler_NoSessionUnauthorized proves this route is
// gated by authn.Begin's own cookie check like every other portal
// identity-level read/write, not left open.
func TestEndAllSessionsHandler_NoSessionUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newEndSessionsServer(db)
	defer srv.Close()

	resp := deleteWithSession(t, srv, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
