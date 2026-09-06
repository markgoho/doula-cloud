package session_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/testdb"
)

// postCreateWithSession signs in as Staff while the browser already
// holds the session named by cookieToken, confirming the eviction or not
// per confirmed.
func postCreateWithSession(t *testing.T, srv *httptest.Server, idToken, cookieToken string, confirmed bool) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/session", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+idToken)
	authntest.AddSessionCookie(req, cookieToken)
	if confirmed {
		req.Header.Set("X-Confirmed", "true")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// readCode reads docs/api-design.md section 7's machine-readable code
// off a refusal -- what tells this 409 apart from any other, rather than
// matching its English (#692).
func readCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return out.Code
}

// staffUID is the identity every test here signs in as -- one name so
// the seeded Staff row, the verifier and the session count all agree.
const staffUID = "staff-uid"

func countSessions(t *testing.T, db *testdb.DB, uid string) int {
	t.Helper()
	return authntest.CountFor(t, db.App, uid)
}

// A doula who is also a Client, signing in to her Practice on the laptop
// where her portal session is live, is told what it costs before
// anything happens -- and nothing happens.
func TestCreateHandler_UnconfirmedPortalSessionWarnsAndMintsNothing(t *testing.T) {
	srv, db := newServer(t, authntest.Verifier{UID: staffUID})
	defer srv.Close()
	portalUID := portalaccount.NewIdentifier()
	portalToken := authntest.SeedSession(t, db.App, portalUID)

	resp := postCreateWithSession(t, srv, "good-token", portalToken, false)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	if got := readCode(t, resp); got != string(authn.EvictionUnconfirmed) {
		t.Fatalf("code = %q, want %q", got, authn.EvictionUnconfirmed)
	}
	if sessionCookie(resp) != nil {
		t.Error("a refused sign-in set a session cookie")
	}
	if got := countSessions(t, db, staffUID); got != 0 {
		t.Errorf("Staff session rows = %d, want 0 -- nothing may mint before she confirms", got)
	}
	if got := countSessions(t, db, portalUID); got != 1 {
		t.Errorf("portal session rows = %d, want 1 -- the refused sign-in must leave it alone", got)
	}
}

func TestCreateHandler_ConfirmedEvictsThePortalSession(t *testing.T) {
	srv, db := newServer(t, authntest.Verifier{UID: staffUID})
	defer srv.Close()
	portalUID := portalaccount.NewIdentifier()
	portalToken := authntest.SeedSession(t, db.App, portalUID)

	resp := postCreateWithSession(t, srv, "good-token", portalToken, true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if sessionCookie(resp) == nil {
		t.Fatal("no __session cookie set")
	}
	if got := countSessions(t, db, staffUID); got != 1 {
		t.Errorf("Staff session rows = %d, want 1", got)
	}
	// Deleted, not left to expire: an evicted token that still verifies
	// is the defect #610's AC names.
	if got := countSessions(t, db, portalUID); got != 0 {
		t.Errorf("portal session rows = %d, want 0", got)
	}
	// A Client's eviction sends no mail -- sessionnotice.QueueSessionEvicted
	// records why.
	if got := countEvictionNotices(t, db, portalUID); got != 0 {
		t.Errorf("eviction notices for an evicted Client = %d, want 0", got)
	}
}

// A live Staff session is not a cross-population eviction: signing in
// again as yourself replaces your own session, which is what a
// re-sign-in has always done and carries nothing to warn about.
func TestCreateHandler_LiveStaffSessionSignsStraightThrough(t *testing.T) {
	srv, db := newServer(t, authntest.Verifier{UID: staffUID})
	defer srv.Close()
	existing := authntest.SeedSession(t, db.App, "other-staff-uid")

	resp := postCreateWithSession(t, srv, "good-token", existing, false)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := countSessions(t, db, "other-staff-uid"); got != 1 {
		t.Errorf("the other Staff session rows = %d, want 1 -- a same-population sign-in evicts nothing", got)
	}
}

func countEvictionNotices(t *testing.T, db *testdb.DB, identityUID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM session_notice_outbox WHERE identity_uid = $1 AND kind = 'session_evicted'`,
		identityUID,
	).Scan(&count); err != nil {
		t.Fatalf("count session_notice_outbox rows: %v", err)
	}
	return count
}
