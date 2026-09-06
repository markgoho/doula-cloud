package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// newPracticeSessionServer mounts this package's whole surface through
// staffauth.Mount, the same call main.go makes on the real GatedRouter.
func newPracticeSessionServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getPracticeSession(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/session", nil)
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

// TestPracticeSessionHandler_ReportsPracticeAndRoles proves the moved
// handler (#836, out of routes_practice.go and into this package)
// answers with the caller's Practice name, roles, and employment-type
// axis -- exercised here through staffauth.Mount, the production
// interface, rather than through api's own route table.
func TestPracticeSessionHandler_ReportsPracticeAndRoles(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-session-owner"
	practiceID := testdb.SeedPractice(t, db, "Practice Session Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{ownerRole, doulaRole}, contractorType)

	srv, session := newPracticeSessionServer(t, db, uid)
	defer srv.Close()

	resp := getPracticeSession(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out staffauth.PracticeSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.PracticeID != practiceID {
		t.Fatalf("practiceId = %q, want %q", out.PracticeID, practiceID)
	}
	if out.PracticeName != "Practice Session Test Practice" {
		t.Fatalf("practiceName = %q, want %q", out.PracticeName, "Practice Session Test Practice")
	}
	if len(out.Roles) != 2 {
		t.Fatalf("roles = %v, want 2 entries", out.Roles)
	}
	if !out.IsContractor {
		t.Fatal("isContractor = false, want true")
	}

	var lastPracticeID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT last_practice_id FROM staff WHERE identity_uid = $1`, uid).Scan(&lastPracticeID); err != nil {
		t.Fatalf("query last_practice_id: %v", err)
	}
	if lastPracticeID != practiceID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, practiceID)
	}
}

// TestPracticeSessionHandler_EmployeeNotContractor proves IsContractor
// reads false for an employee Membership, not just true for a
// contractor.
func TestPracticeSessionHandler_EmployeeNotContractor(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-session-employee"
	practiceID := testdb.SeedPractice(t, db, "Employee Session Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{doulaRole}, employeeType)

	srv, session := newPracticeSessionServer(t, db, uid)
	defer srv.Close()

	resp := getPracticeSession(t, srv, session, practiceID)
	defer resp.Body.Close()

	var out staffauth.PracticeSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.IsContractor {
		t.Fatal("isContractor = true, want false for an employee")
	}
}
