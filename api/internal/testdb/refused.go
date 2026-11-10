package testdb

import (
	"strings"
	"testing"
)

// Write is a statement and its arguments, run on the superuser
// connection so no route or policy stands between it and the schema.
type Write struct {
	Query string
	Args  []any
}

// RequireRefused runs each write on db.Admin, each in its own subtest,
// and fails one the schema accepts, or, when constraint is not empty, one
// refused by anything but that constraint.
func RequireRefused(t *testing.T, db *DB, constraint string, writes map[string]Write) {
	t.Helper()
	for name, w := range writes {
		t.Run(name, func(t *testing.T) {
			_, err := db.Admin.ExecContext(t.Context(), w.Query, w.Args...)
			if err == nil || !strings.Contains(err.Error(), constraint) {
				// coverage:ignore reason: the failure branch ends the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
				t.Fatalf("write = %v, want the schema to refuse it (%s)", err, constraint)
			}
		})
	}
}

// RequireAccepted runs each write on db.Admin and fails one the schema
// refuses: the legal writes beside the refused ones.
func RequireAccepted(t *testing.T, db *DB, writes map[string]Write) {
	t.Helper()
	for name, w := range writes {
		if _, err := db.Admin.ExecContext(t.Context(), w.Query, w.Args...); err != nil {
			// coverage:ignore reason: the failure branch fails the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
			t.Errorf("%s was refused: %v", name, err)
		}
	}
}
