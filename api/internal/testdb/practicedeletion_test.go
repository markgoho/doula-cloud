package testdb_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

// TestSeedPracticeDeletion proves the fixture lands each standing it is
// asked for, and that 00172's CHECKs accept what it writes.
func TestSeedPracticeDeletion(t *testing.T) {
	db := testdb.New(t)
	requested := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	finalize := requested.AddDate(0, 0, 30)

	pending := testdb.SeedPractice(t, db, "Pending")
	testdb.SeedPracticeDeletion(t, db, pending, requested, finalize, nil)
	deleted := testdb.SeedPractice(t, db, "Deleted")
	testdb.SeedPracticeDeletion(t, db, deleted, requested, finalize, &finalize)

	for id, want := range map[string]string{pending: "pending_deletion", deleted: "deleted"} {
		if got := testdb.Scalar[string](t, db.Admin, `SELECT standing::text FROM practices WHERE id = $1`, id); got != want {
			t.Errorf("standing = %s, want %s", got, want)
		}
	}
}
