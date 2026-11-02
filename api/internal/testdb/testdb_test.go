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
	if err := db.Admin.QueryRowContext(t.Context(), "SELECT count(*) FROM goose_bootstrap_check").Scan(&count); err != nil {
		t.Fatalf("query bootstrap table: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty bootstrap table, got %d rows", count)
	}

	if err := db.App.QueryRowContext(t.Context(), "SELECT count(*) FROM practices").Scan(&count); err != nil {
		t.Fatalf("query practices as app role: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty practices table, got %d rows", count)
	}
}

func TestNew_CapsTheAppPoolAtOneConnection(t *testing.T) {
	db := testdb.New(t)

	if got := db.App.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("App MaxOpenConnections = %d, want 1", got)
	}
}

func TestOptOutOfOneConnectionPool_LiftsTheCap(t *testing.T) {
	db := testdb.New(t)

	db.OptOutOfOneConnectionPool("a test that needs two connections on purpose")

	if got := db.App.Stats().MaxOpenConnections; got != testdb.OptOutAppConnections {
		t.Fatalf("App MaxOpenConnections = %d, want %d", got, testdb.OptOutAppConnections)
	}
}
