package testdb_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestHarness is the trivial proof point that the testcontainers-go +
// Podman harness works end-to-end: a real Postgres container starts, the
// goose migrations apply, and the resulting table is queryable.
func TestHarness(t *testing.T) {
	db := testdb.New(t)

	var count int
	if err := db.QueryRow("SELECT count(*) FROM goose_bootstrap_check").Scan(&count); err != nil {
		t.Fatalf("query bootstrap table: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty bootstrap table, got %d rows", count)
	}
}
