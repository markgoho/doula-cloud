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
	practiceID := testdb.SeedPractice(t, db, "Some Practice")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	seedInvoice(t, db, practiceID, contractID, "in_rls_closed", invoiceStatusOpen, 5000, time.Now())

	count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM invoices`)
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_InvoicesVisibilityIsScopedToCurrentPractice proves Practice A's
// session sees only Practice A's invoices -- Practice B's stay invisible.
func TestRLS_InvoicesVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	_, engagementA := testdb.SeedNamedEngagement(t, db, practiceA, "Client A", "a@example.com")
	_, engagementB := testdb.SeedNamedEngagement(t, db, practiceB, "Client B", "b@example.com")
	contractA := seedDraftContract(t, db, engagementA)
	contractB := seedDraftContract(t, db, engagementB)
	invoiceA := seedInvoice(t, db, practiceA, contractA, "in_rls_a", invoiceStatusOpen, 5000, time.Now())
	seedInvoice(t, db, practiceB, contractB, "in_rls_b", invoiceStatusOpen, 7000, time.Now())

	tx := db.BeginAs(t, testdb.Practice(practiceA))
	visibleIDs := testdb.Column[string](t, tx, `SELECT id FROM invoices`)

	if len(visibleIDs) != 1 || visibleIDs[0] != invoiceA {
		t.Fatalf("visible invoices = %v, want only %q", visibleIDs, invoiceA)
	}
}

// TestRLS_InvoicesCannotInsertForAnotherPractice proves Practice B's
// session cannot insert an invoices row for Practice A -- the WITH CHECK
// side of the policy, derived from the same USING clause, rejects it.
func TestRLS_InvoicesCannotInsertForAnotherPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	_, engagementA := testdb.SeedNamedEngagement(t, db, practiceA, "Client A", "a@example.com")
	contractA := seedDraftContract(t, db, engagementA)

	tx := db.BeginAs(t, testdb.Practice(practiceB))
	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, amount_cents, reference, due_at) VALUES ($1, $2, 'in_rls_insert', 5000, 'in_rls_insert', now())`,
		practiceA, contractA,
	)
	if err == nil {
		t.Fatal("expected inserting an invoices row for another Practice to be rejected by RLS, got no error")
	}
}
