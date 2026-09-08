package idempotency_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/idempotency"
)

// discardMounter stands in for staffauth.GatedRouter, which is what
// actually holds the mux in the BFF. These tests are about the registry,
// not about routing, so it keeps nothing -- and never invokes the
// handler it's given, since Router now wraps every handler in
// staffauth.Middleware before handing it here, and that panics on a nil
// *sql.DB if actually served.
type discardMounter struct{}

func (discardMounter) Write(string, http.Handler) {}

func (discardMounter) GatedWrite(string, []string, http.Handler) {}

// ownerRole and adminRole are named once so golangci-lint's goconst check
// doesn't see the two role literals repeated raw across this file's
// ExemptGated tests.
const (
	ownerRole = "owner"
	adminRole = "admin"
)

// TestRouter_ExemptPanicsWithoutAReason mirrors
// staffauth.TestGatedRouter_ExemptPanicsWithoutAReason: an exemption
// nobody had to justify is not a declaration, so Exempt refuses one.
func TestRouter_ExemptPanicsWithoutAReason(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected Exempt to panic on an empty reason, it did not")
		}
	}()
	rt := idempotency.NewRouter(discardMounter{}, nil)
	rt.Exempt("/api/practices/{practiceId}/website", "", false, http.NotFoundHandler())
}

// TestRouter_RegistryIsWalkable proves every mutating route registered
// through Router carries a declaration -- either Replayable or a non-empty
// Reason -- the table a guardrail-shaped test walks against the real route
// table routes() builds.
func TestRouter_RegistryIsWalkable(t *testing.T) {
	rt := idempotency.NewRouter(discardMounter{}, nil)
	rt.Replayable("POST /api/practices/{practiceId}/clients", false, http.NotFoundHandler())
	rt.Exempt("PUT /api/practices/{practiceId}/website", "PUT replaces the declaration wholesale", false, http.NotFoundHandler())

	routes := rt.Routes()
	if len(routes) != 2 {
		t.Fatalf("Routes() = %d entries, want 2", len(routes))
	}
	for _, route := range routes {
		if !route.Replayable && route.Reason == "" {
			t.Fatalf("route %q is neither Replayable nor carries an exemption reason", route.Pattern)
		}
		if route.Replayable && route.Reason != "" {
			t.Fatalf("route %q is Replayable but also carries a Reason, want it empty", route.Pattern)
		}
	}
}

// TestRouter_ExemptWithoutReplayableAreDistinguishable confirms Replayable
// and Exempt entries are told apart by the Replayable flag, not by
// inspecting Reason -- the same shape staffauth.GatedRoute's Exempt field
// gives GatedRoute.
func TestRouter_ExemptWithoutReplayableAreDistinguishable(t *testing.T) {
	rt := idempotency.NewRouter(discardMounter{}, nil)
	rt.Exempt("DELETE /api/practices/{practiceId}/push-subscriptions", "delete; a retry deletes nothing further", false, http.NotFoundHandler())

	routes := rt.Routes()
	if len(routes) != 1 {
		t.Fatalf("Routes() = %d entries, want 1", len(routes))
	}
	if routes[0].Replayable {
		t.Fatal("Exempt entry reports Replayable = true, want false")
	}
	if routes[0].Reason == "" {
		t.Fatal("Exempt entry carries no reason")
	}
}

// TestRouter_AttachingIsRecordedOnBothDeclarations proves attaching=true
// is visible on the registry regardless of which door registered the
// route -- write_gate_guardrail_test.go's
// TestRoutes_NoEngagementWriteBypassesTheAttachingWriteGate (#836) walks
// this field instead of scanning source for staffauth.AttachingWrite(.
func TestRouter_AttachingIsRecordedOnBothDeclarations(t *testing.T) {
	rt := idempotency.NewRouter(discardMounter{}, nil)
	rt.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/visits", true, http.NotFoundHandler())
	rt.Exempt("PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}",
		"plain UPDATE; a retry is a no-op", true, http.NotFoundHandler())
	rt.Replayable("POST /api/practices/{practiceId}/clients", false, http.NotFoundHandler())

	routes := rt.Routes()
	if len(routes) != 3 {
		t.Fatalf("Routes() = %d entries, want 3", len(routes))
	}
	if !routes[0].Attaching {
		t.Errorf("route %q Attaching = false, want true", routes[0].Pattern)
	}
	if !routes[1].Attaching {
		t.Errorf("route %q Attaching = false, want true", routes[1].Pattern)
	}
	if routes[2].Attaching {
		t.Errorf("route %q Attaching = true, want false", routes[2].Pattern)
	}
}

