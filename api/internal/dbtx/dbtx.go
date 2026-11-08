// Package dbtx is the transaction lifecycle every handler that opens its
// own transaction repeats: begin it, roll it back unless it committed, and
// answer 500 when either end fails.
package dbtx

import (
	"context"
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// Begin opens a transaction on db for the request, writing a 500 and
// returning ok=false if the database cannot be reached. Callers defer
// Rollback(tx) straight after.
func Begin(w http.ResponseWriter, r *http.Request, db *sql.DB) (tx *sql.Tx, ok bool) {
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, false
	}
	return tx, true
}

// Rollback undoes tx unless it already committed. Deferring it is safe
// after a successful Commit: the second end of a finished transaction is a
// no-op.
func Rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

// Commit commits tx, writing a 500 and returning false if the commit
// fails.
func Commit(w http.ResponseWriter, tx *sql.Tx) bool {
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB commit failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	return true
}

// CommitWith commits tx and answers status with no body, or answers 500
// instead if the commit fails.
func CommitWith(w http.ResponseWriter, tx *sql.Tx, status int) {
	if Commit(w, tx) {
		w.WriteHeader(status)
	}
}

// ExecRows runs a statement on tx and returns the rows it affected,
// writing a 500 and returning ok=false if the statement or the driver's
// count fails.
func ExecRows(ctx context.Context, w http.ResponseWriter, tx *sql.Tx, query string, args ...any) (rows int64, ok bool) {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return 0, false
	}
	rows, err = result.RowsAffected()
	if err != nil {
		// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return 0, false
	}
	return rows, true
}
