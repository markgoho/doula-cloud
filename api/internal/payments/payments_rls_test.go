package payments_test

import (
	"testing"
	"time"

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
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at) VALUES ($1, $2, $3, $4) RETURNING id`,
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
	practiceID := seedPractice(t, db, "Some Practice")
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_rls_closed", "paid", 5000, time.Now())
	seedPayment(t, db, invoiceID, "pi_rls_closed", 5000, time.Now())

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM payments`).Scan(&count); err != nil {
		t.Fatalf("query payments with no session variables set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_PaymentsVisibilityIsScopedToCurrentPractice proves Practice A's
// session sees only Practice A's payments (via its Invoice's invoice_id
// -> practice_id chain) -- Practice B's stay invisible.
func TestRLS_PaymentsVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	engagementA := seedEngagement(t, db, practiceA, "Client A", "a@example.com")
	engagementB := seedEngagement(t, db, practiceB, "Client B", "b@example.com")
	contractA := seedContract(t, db, engagementA)
	contractB := seedContract(t, db, engagementB)
	invoiceA := seedInvoice(t, db, practiceA, contractA, "in_rls_a", "paid", 5000, time.Now())
	invoiceB := seedInvoice(t, db, practiceB, contractB, "in_rls_b", "paid", 7000, time.Now())
	paymentA := seedPayment(t, db, invoiceA, "pi_rls_a", 5000, time.Now())
	seedPayment(t, db, invoiceB, "pi_rls_b", 7000, time.Now())

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM payments`)
	if err != nil {
		t.Fatalf("query payments: %v", err)
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
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	engagementA := seedEngagement(t, db, practiceA, "Client A", "a@example.com")
	contractA := seedContract(t, db, engagementA)
	invoiceA := seedInvoice(t, db, practiceA, contractA, "in_rls_insert", invoiceStatusOpen, 5000, time.Now())

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceB); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	_, err = tx.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at) VALUES ($1, 'pi_rls_insert', 5000, now())`,
		invoiceA,
	)
	if err == nil {
		t.Fatal("expected inserting a payments row against another Practice's Invoice to be rejected by RLS, got no error")
	}
}
