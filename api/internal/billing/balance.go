package billing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/staffauth"
)

// LedgerEntry is one row of a Practice's credit_ledger: where credits came
// from (or went to), the signed quantity, and when it happened.
type LedgerEntry struct {
	Origin    string    `json:"origin"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"createdAt"`
}

// BalanceResponse is the body of GetBalanceHandler's response: a Practice's
// current derived balance plus the ledger rows that produced it, most
// recent first.
type BalanceResponse struct {
	Balance int           `json:"balance"`
	Ledger  []LedgerEntry `json:"ledger"`
}

// Balance returns a Practice's current billing credit balance, derived by
// summing its credit_ledger rows -- the balance is never stored, per
// 00015_credit_ledger.sql. Exported so other packages and the BFF can look
// up a Practice's balance without duplicating the query.
func Balance(ctx context.Context, tx *sql.Tx, practiceID string) (int, error) {
	var balance int
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(quantity), 0) FROM credit_ledger WHERE practice_id = $1`,
		practiceID,
	).Scan(&balance)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, fmt.Errorf("billing: sum credit_ledger: %w", err)
	}
	return balance, nil
}

// ledgerHistory lists a Practice's credit_ledger rows, most recent first --
// (created_at, id) DESC follows the cursor-friendly ordering from
// docs/api-design.md so a future paginated version of this endpoint can
// adopt cursors without changing row order underneath existing callers.
func ledgerHistory(ctx context.Context, tx *sql.Tx, practiceID string) ([]LedgerEntry, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT origin, quantity, created_at FROM credit_ledger
		 WHERE practice_id = $1
		 ORDER BY created_at DESC, id DESC`,
		practiceID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("billing: list credit_ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries := []LedgerEntry{}
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.Origin, &e.Quantity, &e.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("billing: scan credit_ledger row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: iterate credit_ledger rows: %w", err)
	}
	return entries, nil
}

// GetBalanceHandler reads a Practice's billing credit balance and ledger
// history. Owner and Admin only (ADR-0008's read table) -- a Doula never
// reaches it, enforced by the "owner","admin" role declaration on this
// route's GatedRouter mount in main.go, not inside this handler. Must be
// mounted behind staffauth.Middleware.
func GetBalanceHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		balance, err := Balance(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		ledger, err := ledgerHistory(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(BalanceResponse{Balance: balance, Ledger: ledger}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
