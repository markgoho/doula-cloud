package staffauth_test

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rawSelfLookup is the pair of SQL fragments that together mean "resolve
// the caller's own Staff row before any Practice is known": setting
// app.current_identity_uid, the session variable staff_self_visibility
// (00006) reads, and the lookup that variable admits.
//
// They have to move together. Setting the variable without resolving
// leaves the policy satisfied for a row nobody checked exists; resolving
// without setting it reads through some other policy's window and misses
// the RLS the pair is for. No Go type can refuse that split -- both
// halves are strings handed to a driver -- which is why this gate is
// lexical.
var rawSelfLookup = []string{
	`set_config('app.current_identity_uid'`,
	`FROM staff WHERE identity_uid`,
}

// selfLookupOwners is every non-test file in this package permitted to
// name those fragments, and why. Anything not here reaches the pair
// through the one owner instead, so the two halves cannot come apart.
var selfLookupOwners = map[string]string{
	"self.go":              "the owner: the single place the pair is written, and the only place it may be",
	"signup.go":            "creates the staff row and then sets the variable for the row it just wrote -- there is nothing to resolve first",
	"accept.go":            "claims or creates the staff row from an invitation's token, for the same reason as signup.go",
	"signupresume.go":      "asks whether a half-landed signup already left a row, a question with three answers rather than one refusal (#745)",
	"mfarecovery_spend.go": "unauthenticated: the spent code names whose row it is, and app.current_identity_uid is deliberately never set here",
}

// TestOnlyOneOwnerWritesTheSelfLookup fails the build on a route that
// hand-rolls the pre-Practice self-resolution again instead of reaching
// the owner. It is lexical because the failure it catches is lexical:
// several handlers each wrote these two statements out, and most of them
// wrote them differently.
func TestOnlyOneOwnerWritesTheSelfLookup(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		// coverage:ignore reason: reading this package's own directory, which the test binary was built from
		t.Fatalf("read package dir: %v", err)
	}

	unmatched := maps.Clone(selfLookupOwners)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// #nosec G304 -- name comes from ReadDir on this package's own
		// source directory, not from anything a caller supplies
		source, err := os.ReadFile(name)
		if err != nil {
			// coverage:ignore reason: reading a file os.ReadDir has just listed
			t.Fatalf("read %s: %v", name, err)
		}
		text := string(source)
		for _, fragment := range rawSelfLookup {
			if !strings.Contains(text, fragment) {
				continue
			}
			delete(unmatched, name)
			if _, allowed := selfLookupOwners[name]; !allowed {
				t.Errorf("%s writes %s itself -- resolve the caller's own staff row through this package's one owner instead, or record here why this file may not", name, fragment)
			}
		}
	}

	for name := range unmatched {
		t.Errorf("selfLookupOwners excuses %s, which no longer writes the raw lookup -- drop the excuse", name)
	}
}
