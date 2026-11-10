package paymentstanding_test

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	"doula-cloud/api/internal/paymentstanding"
	"doula-cloud/api/internal/testdb"
)

// TestMain terminates the shared Postgres container testdb.New starts for
// this test process -- see testdb.Main's doc comment.
func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

// paidInvoice seeds a by-hand Invoice paid by a 15000-cent check and
// returns that Payment's id and the Invoice's.
func paidInvoice(t *testing.T, db *testdb.DB, practiceID string) (invoiceID, paymentID string) {
	t.Helper()
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	contractID := testdb.Scalar[string](t, db.Admin,
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, 'signed', 'Test prose', 15000) RETURNING id`, engagementID)
	invoiceID = testdb.Scalar[string](t, db.Admin,
		`INSERT INTO invoices (practice_id, contract_id, status, amount_cents, currency, reference, due_at, paid_at)
		 VALUES ($1, $2, 'paid', 15000, 'usd', 'INV-STANDING', now(), now()) RETURNING id`, practiceID, contractID)
	paymentID = testdb.Scalar[string](t, db.Admin,
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method) VALUES ($1, 15000, now(), 'manual', 'check') RETURNING id`, invoiceID)
	return invoiceID, paymentID
}

func point(t *testing.T, db *testdb.DB, invoiceID, paymentID, kind string, amountCents int64) {
	t.Helper()
	var reason sql.NullString
	if kind == paymentstanding.KindReversal {
		reason = sql.NullString{String: "a reason", Valid: true}
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, reason)
		 VALUES ($1, $2, now(), $3::payment_kind, $4, $5)`,
		invoiceID, amountCents, kind, paymentID, reason); err != nil {
		t.Fatalf("insert %s: %v", kind, err)
	}
}

// TestRead_DerivesEachStanding is the one derivation, read for a Payment
// in each of its four standings, and the refunded total beside it.
func TestRead_DerivesEachStanding(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Standing Practice")

	_, livePayment := paidInvoice(t, db, practiceID)
	partlyInvoice, partlyPayment := paidInvoice(t, db, practiceID)
	point(t, db, partlyInvoice, partlyPayment, paymentstanding.KindRefund, -4000)
	point(t, db, partlyInvoice, partlyPayment, paymentstanding.KindRefund, -1000)
	fullyInvoice, fullyPayment := paidInvoice(t, db, practiceID)
	point(t, db, fullyInvoice, fullyPayment, paymentstanding.KindRefund, -15000)
	reversedInvoice, reversedPayment := paidInvoice(t, db, practiceID)
	point(t, db, reversedInvoice, reversedPayment, paymentstanding.KindReversal, -15000)

	cases := []struct {
		paymentID string
		standing  paymentstanding.Standing
		refunded  int64
	}{
		{livePayment, paymentstanding.Live, 0},
		{partlyPayment, paymentstanding.PartlyRefunded, 5000},
		{fullyPayment, paymentstanding.FullyRefunded, 15000},
		{reversedPayment, paymentstanding.Reversed, 0},
	}
	for _, tc := range cases {
		standing, refunded, err := paymentstanding.Read(t.Context(), db.Admin, tc.paymentID)
		if err != nil || standing != tc.standing || refunded != tc.refunded {
			t.Errorf("Read = (%s, %d, %v), want (%s, %d)", standing, refunded, err, tc.standing, tc.refunded)
		}
	}
}

// TestRead_ARowPointingAtAPaymentHasNoStanding: a reversal or Refund row
// is not a Payment, so it has no standing of its own to read.
func TestRead_ARowPointingAtAPaymentHasNoStanding(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "No Standing Practice")
	invoiceID, paymentID := paidInvoice(t, db, practiceID)
	point(t, db, invoiceID, paymentID, paymentstanding.KindRefund, -100)
	refundID := testdb.Scalar[string](t, db.Admin, `SELECT id FROM payments WHERE kind = 'refund' AND target_payment_id = $1`, paymentID)

	if _, _, err := paymentstanding.Read(t.Context(), db.Admin, refundID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Read(refund) err = %v, want sql.ErrNoRows", err)
	}
}
