package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// gateWriteMutating finds every mutating (POST/PUT/PATCH/DELETE) route
// registered straight through GatedRouter.Write, skipping
// idempotency.Router.
//
// This regex used to look for mux.Handle, which routes_practice.go never
// had access to in the first place -- that file has always taken
// (g, ir, d) and no mux, so the check could not fail however wrong the
// file got. Now that every other route file goes through the same router,
// g.Write is the way a mutating route here would actually bypass its
// idempotency stance, and the test has something real to catch.
var gateWriteMutating = regexp.MustCompile(`g\.Write\(\s*"(POST|PUT|PATCH|DELETE) ([^"]+)"`)

// TestRoutes_NoMutatingRouteInPracticeRoutesBypassesIdempotencyRouter:
// every mutating route routes_practice.go registers must go through
// idempotency.Router (ir.Replayable or ir.Exempt), which is what forces a
// stance to be declared. GatedRouter.Write would mount it just as well
// and ask for nothing, so a mutating route reaching for that verb in this
// file is the bypass -- everywhere else in the package it is the ordinary
// way to mount a write.
func TestRoutes_NoMutatingRouteInPracticeRoutesBypassesIdempotencyRouter(t *testing.T) {
	src := readSourceFile(t, "routes_practice.go")

	if matches := gateWriteMutating.FindAllStringSubmatch(src, -1); len(matches) > 0 {
		for _, m := range matches {
			t.Errorf("route %q %q is registered through g.Write in routes_practice.go, bypassing idempotency.Router -- register it through ir.Replayable or ir.Exempt instead", m[1], m[2])
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
