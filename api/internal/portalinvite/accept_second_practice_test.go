package portalinvite_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// newAcceptThenSessionServer mounts the two surfaces this file's one test
// walks in order -- portalinvite's accept and clientauth's session read --
// on a single mux over a single database, the way main.go mounts them.
// Both surfaces are covered in their own package's tests already; what
// only a shared server can show is that the session an accept mints is
// the same credential the session read then answers.
func newAcceptThenSessionServer(t *testing.T, db *testdb.DB) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	portalinvite.Mount(g, ir, db.App, &tasknudge.FakeEnqueuer{})
	clientauth.Mount(g, db.App, tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux)
}

// getPortalSession reads GET /api/portal/session carrying session as the
// __session cookie -- the request AcceptInviteHandler's own doc comment
// says the frontend makes straight after a successful accept.
func getPortalSession(t *testing.T, srv *httptest.Server, session string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/portal/session", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// seedInvitedClientAtNewPractice seeds a Practice of its own, a Client at
// it, and a pending invitation out to invitedAddress -- the state
// InviteHandler leaves behind, for a test that needs two unrelated
// Practices inviting the same person. The package's own
// seedPendingPortalInvite cannot be called twice for that: it names its
// Staff member and its Practice the same way every time, so a second call
// would collide on the Staff identity and leave both Practices sharing
// one name, which is exactly the fact this test asserts on.
func seedInvitedClientAtNewPractice(t *testing.T, db *testdb.DB, practiceName, clientName string) (engagementID, inviteToken string) {
	t.Helper()
	practiceID := testdb.SeedPractice(t, db, practiceName)
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, clientName, invitedAddress)
	return engagementID, insertPendingPortalInvite(t, db, clientID, time.Now().Add(7*24*time.Hour))
}

// TestAcceptSecondPracticeInvite_SessionListsBothPractices is #831's own
// scenario, end to end and in the order a person meets it: she accepts one
// Practice's invitation, later accepts a second Practice's, and then --
// because AcceptInviteHandler's doc comment says the frontend follows a
// successful accept with GET /api/portal/session to decide where to land
// -- must find both Practices' Engagements in the very response meant to
// land her there (ADR-0015: one Portal Account, many Clients, at most one
// per Practice).
//
// Neither package could prove this alone. portalinvite's own reuse test
// stops at the claimed row and never reads a session; clientauth's
// multi-Client test starts from two link rows seeded directly, so it never
// exercises the accept path that creates the second one, nor the session
// that accept mints. Walking accept -> accept -> cookie -> session read
// once is what fails on a regression in either half: a session read that
// narrows back to one Client, or an accept that mints a second Portal
// Account instead of reusing hers.
func TestAcceptSecondPracticeInvite_SessionListsBothPractices(t *testing.T) {
	db := testdb.New(t)

	// Two unrelated Practices, each with an invitation out to the same
	// person -- the only thing they have in common is her address.
	engagementA, tokenA := seedInvitedClientAtNewPractice(t, db, "Practice A", "Camille at A")
	engagementB, tokenB := seedInvitedClientAtNewPractice(t, db, "Practice B", "Camille at B")

	srv := newAcceptThenSessionServer(t, db)
	defer srv.Close()

	first := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: tokenA})
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first accept status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: tokenB})
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second accept status = %d, want %d", second.StatusCode, http.StatusOK)
	}
	cookie := sessionCookie(second)
	if cookie == nil {
		t.Fatal("no __session cookie set on the second accept")
	}

	// The second accept reuses the Portal Account the first one minted
	// rather than minting a second for the same address (#309, ADR-0015).
	// Asserted here because everything below it depends on it: two
	// accounts would mean two identities, and each would honestly see one
	// Practice.
	var accounts int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM portal_accounts WHERE lower(sign_in_address) = lower($1)`, invitedAddress,
	).Scan(&accounts); err != nil {
		t.Fatalf("count portal accounts: %v", err)
	}
	if accounts != 1 {
		t.Fatalf("portal_accounts rows for her address = %d, want 1 reused across both Practices", accounts)
	}

	resp := getPortalSession(t, srv, cookie.Value)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out clientauth.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if len(out.Engagements) != 2 {
		t.Fatalf("engagements = %+v, want both Practices' Engagements", out.Engagements)
	}

	// Keyed by Engagement id rather than compared position by position:
	// the list is ordered by created_at, which is not what this test is
	// about. Each Engagement's Practice name is what lets the chooser
	// label two Engagements from different Practices apart, so both are
	// checked, not just their count.
	byID := map[string]string{}
	for _, e := range out.Engagements {
		byID[e.EngagementID] = e.PracticeName
	}
	if byID[engagementA] != "Practice A" {
		t.Fatalf("engagement %q reported Practice %q, want %q", engagementA, byID[engagementA], "Practice A")
	}
	if byID[engagementB] != "Practice B" {
		t.Fatalf("engagement %q reported Practice %q, want %q", engagementB, byID[engagementB], "Practice B")
	}
}
