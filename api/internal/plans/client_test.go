package plans_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newPortalServer mounts the same route main.go wires up for the
// Client-portal Birth Plan view, behind clientauth.Middleware.
func newPortalServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	plans.Mount(g, ir, db.App)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getClientBirthPlan(t *testing.T, srv *httptest.Server, session string, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/portal/engagements/"+engagementID+"/birth-plan", nil)
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

func postAcknowledgeBirthPlan(t *testing.T, srv *httptest.Server, session string, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/portal/engagements/"+engagementID+"/birth-plan/acknowledge", nil)
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

func TestClientGetBirthPlanHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-viewing-birth-plan"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","options":["Home","Hospital"],"order":0}]`,
		`{"location":"Hospital"}`,
	)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out plans.InstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.EngagementID != engagementID || out.PlanType != birthPlanType || out.Answers["location"] != "Hospital" {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestClientGetBirthPlanHandler_NoInstanceYet404(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-no-birth-plan-yet"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestClientGetBirthPlanHandler_CarePlanNeverReturned proves the
// HTTP-level half of "Care Plan stays completely unreachable from the
// client-portal role": a Care Plan instance exists on the Engagement, but
// with no Birth Plan instance, the endpoint 404s rather than falling back
// to whatever plan_instances row it can find.
func TestClientGetBirthPlanHandler_CarePlanNeverReturned(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-only-care-plan"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	seedInstance(t, db, engagementID, carePlanType,
		`[{"id":"f1","type":"short_text","label":"Name","order":0}]`, `{"f1":"secret"}`,
	)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestClientGetBirthPlanHandler_PostpartumEngagementRefused proves #311's
// AC directly: the endpoint refuses a postpartum-only Engagement
// independently of the portal's own nav/hub gating -- kind = postpartum
// never offers a Birth Plan, even when reached straight by URL.
func TestClientGetBirthPlanHandler_PostpartumEngagementRefused(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-postpartum-only"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Jordan Client", "jordan@example.com", "postpartum")
	testdb.SeedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestClientGetBirthPlanHandler_OtherClientsEngagementRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-not-linked"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	_, otherEngagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Other Client", "other@example.com")
	clientID, _ := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientBirthPlan(t, srv, session, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestClientAcknowledgeBirthPlanHandler_Success is #301's central AC: a
// Client can confirm she has read her Birth Plan, the write lands on the
// row, and it is attributed to her in the audit trail (ADR-0022).
func TestClientAcknowledgeBirthPlanHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-acknowledging-birth-plan"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","options":["Home","Hospital"],"order":0}]`,
		`{"location":"Hospital"}`,
	)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := postAcknowledgeBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out plans.InstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ClientAcknowledgedAt == nil {
		t.Fatalf("clientAcknowledgedAt = nil, want it set by the acknowledge write")
	}

	getResp := getClientBirthPlan(t, srv, session, engagementID)
	defer getResp.Body.Close()
	var getOut plans.InstanceResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getOut.ClientAcknowledgedAt == nil {
		t.Fatalf("a subsequent GET clientAcknowledgedAt = nil, want the acknowledgement to persist")
	}

	var action, actorKind, actorClientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_kind::text, actor_client_id FROM activity
		 WHERE subject_kind = 'engagement' AND subject_id = $1 ORDER BY created_at DESC LIMIT 1`,
		engagementID,
	).Scan(&action, &actorKind, &actorClientID); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if action != "birth_plan_acknowledged" || actorKind != "client" || actorClientID != clientID {
		t.Fatalf("activity row = (action=%q, actorKind=%q, actorClientID=%q), want (birth_plan_acknowledged, client, %q)", action, actorKind, actorClientID, clientID)
	}
}

func TestClientAcknowledgeBirthPlanHandler_NoInstanceYet404(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-acknowledging-no-birth-plan"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := postAcknowledgeBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestClientAcknowledgeBirthPlanHandler_OtherClientsEngagementRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-acknowledging-not-linked"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	_, otherEngagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Other Client", "other@example.com")
	clientID, _ := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := postAcknowledgeBirthPlan(t, srv, session, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
