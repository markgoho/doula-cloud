package payments_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise the invoices_practice_visibility policy from
// 00024_invoices.sql directly via db.App and set_config, following the
// pattern in billing/rls_test.go (credit_ledger's own direct-column
// practice_id policy, the shape invoices reuses).

// billed is what seedBilled leaves at a Practice: a Client with an
// Engagement, a signed Contract and one Invoice.
type billed struct {
	clientID   string
	contractID string
	invoiceID  string
}

// seedBilled seeds a new Client at p with an Engagement, a signed Contract
// and one Invoice in status.
func seedBilled(t *testing.T, db *testdb.DB, p fixture.PracticeRecord, stripeInvoiceID, status string, amountCents int64) billed {
	t.Helper()
	engagement := fixture.Engagement(t, db, p)
	contractID := seedSignedContract(t, db, engagement.ID)
	return billed{
		clientID:   engagement.Client.ID,
		contractID: contractID,
		invoiceID:  seedInvoice(t, db, p.ID, contractID, stripeInvoiceID, status, amountCents, time.Now()),
	}
}

// TestRLS_InvoicesFailsClosedWithNoSessionVarSet proves invoices denies
// all rows when app.current_practice_id is unset.
func TestRLS_InvoicesFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	seedBilled(t, db, fixture.Practice(t, db), "in_rls_closed", invoiceStatusOpen, 5000)

	count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM invoices`)
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_InvoicesVisibilityIsScopedToCurrentPractice proves Practice A's
// session sees only Practice A's invoices -- Practice B's stay invisible.
func TestRLS_InvoicesVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	invoiceA := seedBilled(t, db, practiceA, "in_rls_a", invoiceStatusOpen, 5000).invoiceID
	seedBilled(t, db, practiceB, "in_rls_b", invoiceStatusOpen, 7000)

	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))
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
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	contractA := seedDraftContract(t, db, fixture.Engagement(t, db, practiceA).ID)

	tx := db.BeginAs(t, testdb.Practice(practiceB.ID))
	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, amount_cents, reference, due_at) VALUES ($1, $2, 'in_rls_insert', 5000, 'in_rls_insert', now())`,
		practiceA.ID, contractA,
	)
	if err == nil {
		t.Fatal("expected inserting an invoices row for another Practice to be rejected by RLS, got no error")
	}
}
