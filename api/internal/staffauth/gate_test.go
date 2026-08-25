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

func seedMembershipWithRoles(t *testing.T, db *testdb.DB, practiceID, staffID, roles string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, $3::practice_role[], 'employee')`,
		practiceID, staffID, roles,
	); err != nil {
		t.Fatalf("seed membership with roles %q: %v", roles, err)
	}
}

// TestGatedRouter_PanicsOnUndeclaredRoute is the guardrail-shaped test #315
// asks for: a route mounted without a role declaration fails at startup,
// not silently 200s for every Staff member. This is what a table walk
// over Routes() cannot catch by itself -- Get is the only door, and it
// panics before the route is even added to the table.
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
// router carries a non-empty declaration -- the rlsguardrail-shaped
// assertion main_test.go's TestRoutes_EveryDeclaredGETHasRoleDeclaration
// runs against the real route table.
func TestGatedRouter_RegistryIsWalkable(t *testing.T) {
	g := staffauth.NewGatedRouter(http.NewServeMux(), nil)
	g.Get("/api/practices/{practiceId}/billing", []string{ownerRole, adminRole}, billing.GetBalanceHandler())
	g.Get("/api/practices/{practiceId}/clients", staffauth.AnyStaff, http.NotFoundHandler())

	for _, route := range g.Routes() {
		if len(route.Roles) == 0 {
			t.Fatalf("route %q has no role declaration -- a new endpoint that reaches this table without one is exactly the bug ADR-0008 names", route.Pattern)
		}
	}
	if len(g.Routes()) != 2 {
		t.Fatalf("Routes() = %d entries, want 2", len(g.Routes()))
	}
}

// TestGatedRouter_ExemptIsDeclaredInTheSameRegistry is ADR-0008's
// requirement for the pre-account Offer read (#317): a GET mounted
// outside staffauth.Middleware cannot be caught by the startup panic --
// GatedRouter never sees it -- so it has to appear in the same table the
// guardrail test walks, carrying a reason instead of a role list.
func TestGatedRouter_ExemptIsDeclaredInTheSameRegistry(t *testing.T) {
	g := staffauth.NewGatedRouter(http.NewServeMux(), nil)
	g.Get("/api/practices/{practiceId}/billing", []string{ownerRole, adminRole}, billing.GetBalanceHandler())
	g.Exempt("/api/offers/{offerId}", "token-authenticated pre-account read")

	routes := g.Routes()
	if len(routes) != 2 {
		t.Fatalf("Routes() = %d entries, want the mounted GET and the exemption", len(routes))
	}
	exemption := routes[1]
	if !exemption.Exempt || exemption.Reason == "" || len(exemption.Roles) != 0 {
		t.Fatalf("exemption = %+v, want Exempt with a reason and no roles", exemption)
	}
}

// An exemption nobody had to justify is not a declaration, so Exempt
// refuses one -- the same argument AnyStaff makes about a role list.
func TestGatedRouter_ExemptPanicsWithoutAReason(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected Exempt to panic on an empty reason, it did not")
		}
	}()
	g := staffauth.NewGatedRouter(http.NewServeMux(), nil)
	g.Exempt("/api/offers/{offerId}", "")
}

// TestGatedRouter_BillingBalance_DoulaForbidden runs the real
// billing.GetBalanceHandler behind the gate and confirms ADR-0008's
// "Credit balance and ledger: Doula ✗" cell holds.
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
	g.Get("/practices/{practiceId}/billing", []string{ownerRole, adminRole}, billing.GetBalanceHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ownerSession := authntest.SeedSession(t, db.App, ownerUID)
	doulaSession := authntest.SeedSession(t, db.App, doulaUID)

	ownerResp := getWithSession(t, srv.URL+"/practices/"+practiceID+"/billing", ownerSession)
	defer ownerResp.Body.Close()
	if ownerResp.StatusCode != http.StatusOK {
		t.Fatalf("owner: status = %d, want 200", ownerResp.StatusCode)
	}
	doulaResp := getWithSession(t, srv.URL+"/practices/"+practiceID+"/billing", doulaSession)
	defer doulaResp.Body.Close()
	if doulaResp.StatusCode != http.StatusForbidden {
		t.Fatalf("doula: status = %d, want 403", doulaResp.StatusCode)
	}
}

// TestGatedRouter_AnyStaff_OpensToEveryRole proves the AnyStaff sentinel's
// fast path: a Doula, who TestGatedRouter_BillingBalance_DoulaForbidden
// proves is refused a bare-role-declared route, reaches one declared
// staffauth.AnyStaff.
func TestGatedRouter_AnyStaff_OpensToEveryRole(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "AnyStaff Test Practice")

	doulaUID := "any-staff-doula"
	doulaID := seedStaff(t, db, doulaUID)
	seedMembershipWithRoles(t, db, practiceID, doulaID, "{doula}")

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	g.Get("/practices/{practiceId}/clients", staffauth.AnyStaff, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	doulaSession := authntest.SeedSession(t, db.App, doulaUID)
	resp := getWithSession(t, srv.URL+"/practices/"+practiceID+"/clients", doulaSession)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("doula through AnyStaff: status = %d, want 200", resp.StatusCode)
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
