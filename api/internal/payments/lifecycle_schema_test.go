package payments_test

import (
	"strings"
	"testing"
	"time"
)

// TestAPaymentOnItsWayIsNeverOverdue is the one rule for "overdue" and
// "payment on its way" (overdue.Days and overdue.Expr): an open Invoice
// past its due day whose payment is clearing is not late. Before it, the
// row's DaysOverdue, the Practice's overdue total and its ?overdue=true
// list all counted it, while every Staff screen labeled it "Payment on its
// way" and the Clients list left it out. A late Invoice with nothing on
// its way beside it keeps the test from passing on an empty book.
func TestAPaymentOnItsWayIsNeverOverdue(t *testing.T) {
	now := overdueInstants[0]
	e := newEnv(t, atInstant(now))
	engagementID, contractID := e.contractWithStatus("draft")
	db, practiceID := e.DB, e.Practice.ID
	base := now.Add(-time.Hour)

	lateID := seedInvoiceDue(t, db, practiceID, contractID, "in_late_nothing_coming", invoiceStatusOpen, 15000,
		base, dueOnPracticeDay(t, now, -5))
	clearingID := seedInvoiceDue(t, db, practiceID, contractID, "in_late_but_clearing", invoiceStatusOpen, 22000,
		base.Add(time.Second), dueOnPracticeDay(t, now, -5))
	markProcessing(t, db, clearingID)

	whole := e.practiceInvoices()
	if whole.OverdueCents != 15000 || whole.OverdueCount != 1 {
		t.Fatalf("overdue totals = (%d, %d), want (15000, 1)", whole.OverdueCents, whole.OverdueCount)
	}
	if whole.OutstandingCount != 2 {
		t.Fatalf("outstanding count = %d, want 2 -- money on its way is still owed until it clears", whole.OutstandingCount)
	}
	narrowed := e.practiceInvoices("overdue=true")
	if len(narrowed.Items) != 1 || narrowed.Items[0].ID != lateID {
		t.Fatalf("overdue items = %+v, want only %q", narrowed.Items, lateID)
	}
	for _, it := range whole.Items {
		if it.ID == clearingID && (it.DaysOverdue != 0 || !it.PaymentProcessing) {
			t.Fatalf("Practice-wide row = %+v, want 0 days overdue with a payment on its way", it)
		}
	}
	for _, it := range e.listInvoices(engagementID).Items {
		if it.ID == clearingID && it.DaysOverdue != 0 {
			t.Fatalf("Engagement row daysOverdue = %d, want 0", it.DaysOverdue)
		}
	}
}

// TestInvoicesRefuseWhatNoMoveLeaves pins 00167's CHECKs: each row below
// is one no declared Transition can leave, and the database refuses it
// rather than storing it.
func TestInvoicesRefuseWhatNoMoveLeaves(t *testing.T) {
	cases := []struct {
		name       string
		stripeID   any
		status     string
		paidAt     any
		processing any
		constraint string
	}{
		{"paid with no paid_at", "in_ck_paid", "paid", nil, nil, "invoices_paid_at_only_while_paid"},
		{"open with a paid_at", "in_ck_open", "open", time.Now(), nil, "invoices_paid_at_only_while_paid"},
		{"a payment on its way on a void Invoice", "in_ck_void", "void", nil, time.Now(), "invoices_processing_only_while_open"},
		{"a payment on its way on a by-hand Invoice", nil, "open", nil, time.Now(), "invoices_processing_only_on_stripe"},
		{"a by-hand draft", nil, "draft", nil, nil, "invoices_draft_only_on_stripe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			_, contractID := e.contractWithStatus("draft")
			_, err := e.DB.Admin.ExecContext(t.Context(),
				`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, reference, due_at, paid_at, payment_processing_since)
				 VALUES ($1, $2, $3, $4::invoice_status, 5000, 'INV-CHECK', now(), $5, $6)`,
				e.Practice.ID, contractID, tc.stripeID, tc.status, tc.paidAt, tc.processing)
			if err == nil || !strings.Contains(err.Error(), tc.constraint) {
				t.Fatalf("insert err = %v, want a violation of %s", err, tc.constraint)
			}
		})
	}
}
