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

// TestRoutes_EveryPortalWriteDeclaresAnIdempotencyStance is the
// Client-portal mirror of TestRoutes_NoPracticeWriteBypassesIdempotencyRouter
// (#1924): every mutating route under /api/portal goes through
// idempotency.Router's portal doors -- PortalReplayable, keyed to the
// calling Client, or PortalExempt with its reason -- never a bare
// GatedRouter.Write that asks for neither. The Replayable ones are named
// here too, so a route quietly moved from Replayable to Exempt fails the
// build rather than losing its replay unseen.
func TestRoutes_EveryPortalWriteDeclaresAnIdempotencyStance(t *testing.T) {
	_, gRoutes, irRoutes := routes(testDeps())

	declared := make(map[string]bool, len(irRoutes))
	replayable := map[string]bool{}
	for _, route := range irRoutes {
		if !route.Portal {
			continue
		}
		declared[route.Pattern] = true
		if route.Replayable {
			replayable[route.Pattern] = true
		}
	}

	found := 0
	for _, route := range gRoutes {
		if !route.Write || !strings.HasPrefix(route.Pattern, "/api/portal/") {
			continue
		}
		found++
		key := route.Method + " " + route.Pattern
		if !declared[key] {
			t.Errorf("route %q is a portal write not registered through idempotency.Router -- register it through ir.PortalReplayable or ir.PortalExempt instead", key)
		}
	}
	if found == 0 {
		t.Fatal("found zero portal writes in g.Routes() -- did registerPortalRoutes stop wiring feature Mounts?")
	}

	for _, want := range []string{
		"POST /api/portal/engagements/{engagementId}/messages",
		"POST /api/portal/engagements/{engagementId}/contract/sign",
	} {
		if !replayable[want] {
			t.Errorf("route %q is not PortalReplayable, want it replayable (#1924)", want)
		}
	}
}
