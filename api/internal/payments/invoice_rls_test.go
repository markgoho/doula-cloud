package payments_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the invoices_practice_visibility policy from
// 00024_invoices.sql directly via db.App and set_config, following the
// pattern in billing/rls_test.go (credit_ledger's own direct-column
// practice_id policy, the shape invoices reuses).

// TestRLS_InvoicesFailsClosedWithNoSessionVarSet proves invoices denies
// all rows when app.current_practice_id is unset.
func TestRLS_InvoicesFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Some Practice")
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	seedInvoice(t, db, practiceID, contractID, "in_rls_closed", "open", 5000, time.Now())

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM invoices`).Scan(&count); err != nil {
		t.Fatalf("query invoices with no session variables set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_InvoicesVisibilityIsScopedToCurrentPractice proves Practice A's
// session sees only Practice A's invoices -- Practice B's stay invisible.
func TestRLS_InvoicesVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	engagementA := seedEngagement(t, db, practiceA, "Client A", "a@example.com")
	engagementB := seedEngagement(t, db, practiceB, "Client B", "b@example.com")
	contractA := seedContract(t, db, engagementA)
	contractB := seedContract(t, db, engagementB)
	invoiceA := seedInvoice(t, db, practiceA, contractA, "in_rls_a", "open", 5000, time.Now())
	seedInvoice(t, db, practiceB, contractB, "in_rls_b", "open", 7000, time.Now())

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM invoices`)
	if err != nil {
		t.Fatalf("query invoices: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visibleIDs = append(visibleIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visibleIDs) != 1 || visibleIDs[0] != invoiceA {
		t.Fatalf("visible invoices = %v, want only %q", visibleIDs, invoiceA)
	}
}

// TestRLS_InvoicesCannotInsertForAnotherPractice proves Practice B's
// session cannot insert an invoices row for Practice A -- the WITH CHECK
// side of the policy, derived from the same USING clause, rejects it.
func TestRLS_InvoicesCannotInsertForAnotherPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	engagementA := seedEngagement(t, db, practiceA, "Client A", "a@example.com")
	contractA := seedContract(t, db, engagementA)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceB); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	_, err = tx.ExecContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, amount_cents) VALUES ($1, $2, 'in_rls_insert', 5000)`,
		practiceA, contractA,
	)
	if err == nil {
		t.Fatal("expected inserting an invoices row for another Practice to be rejected by RLS, got no error")
	}
}
