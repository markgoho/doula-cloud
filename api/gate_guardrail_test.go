package main

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestRoutes_EveryDeclaredGETHasRoleDeclaration is the rlsguardrail-shaped
// assertion #315's AC asks for, run against the real registry routes()
// builds: every GET GatedRouter mounted carries a non-empty role
// declaration. GatedRouter.Get already panics before an empty one could
// reach this table, so this test is the belt to that braces --
// catastrophic failure (a panic taking the whole binary down) and an
// ordinary failing test should both catch the same mistake.
func TestRoutes_EveryDeclaredGETHasRoleDeclaration(t *testing.T) {
	_, registry, _ := routes(testDeps())
	if len(registry) == 0 {
		t.Fatal("routes() registered zero routes through GatedRouter -- did routes() stop wiring g.Get calls?")
	}
	for _, route := range registry {
		if route.Write {
			// A write carries no role declaration by design: ADR-0008's
			// read table is about reads. What it must do is be here at all,
			// which TestRoutes_EveryRouteIsRegisteredThroughTheGate checks.
			continue
		}
		if route.Exempt {
			// An exempt route has no roles by definition -- it is outside
			// staffauth.Middleware, so there is no membership to check
			// against. What it must carry instead is a reason, which
			// GatedRouter.OpenGet refuses to register without.
			if route.Reason == "" {
				t.Errorf("exempt route %q carries no reason", route.Pattern)
			}
			continue
		}
		if len(route.Roles) == 0 {
			t.Errorf("route %q has no role declaration", route.Pattern)
		}
	}
}

// muxRegistration finds a route mounted straight onto an http.ServeMux in
// this package's own source.
//
// This regex used to be the load-bearing part of this file: the raw mux
// travelled into every route file beside GatedRouter, so a route could
// skip the gate entirely and the only thing that would notice was a scan
// of the source text. routes() now hands the mux to GatedRouter and names
// it nowhere else, so a bypass does not compile -- and what remains here
// is a check that the arrangement itself has not been undone.
var muxRegistration = regexp.MustCompile(`\bmux\.(?:Handle|HandleFunc)\(`)

// TestRoutes_EveryRouteIsRegisteredThroughTheGate is the structural half
// of #231's finding, and it is now a statement about the shape of the
// package rather than about any one route: nothing in this package
// registers on a mux directly, because nothing but routes() can name one.
//
// It stays a source scan on purpose. The compiler already refuses a
// bypass -- a route file has no mux parameter to reach for -- so this
// does not catch a route sneaking past. What it catches is somebody
// handing the mux back out, which is the change that would quietly make
// every other guardrail here optional again.
func TestRoutes_EveryRouteIsRegisteredThroughTheGate(t *testing.T) {
	for name, src := range packageSources(t) {
		if name == "routes.go" {
			// routes.go constructs the mux and hands it to GatedRouter.
			// That one call is the arrangement, not a violation of it.
			continue
		}
		if muxRegistration.MatchString(src) {
			t.Errorf("%s registers a route directly on a mux -- register it through GatedRouter (Get, OpenGet or Write) so it lands in the registry", name)
		}
	}
}

// TestRoutes_NoGETIsMountedAsAWrite guards the other direction: a GET
// registered through Write would serve reads that ADR-0008's read table
// never sees. GatedRouter.Write panics on one, so this is that panic's
// belt-and-braces mirror over the real table.
func TestRoutes_NoGETIsMountedAsAWrite(t *testing.T) {
	_, registry, _ := routes(testDeps())
	for _, route := range registry {
		if route.Write && route.Method == http.MethodGet {
			t.Errorf("route %q is mounted through Write -- a GET belongs at Get or OpenGet", route.Pattern)
		}
	}
}

// TestRoutes_RegistryCoversEveryArea is the sanity check that keeps the
// two tests above honest: a registry that lost a whole route file would
// still pass them by having nothing to complain about. The BFF serves
// reads and writes in both populations, so all four shapes must be
// present.
func TestRoutes_RegistryCoversEveryArea(t *testing.T) {
	_, registry, _ := routes(testDeps())

	var gated, exempt, writes int
	for _, route := range registry {
		switch {
		case route.Write:
			writes++
		case route.Exempt:
			exempt++
		default:
			gated++
		}
	}
	if gated == 0 {
		t.Error("no role-gated GETs in the registry")
	}
	if exempt == 0 {
		t.Error("no ungated GETs in the registry -- the portal reads and the health probe are both OpenGet")
	}
	if writes == 0 {
		t.Error("no writes in the registry")
	}
}

// packageSources is every non-test .go file in this package, by filename.
//
// Read by directory rather than from a list of filenames, and that is the
// point of it: #482 split the route table into one file per area, and a
// hardcoded list would mean the next area added is a set of routes this
// guardrail quietly stops looking at. A file that appears beside these is
// scanned because it is there.
func packageSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		// coverage:ignore reason: the package's own directory is always readable while its tests run
		t.Fatalf("read package directory: %v", err)
	}
	sources := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// #nosec G304 -- name comes from ReadDir on this package's own
		// directory, not from anything a caller supplies
		src, err := os.ReadFile(name)
		if err != nil {
			// coverage:ignore reason: a file ReadDir listed a moment ago
			t.Fatalf("read %s: %v", name, err)
		}
		sources[name] = string(src)
	}
	return sources
}
