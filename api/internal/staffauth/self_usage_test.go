package staffauth_test

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rawSelfLookup is the pair of SQL shapes that together mean "resolve
// the caller's own Staff row before any Practice is known": setting
// app.current_identity_uid, the session variable staff_self_visibility
// (00006) reads, and naming a `staff` row by the identity_uid that
// variable admits.
//
// They have to move together. Setting the variable without resolving
// leaves the policy satisfied for a row nobody checked exists; resolving
// without setting it reads through some other policy's window and misses
// the RLS the pair is for. No Go type can refuse that split -- both
// halves are strings handed to a driver -- which is why this gate is
// lexical.
//
// The second is a pattern rather than a literal because the shape this
// gate has to catch is not only a SELECT. ChangeEmailHandler's own bug
// was an `UPDATE staff SET email = $1 WHERE identity_uid = $2` whose
// rows-affected count stood in for a guard that had never run: a
// re-resolution on the write side, which a `FROM staff WHERE
// identity_uid` literal would have let straight through. The character
// class is SQL-shaped and single-line on purpose, so prose in a comment
// that happens to mention `staff` before a later query cannot match
// across the gap between them.
var rawSelfLookup = []*regexp.Regexp{
	regexp.MustCompile(`set_config\('app\.current_identity_uid'`),
	regexp.MustCompile(`\bstaff\b[A-Za-z0-9_ ,.()*$='-]{0,120}WHERE identity_uid`),
}

// selfOwnerFile is the one file that must hold both halves, rather than
// merely being permitted either.
const selfOwnerFile = "self.go"

// selfLookupOwners is every non-test file in this package permitted to
// name those shapes, and why. Anything not here reaches the pair
// through the one owner instead, so the two halves cannot come apart.
var selfLookupOwners = map[string]string{
	selfOwnerFile:          "the owner: the single place the pair is written, and the only place it may be",
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
	// The owner has to write both halves, not just either. Permission
	// alone would be satisfied by a self.go that had quietly lost one of
	// them -- which is the same split this whole gate exists to refuse,
	// arrived at from the other direction.
	ownerWrites := map[int]bool{}
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
		for i, shape := range rawSelfLookup {
			hit := shape.FindString(text)
			if hit == "" {
				continue
			}
			delete(unmatched, name)
			if name == selfOwnerFile {
				ownerWrites[i] = true
			}
			if _, allowed := selfLookupOwners[name]; !allowed {
				t.Errorf("%s writes %q itself -- resolve the caller's own staff row through this package's one owner instead, or record here why this file may not", name, hit)
			}
		}
	}

	for i, shape := range rawSelfLookup {
		if !ownerWrites[i] {
			t.Errorf("%s does not write %v -- the owner holds both halves of the pair or it is not the owner", selfOwnerFile, shape)
		}
	}

	for name := range unmatched {
		t.Errorf("selfLookupOwners excuses %s, which no longer writes the raw lookup -- drop the excuse", name)
	}
}

// TestTheSelfLookupGateCatchesEveryShapeItClaimsTo checks the gate
// against the statements it exists to stop, because a lexical gate that
// silently stopped matching would pass forever and say nothing. Each
// case is a real statement this package held before #1182, including the
// write-side re-resolution whose rows-affected count stood in for the
// guard -- the one a `FROM staff WHERE identity_uid` literal would have
// missed.
func TestTheSelfLookupGateCatchesEveryShapeItClaimsTo(t *testing.T) {
	caught := []struct {
		name string
		sql  string
	}{
		{"the variable", `SELECT set_config('app.current_identity_uid', $1, true)`},
		{"a read", "SELECT id FROM staff WHERE identity_uid = $1"},
		{"a locking read", "SELECT id, deleted_at FROM staff WHERE identity_uid = $1 FOR UPDATE"},
		{"a write", "UPDATE staff SET email = $1 WHERE identity_uid = $2"},
		{"an existence check", "SELECT EXISTS(SELECT 1 FROM staff WHERE identity_uid = $1)"},
	}
	for _, c := range caught {
		t.Run(c.name, func(t *testing.T) {
			for _, shape := range rawSelfLookup {
				if shape.MatchString(c.sql) {
					return
				}
			}
			t.Errorf("no shape in rawSelfLookup matches %q -- the gate would let this past", c.sql)
		})
	}

	// Statements that name one of the two halves and are nothing to do
	// with resolving the caller's own row. A gate that failed these
	// would push the next person into an excuse they do not need.
	ignored := []struct {
		name string
		sql  string
	}{
		{"another table keyed on the same column", "DELETE FROM sessions WHERE identity_uid = $1"},
		{"a table whose name merely starts with staff", "UPDATE staff_token_mail_outbox SET status = 'sent' WHERE identity_uid = $1"},
		{"a staff read that names a row by its id", "SELECT work_state FROM staff WHERE id = $1"},
	}
	for _, c := range ignored {
		t.Run(c.name, func(t *testing.T) {
			for _, shape := range rawSelfLookup {
				if shape.MatchString(c.sql) {
					t.Errorf("%v matches %q, which resolves no Staff caller", shape, c.sql)
				}
			}
		})
	}
}
