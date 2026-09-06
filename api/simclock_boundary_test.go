package main

import (
	"os/exec"
	"strings"
	"testing"
)

// simclockPackage is the one package in api/ that names a Stripe test
// clock. Everything test-only about simulated time lives behind it.
const simclockPackage = "doula-cloud/api/internal/simclock"

// TestBFFDoesNotDependOnSimclock is the guard behind #780's "no test-only
// parameter exists anywhere in api/". One does exist -- `TestClock` on the
// Customer-creation params in internal/simclock/stripe.go -- and it is
// reachable only from cmd/simclock, a binary nothing deploys.
//
// The claim that the BFF cannot reach it is checked here rather than
// asserted in a comment, because an import added in a year's time would
// quietly make the comment false. This is the whole reason the product
// resolves a Client's Stripe Customer from a mapping row instead of
// carrying a sandbox-only parameter of its own: a deployed configuration
// has no path to a test clock at all.
func TestBFFDoesNotDependOnSimclock(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", ".").Output()
	if err != nil {
		// coverage:ignore reason: requires a broken toolchain, not exercised by unit tests
		t.Fatalf("go list -deps: %v", err)
	}
	for pkg := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if pkg == simclockPackage {
			t.Fatalf("the BFF binary depends on %s -- a Stripe test clock is now reachable from a deployed configuration", simclockPackage)
		}
	}
}
