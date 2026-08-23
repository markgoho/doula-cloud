// PROTOTYPE -- throwaway. Exercises Shape A (prototype_mount.go) against
// real handlers: billing.GetBalanceHandler (whole-endpoint, ADR-0006 says
// Owner/Admin only) and a roster-shaped stand-in for the Staff roster
// (Owner/Admin only). See prototype_reader_test.go for Shape B and the
// Contract case Shape A cannot express.
package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func seedMembershipWithRoles(t *testing.T, db *testdb.DB, practiceID, staffID string, roles string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES ($1, $2, $3::practice_role[])`,
		practiceID, staffID, roles,
	); err != nil {
		t.Fatalf("seed membership with roles %q: %v", roles, err)
	}
}

// TestGatedRouter_PanicsOnUndeclaredRoute is the guardrail-shaped test the
// AC asks for: a route mounted without a role declaration fails at
// startup, not silently 200s for every Staff member. This is what a table
// walk over Routes() cannot catch by itself -- Get is the only door, and
// it panics before the route is even added to the table.
func TestGatedRouter_PanicsOnUndeclaredRoute(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected Get to panic on an empty role declaration, it did not")
		}
	}()
	g := staffauth.NewGatedRouter(http.NewServeMux(), nil)
	g.Get("/api/practices/{practiceId}/billing", nil, billing.GetBalanceHandler())
}

// TestGatedRouter_RegistryIsWalkable proves every GET mounted through the
// router carries a non-empty declaration -- the table-driven assertion a
// rlsguardrail-shaped test would run in the real router once every read
// endpoint moves onto this seam.
func TestGatedRouter_RegistryIsWalkable(t *testing.T) {
	g := staffauth.NewGatedRouter(http.NewServeMux(), nil)
	g.Get("/api/practices/{practiceId}/billing", []string{"owner", "office_manager"}, billing.GetBalanceHandler())
	g.Get("/api/practices/{practiceId}/clients", staffauth.AnyStaff, http.NotFoundHandler())

	for _, route := range g.Routes() {
		if len(route.Roles) == 0 {
			t.Fatalf("route %q has no role declaration -- a new endpoint that reaches this table without one is exactly the bug ADR-0006 names", route.Pattern)
		}
	}
	if len(g.Routes()) != 2 {
		t.Fatalf("Routes() = %d entries, want 2", len(g.Routes()))
	}
}

// TestGatedRouter_BillingBalance_DoulaForbidden runs the real
// billing.GetBalanceHandler behind the gate and confirms ADR-0006's
// "Credit balance and ledger: Doula ✗" cell holds -- a whole-endpoint
// case Shape A handles cleanly.
func TestGatedRouter_BillingBalance_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Gate Test Practice")

	ownerUID, doulaUID := "gate-owner", "gate-doula"
	ownerID := seedStaff(t, db, ownerUID)
	doulaID := seedStaff(t, db, doulaUID)
	seedMembershipWithRoles(t, db, practiceID, ownerID, "{owner}")
	seedMembershipWithRoles(t, db, practiceID, doulaID, "{doula}")

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	g.Get("/practices/{practiceId}/billing", []string{"owner", "office_manager"}, billing.GetBalanceHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ownerSession := authntest.SeedSession(t, db.App, ownerUID)
	doulaSession := authntest.SeedSession(t, db.App, doulaUID)

	if resp := getWithSession(t, srv.URL+"/practices/"+practiceID+"/billing", ownerSession); resp.StatusCode != http.StatusOK {
		t.Fatalf("owner: status = %d, want 200", resp.StatusCode)
	}
	if resp := getWithSession(t, srv.URL+"/practices/"+practiceID+"/billing", doulaSession); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("doula: status = %d, want 403", resp.StatusCode)
	}
}

func getWithSession(t *testing.T, url, session string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
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
