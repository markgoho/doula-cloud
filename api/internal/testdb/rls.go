package testdb

import (
	"context"
	"database/sql"
	"testing"
)

// SessionVar is one app.* setting a Row-Level Security policy reads.
type SessionVar struct{ Name, Value string }

// Practice scopes a transaction the way the Staff middleware does.
func Practice(id string) SessionVar { return SessionVar{"app.current_practice_id", id} }

// Client scopes a transaction the way the portal middleware does.
func Client(id string) SessionVar { return SessionVar{"app.current_client_id", id} }

// Staff names the signed-in Staff member.
func Staff(id string) SessionVar { return SessionVar{"app.current_staff_id", id} }

// Identity names the signed-in identity.
func Identity(uid string) SessionVar { return SessionVar{"app.current_identity_uid", uid} }

// Querier is what *sql.DB and *sql.Tx share.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// BeginAs opens a transaction on App, sets each var for its length, and
// rolls it back when the test ends. App holds one connection, so the
// transaction owns it until then: a closure or loop that opens a second
// one in the same test rolls the first back itself.
func (d *DB) BeginAs(t *testing.T, vars ...SessionVar) *sql.Tx {
	t.Helper()
	tx, err := d.App.BeginTx(t.Context(), nil)
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("testdb: begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	for _, v := range vars {
		if _, err := tx.ExecContext(t.Context(), `SELECT set_config($1, $2, true)`, v.Name, v.Value); err != nil {
			// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
			t.Fatalf("testdb: set_config %s: %v", v.Name, err)
		}
	}
	return tx
}

// Scalar returns the one value query selects, for a count or a single
// column.
func Scalar[T any](t *testing.T, q Querier, query string, args ...any) T {
	t.Helper()
	var out T
	if err := q.QueryRowContext(t.Context(), query, args...).Scan(&out); err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("testdb: query %q: %v", query, err)
	}
	return out
}

// Column returns the first column of every row query selects.
func Column[T any](t *testing.T, q Querier, query string, args ...any) []T {
	t.Helper()
	rows, err := q.QueryContext(t.Context(), query, args...)
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("testdb: query %q: %v", query, err)
	}
	defer func() { _ = rows.Close() }()
	var out []T
	for rows.Next() {
		var v T
		if err := rows.Scan(&v); err != nil {
			// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
			t.Fatalf("testdb: scan %q: %v", query, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("testdb: iterate %q: %v", query, err)
	}
	return out
}

// RowsAffected runs a write on tx and returns how many rows it touched,
// for a test that proves Row-Level Security hid the rows from it.
func RowsAffected(t *testing.T, tx *sql.Tx, query string, args ...any) int64 {
	t.Helper()
	result, err := tx.ExecContext(t.Context(), query, args...)
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("testdb: exec %q: %v", query, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("testdb: rows affected by %q: %v", query, err)
	}
	return rows
}

// RequireFailsClosed proves table's policy shows App no row while no
// session variable is set.
func RequireFailsClosed(t *testing.T, db *DB, table string) {
	t.Helper()
	if count := Scalar[int](t, db.App, `SELECT count(*) FROM `+table); count != 0 {
		// coverage:ignore reason: the failure branch ends the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
		t.Fatalf("%s: %d rows visible with no session variable set, want 0", table, count)
	}
}

// ReadAs runs a reader that answers (value, found, err) for subjectID in a
// transaction scoped by who, and returns what it read. It rolls the
// transaction back when it returns, so a test can read again after each
// change it makes, as a different state of the same Client.
func ReadAs[T any](t *testing.T, db *DB, who SessionVar, subjectID string, read func(context.Context, *sql.Tx, string) (T, bool, error)) (T, bool) {
	t.Helper()
	tx := db.BeginAs(t, who)
	defer func() { _ = tx.Rollback() }()
	value, found, err := read(t.Context(), tx, subjectID)
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("testdb: read as %s: %v", who.Name, err)
	}
	return value, found
}
