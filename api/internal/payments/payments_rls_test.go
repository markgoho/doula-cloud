package payments_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise the payments_practice_visibility policy from
// 00025_payments.sql directly via db.App and set_config, following the
// pattern in invoice_rls_test.go -- an EXISTS subquery through
// invoice_id -> invoices, the same shape as contracts_practice_visibility
// (00016_contracts.sql), since payments has no practice_id column of its
// own.

// seedPayment inserts a payments row directly, bypassing the webhook
// handler (this file only exercises the RLS policy itself), using the
// superuser Admin connection.
func seedPayment(t *testing.T, db *testdb.DB, invoiceID, stripePaymentReference string, amountCents int64, paidAt time.Time) (paymentID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at, kind) VALUES ($1, $2, $3, $4, 'stripe') RETURNING id`,
		invoiceID, stripePaymentReference, amountCents, paidAt,
	).Scan(&paymentID); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	return paymentID
}

// TestRLS_PaymentsFailsClosedWithNoSessionVarSet proves payments denies
// all rows when app.current_practice_id is unset.
func TestRLS_PaymentsFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	invoice := seedBilled(t, db, fixture.Practice(t, db), "in_rls_closed", invoiceStatusPaid, 5000)
	seedPayment(t, db, invoice.invoiceID, "pi_rls_closed", 5000, time.Now())

	count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM payments`)
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_PaymentsVisibilityIsScopedToCurrentPractice proves Practice A's
// session sees only Practice A's payments (via its Invoice's invoice_id
// -> practice_id chain) -- Practice B's stay invisible.
func TestRLS_PaymentsVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	invoiceA := seedBilled(t, db, practiceA, "in_rls_a", invoiceStatusPaid, 5000).invoiceID
	invoiceB := seedBilled(t, db, practiceB, "in_rls_b", invoiceStatusPaid, 7000).invoiceID
	paymentA := seedPayment(t, db, invoiceA, "pi_rls_a", 5000, time.Now())
	seedPayment(t, db, invoiceB, "pi_rls_b", 7000, time.Now())

	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))
	visibleIDs := testdb.Column[string](t, tx, `SELECT id FROM payments`)

	if len(visibleIDs) != 1 || visibleIDs[0] != paymentA {
		t.Fatalf("visible payments = %v, want only %q", visibleIDs, paymentA)
	}
}

// TestRLS_PaymentsCannotInsertForAnotherPracticesInvoice proves Practice
// B's session cannot insert a payments row against Practice A's Invoice --
// the WITH CHECK side of the policy, derived from the same USING clause,
// rejects it.
func TestRLS_PaymentsCannotInsertForAnotherPracticesInvoice(t *testing.T) {
	db := testdb.New(t)
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	invoiceA := seedBilled(t, db, practiceA, "in_rls_insert", invoiceStatusOpen, 5000).invoiceID

	tx := db.BeginAs(t, testdb.Practice(practiceB.ID))
	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at, kind) VALUES ($1, 'pi_rls_insert', 5000, now(), 'stripe')`,
		invoiceA,
	)
	if err == nil {
		t.Fatal("expected inserting a payments row against another Practice's Invoice to be rejected by RLS, got no error")
	}
}
