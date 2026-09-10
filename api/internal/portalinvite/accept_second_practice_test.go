package portalinvite_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

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
// Both halves of the walk exist in their own package's tests already; what
// only a shared server can show is that the session the accept mints is
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
// __session cookie -- the request the frontend makes straight after a
// successful accept.
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

// engagementAtPracticeOf reads the Engagement a Client holds and the name
// of the Practice it is at, through the admin connection -- fixture
// bookkeeping, not a read under test.
func engagementAtPracticeOf(t *testing.T, db *testdb.DB, clientID string) (engagementID, practiceName string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT e.id, p.name
		   FROM engagements e
		   JOIN practices p ON p.id = e.practice_id
		  WHERE e.client_id = $1`,
		clientID,
	).Scan(&engagementID, &practiceName); err != nil {
		t.Fatalf("query engagement and practice for client %s: %v", clientID, err)
	}
	return engagementID, practiceName
}

// TestAcceptSecondPracticeInvite_SessionListsBothPractices is #831's own
// scenario, end to end and in the order a person actually meets it:
// AcceptInviteHandler's doc comment says the frontend follows a successful
// accept with GET /api/portal/session to decide where to land, so a Client
// who has just accepted a second Practice's invitation must find that
// Practice's Engagement in the very response meant to land her there --
// beside the one she already held at the first Practice (ADR-0015: one
// Portal Account, many Clients, at most one per Practice).
//
// Neither package could prove this alone. portalinvite's own reuse test
// stops at the claimed row and never reads the session; clientauth's
// multi-Client test starts from two link rows seeded directly, so it never
// exercises the accept path that creates the second one, nor the session
// that accept mints. This walks accept -> cookie -> session read once, so
// a regression in either half -- a resolver that narrows the read back to
// one Client, or an accept that mints a session for the wrong Portal
// Account -- fails here.
func TestAcceptSecondPracticeInvite_SessionListsBothPractices(t *testing.T) {
	db := testdb.New(t)

	// Practice A: the care she already has, reached by a Portal Account
	// that already exists -- signing in with the same address the second
	// Practice will invite.
	const identityUID = "portal_returning-client"
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	clientA, engagementA := testdb.SeedNamedEngagement(t, db, practiceA, "Camille at A", "invited@example.com")
	// SeedPortalAccount plus AttachPortalUser rather than SeedPortalUser:
	// the sign-in address has to be the address Practice B's invitation
	// goes to, which is what makes the accept reuse this Portal Account
	// instead of minting a second one.
	testdb.SeedPortalAccount(t, db, identityUID, "invited@example.com")
	testdb.AttachPortalUser(t, db, identityUID, clientA)

	// Practice B: a second, unrelated Practice with a pending invitation
	// out to the same person.
	clientB, inviteToken := seedPendingPortalInvite(t, db)
	engagementB, practiceBName := engagementAtPracticeOf(t, db, clientB)

	srv := newAcceptThenSessionServer(t, db)
	defer srv.Close()

	accept := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	defer accept.Body.Close()
	if accept.StatusCode != http.StatusOK {
		t.Fatalf("accept status = %d, want %d", accept.StatusCode, http.StatusOK)
	}
	cookie := sessionCookie(accept)
	if cookie == nil {
		t.Fatal("no __session cookie set on successful accept")
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
	// about.
	byID := map[string]string{}
	for _, e := range out.Engagements {
		byID[e.EngagementID] = e.PracticeName
	}
	if byID[engagementA] != "Practice A" {
		t.Fatalf("engagement %q reported Practice %q, want %q", engagementA, byID[engagementA], "Practice A")
	}
	if byID[engagementB] != practiceBName {
		t.Fatalf("engagement %q reported Practice %q, want %q", engagementB, byID[engagementB], practiceBName)
	}

	// The response disambiguates by Practice name, which is the whole
	// point of listing both: two Engagements labeled identically would
	// leave the chooser with nothing to tell them apart.
	names := []string{byID[engagementA], byID[engagementB]}
	if slices.Contains(names, "") {
		t.Fatalf("practice names = %v, want a name on every Engagement", names)
	}
}
