package main

import (
	"strings"
	"testing"
)

// TestRoutes_NoPracticeWriteBypassesIdempotencyRouter: every mutating
// route mounted under /api/practices/{practiceId}/... must go through
// idempotency.Router (ir.Replayable or ir.Exempt), which is what forces
// an idempotency stance to be declared. GatedRouter.Write would mount it
// just as well and ask for nothing.
//
// #836 moved every feature's registration out of routes_practice.go into
// that feature's own Mount, so a source scan of one file can no longer
// see them all -- this walks the real registries routes() builds
// instead: g.Routes() has every write regardless of which package
// registered it, and ir.Routes() has only the ones that went through the
// idempotency seam. A Practice-scoped write missing from the second is
// the bypass.
func TestRoutes_NoPracticeWriteBypassesIdempotencyRouter(t *testing.T) {
	_, gRoutes, irRoutes := routes(testDeps())

	declared := make(map[string]bool, len(irRoutes))
	for _, route := range irRoutes {
		declared[route.Pattern] = true
	}

	found := 0
	for _, route := range gRoutes {
		if !route.Write || !strings.HasPrefix(route.Pattern, "/api/practices/{practiceId}") {
			continue
		}
		found++
		key := route.Method + " " + route.Pattern
		if !declared[key] {
			t.Errorf("route %q is a Practice-scoped write not registered through idempotency.Router -- register it through ir.Replayable or ir.Exempt instead", key)
		}
	}
	if found == 0 {
		t.Fatal("found zero Practice-scoped writes in g.Routes() -- did registerPracticeRoutes stop wiring feature Mounts?")
	}
}

// TestRoutes_EveryMutatingPracticeRouteHasIdempotencyDeclaration is the
// idempotency-stance mirror of TestRoutes_EveryDeclaredGETHasRoleDeclaration,
// run against the real registry routes() builds: every mutating route
// registered through idempotency.Router is either Replayable or carries a
// non-empty exemption reason. idempotency.Router.Exempt already panics
// before an empty reason could reach this table, so this test is the
// belt to that braces.
func TestRoutes_EveryMutatingPracticeRouteHasIdempotencyDeclaration(t *testing.T) {
	_, _, registry := routes(testDeps())
	if len(registry) == 0 {
		t.Fatal("routes() registered zero mutating routes through idempotency.Router -- did registerPracticeRoutes stop wiring feature Mounts?")
	}
	for _, route := range registry {
		if route.Replayable {
			if route.Reason != "" {
				t.Errorf("route %q is Replayable but also carries a Reason %q, want it empty", route.Pattern, route.Reason)
			}
			continue
		}
		if route.Reason == "" {
			t.Errorf("route %q has no idempotency declaration", route.Pattern)
		}
	}
}
