package testdb

import (
	"database/sql"
	"fmt"
	"testing"
)

// DeadLetterReason reads one outbox row's dead_letter_reason through the
// superuser, "" while it has none. table is a test's own constant.
func DeadLetterReason(t *testing.T, db *DB, table, id string) string {
	t.Helper()
	var reason sql.NullString
	//nolint:gosec // table is a compile-time constant in the calling test, never request data
	if err := db.Admin.QueryRowContext(t.Context(),
		fmt.Sprintf(`SELECT dead_letter_reason FROM %s WHERE id = $1`, table), id,
	).Scan(&reason); err != nil {
		// coverage:ignore reason: fixture read failure, not exercised by the happy-path test
		t.Fatalf("testdb: read %s dead_letter_reason: %v", table, err)
	}
	return reason.String
}
