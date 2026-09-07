package clientauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// newSessionServer mounts this package's whole surface through
// clientauth.Mount, and seeds a live session for uid, returning the token
// its __session cookie carries.
func newSessionServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	clientauth.Mount(g, db.App, tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getSession(t *testing.T, srv *httptest.Server, session string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/portal/session", nil)
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

func TestSessionHandler_MissingCookie(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newSessionServer(t, db, "no-cookie-sent")
	defer srv.Close()

	resp := getSession(t, srv, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestSessionHandler_UnknownSession covers a cookie that names no live
// session -- the shape a stale or forged cookie arrives in.
func TestSessionHandler_UnknownSession(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newSessionServer(t, db, "irrelevant")
	defer srv.Close()

	resp := getSession(t, srv, "never-issued")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestSessionHandler_UnknownClient(t *testing.T) {
	db := testdb.New(t)
	srv, session := newSessionServer(t, db, "no-such-client")
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestSessionHandler_SingleEngagement(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "single-engagement-client"
	_, engagementID := seedClientWithEngagement(t, db, identityUID)

	srv, session := newSessionServer(t, db, identityUID)
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out clientauth.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Engagements) != 1 || out.Engagements[0].EngagementID != engagementID {
		t.Fatalf("engagements = %+v, want single engagement %q", out.Engagements, engagementID)
	}
	if out.Engagements[0].PracticeName != "Test Practice" {
		t.Fatalf("practiceName = %q, want %q", out.Engagements[0].PracticeName, "Test Practice")
	}
	// #619: the change screen has to show her which mailbox signs her in
	// before it asks for another, and this is the read that tells it.
	// seedPortalUser mints the address from the identifier.
	if out.SignInAddress != identityUID+"@example.com" {
		t.Fatalf("signInAddress = %q, want %q", out.SignInAddress, identityUID+"@example.com")
	}
}

func TestSessionHandler_MultipleEngagements(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "multi-engagement-client"
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	clientID, engagementA := testdb.SeedEngagementInStatus(t, db, practiceA, "Shared Client", "shared@example.com", "intake")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, 'intake', 'birth')`,
		clientID, practiceB,
	); err != nil {
		t.Fatalf("seed second engagement: %v", err)
	}
	testdb.SeedPortalUser(t, db, identityUID, clientID)

	srv, session := newSessionServer(t, db, identityUID)
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	var out clientauth.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Engagements) != 2 {
		t.Fatalf("expected 2 engagements, got %d", len(out.Engagements))
	}
	if out.Engagements[0].EngagementID != engagementA {
		t.Fatalf("expected first engagement (by created_at) = %q, got %q", engagementA, out.Engagements[0].EngagementID)
	}
}

// TestSessionHandler_MultipleClients is #312's own scenario, distinct from
// TestSessionHandler_MultipleEngagements above: ADR-0015's "a Portal
// Account reaches many Clients, at most one per Practice" means two
// separate clients rows, each with its own client_portal_users link to
// the same identity -- not one Client whose Engagements happen to carry
// different practice_id values. Before #312, setIdentityAndResolveClient
// picked one client_portal_users row arbitrarily via QueryRow and this
// Engagement was invisible.
func TestSessionHandler_MultipleClients(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "multi-client-portal-account"
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	clientA, engagementA := testdb.SeedEngagementInStatus(t, db, practiceA, "Camille at A", "camille-a@example.com", "completed")
	clientB, engagementB := testdb.SeedEngagementInStatus(t, db, practiceB, "Camille at B", "camille-b@example.com", "active")
	testdb.SeedPortalUser(t, db, identityUID, clientA)
	testdb.AttachPortalUser(t, db, identityUID, clientB)

	srv, session := newSessionServer(t, db, identityUID)
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out clientauth.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Engagements) != 2 {
		t.Fatalf("expected 2 engagements across 2 Practices, got %+v", out.Engagements)
	}
	ids := map[string]string{out.Engagements[0].EngagementID: out.Engagements[0].PracticeName, out.Engagements[1].EngagementID: out.Engagements[1].PracticeName}
	if ids[engagementA] != "Practice A" || ids[engagementB] != "Practice B" {
		t.Fatalf("engagements = %+v, want %q at Practice A and %q at Practice B", out.Engagements, engagementA, engagementB)
	}
	// A completed Engagement stays listed and reachable forever
	// (ADR-0015) -- it must not be filtered out here just because its
	// status has moved on.
	var statuses []string
	for _, e := range out.Engagements {
		statuses = append(statuses, e.Status)
	}
	if !slices.Contains(statuses, "completed") {
		t.Fatalf("expected the completed Engagement to still be listed, got statuses %v", statuses)
	}
}

// TestSessionHandler_MultipleClients_Isolation is the cross-Client read's
// own database-level backstop (ADR-0015, engagements_identity_visibility,
// 00082): a second Portal Account, unrelated to the first, must see only
// the Engagement her own identity reaches.
func TestSessionHandler_MultipleClients_Isolation(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	clientA, engagementA := testdb.SeedEngagementInStatus(t, db, practiceA, "Camille at A", "camille-a@example.com", "active")
	_, engagementB := testdb.SeedEngagementInStatus(t, db, practiceB, "Someone Else", "someone-else@example.com", "active")
	testdb.SeedPortalUser(t, db, "isolated-portal-account", clientA)

	srv, session := newSessionServer(t, db, "isolated-portal-account")
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	var out clientauth.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Engagements) != 1 || out.Engagements[0].EngagementID != engagementA {
		t.Fatalf("engagements = %+v, want only %q -- Practice B's %q must not leak", out.Engagements, engagementA, engagementB)
	}
}

// TestSessionHandler_NoStaffOnlyFact locks the response shape to exactly
// the three Client-facing fields the Agent Brief names -- the Engagement's
// id, the Practice's name, and the raw status (clientRegister.ts labels it
// client-side, per #212). kind, birthOutcome and endingReason are
// staff-only (ADR-0015) and must never reach this DTO.
func TestSessionHandler_NoStaffOnlyFact(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "no-staff-fact-client"
	_, engagementID := seedClientWithEngagement(t, db, identityUID)

	srv, session := newSessionServer(t, db, identityUID)
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	engagements, ok := out["engagements"].([]any)
	if !ok || len(engagements) != 1 {
		t.Fatalf("engagements = %+v, want one entry for %q", out["engagements"], engagementID)
	}
	entry, ok := engagements[0].(map[string]any)
	if !ok {
		t.Fatalf("engagement entry = %+v, want an object", engagements[0])
	}
	wantKeys := map[string]bool{"engagementId": true, "practiceName": true, "status": true}
	for key := range entry {
		if !wantKeys[key] {
			t.Fatalf("engagement entry carries %q -- staff-only facts (kind, birthOutcome, endingReason) must never reach the portal root read", key)
		}
	}
}