// TestRouter_ExemptGatedPanicsWithoutAReason mirrors
// TestRouter_ExemptPanicsWithoutAReason: ExemptGated is Exempt plus a
// role declaration, and inherits the same "no reason, no registration"
// refusal.
func TestRouter_ExemptGatedPanicsWithoutAReason(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected ExemptGated to panic on an empty reason, it did not")
		}
	}()
	rt := idempotency.NewRouter(discardMounter{}, nil)
	rt.ExemptGated("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void",
		"", true, []string{ownerRole, adminRole}, http.NotFoundHandler())
}

// TestRouter_ExemptGatedRecordsRoles proves a route registered through
// ExemptGated carries its role declaration on the registry -- #970's
// TestRoutes_ContractWritesDeclareRoles walks this field against the
// real route table routes() builds.
func TestRouter_ExemptGatedRecordsRoles(t *testing.T) {
	rt := idempotency.NewRouter(discardMounter{}, nil)
	rt.ExemptGated("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void",
		"state-guarded", true, []string{ownerRole, adminRole}, http.NotFoundHandler())
	rt.Exempt("PUT /api/practices/{practiceId}/website", "PUT replaces the declaration wholesale", false, http.NotFoundHandler())

	routes := rt.Routes()
	if len(routes) != 2 {
		t.Fatalf("Routes() = %d entries, want 2", len(routes))
	}
	if len(routes[0].Roles) != 2 {
		t.Fatalf("ExemptGated route Roles = %v, want [owner admin]", routes[0].Roles)
	}
	if len(routes[1].Roles) != 0 {
		t.Fatalf("plain Exempt route Roles = %v, want empty", routes[1].Roles)
	}
}

// TestRouter_ReplayableGatedRecordsRoles proves a route registered
// through ReplayableGated carries both its Replayable flag and its role
// declaration on the registry -- #990's
// TestRoutes_PaymentAndRateWritesDeclareRoles walks this field against
// the real route table routes() builds, the same way #970's own
// ExemptGated test does above. Registered twice, attaching=true and
// attaching=false, so both of ReplayableGated's branches run.
func TestRouter_ReplayableGatedRecordsRoles(t *testing.T) {
	rt := idempotency.NewRouter(discardMounter{}, nil)
	rt.ReplayableGated("POST /api/practices/{practiceId}/engagements/{engagementId}/widgets",
		true, []string{ownerRole, adminRole}, http.NotFoundHandler())
	rt.ReplayableGated("POST /api/practices/{practiceId}/invoices/{invoiceId}/payments",
		false, []string{ownerRole, adminRole}, http.NotFoundHandler())

	routes := rt.Routes()
	if len(routes) != 2 {
		t.Fatalf("Routes() = %d entries, want 2", len(routes))
	}
	for _, route := range routes {
		if !route.Replayable {
			t.Errorf("route %q Replayable = false, want true", route.Pattern)
		}
		if route.Reason != "" {
			t.Errorf("route %q Reason = %q, want empty", route.Pattern, route.Reason)
		}
		if len(route.Roles) != 2 {
			t.Errorf("route %q Roles = %v, want [owner admin]", route.Pattern, route.Roles)
		}
	}
	if !routes[0].Attaching {
		t.Error("first route Attaching = false, want true")
	}
	if routes[1].Attaching {
		t.Error("second route Attaching = true, want false")
	}
}
