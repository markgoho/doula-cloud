package main

import "testing"

// contractWriteRoutes are the Contract writes #970 exists to gate: the
// five named on the ticket (create, set values, send, void, and the
// Contract Template's own write), each mounted through
// idempotency.Router.ExemptGated rather than the role-free Exempt. The
// map's value is unused -- only the key set matters -- kept as a map
// rather than a slice so the test below can strike off each pattern it
// finds and report any it never saw.
var contractWriteRoutes = map[string]bool{
	"PUT /api/practices/{practiceId}/contract-template":                         true,
	"POST /api/practices/{practiceId}/engagements/{engagementId}/contract":      true,
	"PUT /api/practices/{practiceId}/engagements/{engagementId}/contract":       true,
	"POST /api/practices/{practiceId}/engagements/{engagementId}/contract/send": true,
	"POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void": true,
}

// TestRoutes_ContractWritesDeclareRoles is #970's own guardrail: the
// startup panic in staffauth.GatedRouter.GatedWrite catches an empty role
// list passed to the gated door, but it cannot catch a Contract write
// mounted through the role-free Exempt/Replayable door instead -- that
// path never calls GatedWrite at all. This walks the real registry
// idempotency.Router builds and fails if any of contractWriteRoutes'
// patterns is missing a role declaration, or missing from the registry
// entirely (mounted under a different pattern, or not mounted through
// ExemptGated).
func TestRoutes_ContractWritesDeclareRoles(t *testing.T) {
	_, _, irRoutes := routes(testDeps())

	seen := make(map[string]bool, len(contractWriteRoutes))
	for _, route := range irRoutes {
		if !contractWriteRoutes[route.Pattern] {
			continue
		}
		seen[route.Pattern] = true
		if len(route.Roles) == 0 {
			t.Errorf("Contract write %q carries no role declaration -- #970's own gap, reopened", route.Pattern)
		}
	}
	for pattern := range contractWriteRoutes {
		if !seen[pattern] {
			t.Errorf("Contract write %q not found in the registry -- did it move, or stop being mounted through ExemptGated?", pattern)
		}
	}
}
