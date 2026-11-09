package testdb

import (
	"database/sql"
	"testing"
)

// BeginAdmin opens a transaction on the superuser connection, for a test
// whose subject is a function that takes a *sql.Tx and no session. The
// test commits it if it needs the writes to outlive it; otherwise it is
// rolled back when the test ends.
func (d *DB) BeginAdmin(t *testing.T) *sql.Tx {
	t.Helper()
	tx, err := d.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		// coverage:ignore reason: a failure to open a transaction on the fixture's own database
		t.Fatalf("testdb: begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}
