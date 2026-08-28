package plans_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/testdb"
)

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection -- mirrors
// message/helpers_test.go's seedClientEngagement (package-local, not
// exported across packages).
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID, name, email string) (clientID, engagementID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, $2, $3) RETURNING id`,
		practiceID, name, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}

// seedPortalUser links identityUID to clientID via client_portal_users,
// using the superuser Admin connection.
func seedPortalUser(t *testing.T, db *testdb.DB, identityUID, clientID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		identityUID, clientID,
	); err != nil {
		t.Fatalf("seed client_portal_users: %v", err)
	}
}

// newPortalServer mounts the same route main.go wires up for the
// Client-portal Birth Plan view, behind clientauth.Middleware.
func newPortalServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /portal/engagements/{engagementId}/birth-plan",
		clientauth.Middleware(db.App)(plans.ClientGetBirthPlanHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getClientBirthPlan(t *testing.T, srv *httptest.Server, session string, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/"+engagementID+"/birth-plan", nil)
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
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
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
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)

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
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
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

func TestClientGetBirthPlanHandler_OtherClientsEngagementRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-not-linked"
	practiceID := seedPractice(t, db, "Practice")
	_, otherEngagementID := seedClientEngagement(t, db, practiceID, "Other Client", "other@example.com")
	clientID, _ := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientBirthPlan(t, srv, session, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
