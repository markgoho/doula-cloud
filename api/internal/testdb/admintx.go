package testdb

import (
	"context"
	"database/sql"
	"testing"
)

// RunAsNotificationWorker runs process in a transaction behind the
// trusted session var outbox.ProcessHandler would set after its own
// secret check, then commits it: an outbox worker exactly as the handler
// drives it, without HTTP.
func (d *DB) RunAsNotificationWorker(t *testing.T, process func(context.Context, *sql.Tx) error) {
	t.Helper()
	tx, err := d.App.BeginTx(t.Context(), nil)
	if err != nil {
		// coverage:ignore reason: a failure to open a transaction on the fixture's own database
		t.Fatalf("testdb: begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		// coverage:ignore reason: set_config cannot fail on the fixture's own database
		t.Fatalf("testdb: open the notification worker door: %v", err)
	}
	if err := process(t.Context(), tx); err != nil {
		// coverage:ignore reason: the worker under test failing is that test's failure
		t.Fatalf("testdb: notification worker: %v", err)
	}
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: commit cannot fail on the fixture's own database
		t.Fatalf("testdb: commit: %v", err)
	}
}

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
