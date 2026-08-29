package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// muxHandleMutating finds every mutating (POST/PUT/PATCH/DELETE) route
// registered directly on the raw mux (mux.Handle) in this package's own
// source -- mirrors muxGetPattern's approach in gate_guardrail_test.go.
// routes_practice.go registers every one of its mutating routes through
// idempotency.Router instead (ir.Replayable / ir.Exempt), so this should
// match zero routes in that file; a hit means a mutating route slipped
// back onto the raw mux, bypassing the idempotency-stance requirement
// entirely.
var muxHandleMutating = regexp.MustCompile(`mux\.Handle\(\s*"(POST|PUT|PATCH|DELETE) ([^"]+)"`)

// TestRoutes_NoMutatingRouteInPracticeRoutesBypassesIdempotencyRouter is
// the idempotency-stance mirror of TestRoutes_NoGETBypassesTheGate: every
// mutating route routes_practice.go registers must go through
// idempotency.Router (ir.Replayable or ir.Exempt), not straight onto mux.
// idempotency.Router itself cannot forbid a raw mux.Handle call -- the
// same structural gap staffauth.GatedRouter has -- so this closes it the
// same way: by scanning the file's own source.
func TestRoutes_NoMutatingRouteInPracticeRoutesBypassesIdempotencyRouter(t *testing.T) {
	src := readSourceFile(t, "routes_practice.go")

	if matches := muxHandleMutating.FindAllStringSubmatch(src, -1); len(matches) > 0 {
		for _, m := range matches {
			t.Errorf("route %q %q is registered directly on mux in routes_practice.go, bypassing idempotency.Router -- register it through ir.Replayable or ir.Exempt instead", m[1], m[2])
		}
	}
}

// idempotencyRouterCall finds every ir.Replayable( or ir.Exempt( call in
// this package's own source, so its own balanced-paren statement can be
// checked for whether it actually wraps its handler in idempotency.Wrap.
var idempotencyRouterCall = regexp.MustCompile(`ir\.(Replayable|Exempt)\(`)

// TestRoutes_IdempotencyDeclarationMatchesItsHandlerChain proves the two
// declaration methods aren't just self-reported labels: a route declared
// through ir.Replayable actually carries idempotency.Wrap somewhere in
// its handler chain, and a route declared through ir.Exempt does not.
// idempotency.Router itself can't check this -- an http.Handler value
// carries no way to ask "were you built with Wrap?" -- so this is the
// belt to that braces, scanning routes_practice.go's own text the same
// way TestRoutes_NoEngagementWriteBypassesTheAttachingWriteGate checks
// staffauth.AttachingWrite.
func TestRoutes_IdempotencyDeclarationMatchesItsHandlerChain(t *testing.T) {
	src := readSourceFile(t, "routes_practice.go")

	matches := idempotencyRouterCall.FindAllStringSubmatchIndex(src, -1)
	if len(matches) == 0 {
		t.Fatal("found zero ir.Replayable/ir.Exempt calls in routes_practice.go -- did the regex stop matching the source?")
	}
	for _, m := range matches {
		method := src[m[2]:m[3]]
		stmt := balancedParenStatement(src, m[0])
		wrapped := strings.Contains(stmt, "idempotency.Wrap(")

		switch method {
		case "Replayable":
			if !wrapped {
				t.Errorf("ir.Replayable call does not contain idempotency.Wrap( in its handler chain: %s", stmt)
			}
		case "Exempt":
			if wrapped {
				t.Errorf("ir.Exempt call contains idempotency.Wrap( in its handler chain -- declare it Replayable instead: %s", stmt)
			}
		}
	}
}

// TestRoutes_EveryMutatingPracticeRouteHasIdempotencyDeclaration is the
// idempotency-stance mirror of TestRoutes_EveryDeclaredGETHasRoleDeclaration,
// run against the real registry routes() builds: every mutating route
// registerPracticeRoutes registered through idempotency.Router is either
// Replayable or carries a non-empty exemption reason.
// idempotency.Router.Exempt already panics before an empty reason could
// reach this table, so this test is the belt to that braces.
func TestRoutes_EveryMutatingPracticeRouteHasIdempotencyDeclaration(t *testing.T) {
	_, _, registry := routes(testDeps())
	if len(registry) == 0 {
		t.Fatal("routes() registered zero mutating routes through idempotency.Router -- did registerPracticeRoutes stop wiring ir calls?")
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

// readSourceFile reads one file from this package's own directory --
// mirrors packageSource's #nosec rationale in gate_guardrail_test.go: name
// is a fixed literal this test passes, never attacker-controlled.
func readSourceFile(t *testing.T, name string) string {
	t.Helper()
	// #nosec G304 -- name is a fixed literal this test itself passes, not
	// attacker-controlled
	src, err := os.ReadFile(name)
	if err != nil {
		// coverage:ignore reason: this package's own source file is always readable while its tests run
		t.Fatalf("read %s: %v", name, err)
	}
	return string(src)
}
