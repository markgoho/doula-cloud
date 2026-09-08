package main

import "testing"

// bookingWriteRoutes are the nine writes #1016 exists to gate: the ones
// #970 and #990 left behind. Each was mounted through the role-free
// idempotency.Router.Exempt or Replayable door while checking its role
// only inside the handler (staffauth.RequireOwner /
// RequireOwnerOrAdmin) -- correct at runtime, but invisible to the
// startup panic and to a registry-walking test, exactly the gap #970
// closed for Contract writes and #990 for the payment and rate ones.
// The map's value is unused -- only the key set matters -- kept as a map
// rather than a slice so the test below can strike off each pattern it
// finds and report any it never saw.
var bookingWriteRoutes = map[string]bool{
	"POST /api/practices/{practiceId}/engagements/{engagementId}/offers":       true,
	"POST /api/practices/{practiceId}/offers/{offerId}/withdraw":               true,
	"POST /api/practices/{practiceId}/engagement-requests/{requestId}/approve": true,
	"POST /api/practices/{practiceId}/engagement-requests/{requestId}/refuse":  true,
	"POST /api/practices/{practiceId}/email-suppressions/clear":                true,
	"POST /api/practices/{practiceId}/billing/purchases":                       true,
	"POST /api/practices/{practiceId}/clients/{clientId}/erasure":              true,
	"PUT /api/practices/{practiceId}/website":                                  true,
	"PUT /api/practices/{practiceId}/client-field-template":                    true,
}

// TestRoutes_BookingAndSettingsWritesDeclareRoles is #1016's own
// guardrail, mirroring TestRoutes_ContractWritesDeclareRoles and
// TestRoutes_PaymentAndRateWritesDeclareRoles: the startup panic in
// staffauth.GatedRouter.GatedWrite catches an empty role list passed to
// the gated door, but it cannot catch one of these writes mounted
// through the role-free Exempt/Replayable door instead -- that path
// never calls GatedWrite at all. This walks the real registry
// idempotency.Router builds and fails if any of bookingWriteRoutes'
// patterns is missing a role declaration, or missing from the registry
// entirely (mounted under a different pattern, or not mounted through
// ExemptGated / ReplayableGated).
func TestRoutes_BookingAndSettingsWritesDeclareRoles(t *testing.T) {
	_, _, irRoutes := routes(testDeps())

	seen := make(map[string]bool, len(bookingWriteRoutes))
	for _, route := range irRoutes {
		if !bookingWriteRoutes[route.Pattern] {
			continue
		}
		seen[route.Pattern] = true
		if len(route.Roles) == 0 {
			t.Errorf("write %q carries no role declaration -- #1016's own gap, reopened", route.Pattern)
		}
	}
	for pattern := range bookingWriteRoutes {
		if !seen[pattern] {
			t.Errorf("write %q not found in the registry -- did it move, or stop being mounted through ExemptGated/ReplayableGated?", pattern)
		}
	}
}
