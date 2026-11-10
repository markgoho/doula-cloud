package testdb

import (
	"testing"
	"time"
)

// SeedPracticeDeletion writes practiceID's deletion facts straight onto
// practices, bypassing practicedeletion's request handler and finalize
// worker: a Practice pending deletion since requestedAt and due to
// finalize at finalizeAt, and, when deletedAt is not nil, finalized then.
//
// 00172 holds a request to all three of its facts, so the fixture names
// a requester too: a Staff row seeded for the purpose, with no
// membership, so it changes no count a caller's own Practice reads. Six
// packages' tests need a Practice in one of these standings, which is
// why it lives here rather than in each.
func SeedPracticeDeletion(t *testing.T, db *DB, practiceID string, requestedAt, finalizeAt time.Time, deletedAt *time.Time) {
	t.Helper()
	requester := SeedStaff(t, db, "deletion-requester-"+practiceID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices
		    SET deletion_requested_at = $2, deletion_requested_by = $3, deletion_finalize_at = $4, deleted_at = $5
		  WHERE id = $1`,
		practiceID, requestedAt, requester, finalizeAt, deletedAt,
	); err != nil {
		// coverage:ignore reason: fixture update failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed practice deletion %q: %v", practiceID, err)
	}
}
