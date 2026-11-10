package payments_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/webhookarrival"
)

const stripeEventTypeInvoiceVoided = "invoice.voided"

// invoiceVoidedPayload builds a raw invoice.voided event body referencing
// stripeInvoiceID.
func invoiceVoidedPayload(t *testing.T, eventID, accountID, stripeInvoiceID string) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeInvoiceVoided, accountID, map[string]any{
		"id":      stripeInvoiceID,
		objectKey: stripeObjectInvoice,
	})
}

// outcomeOf is the one webhook_arrivals outcome recorded for eventID.
func outcomeOf(t *testing.T, db *testdb.DB, eventID string) webhookarrival.Outcome {
	t.Helper()
	for _, a := range testdb.WebhookArrivals(t, db) {
		if a.EventID == eventID {
			return webhookarrival.Outcome(a.Outcome)
		}
	}
	t.Fatalf("no arrival recorded for %s", eventID)
	return ""
}

// TestPostConnectWebhookHandler_InvoicePaidFollowsTheTable is
// TransitionStripePaid, stage by stage. Before the table, invoice.paid
// skipped only an Invoice already paid, so it moved a void one to paid.
// Money Stripe says moved on an uncollectible Invoice, or on a draft whose
// finalization DoulaCloud never heard back about, is recorded. A void
// Invoice is refused: left void with no Payment, acknowledged with a 200
// so Stripe does not redeliver it, and counted as refused for a person to
// reconcile.
func TestPostConnectWebhookHandler_InvoicePaidFollowsTheTable(t *testing.T) {
	cases := []struct {
		from         string
		wantStatus   string
		wantPayments int
		wantOutcome  webhookarrival.Outcome
	}{
		{invoiceStatusUncollectible, invoiceStatusPaid, 1, webhookarrival.Accepted},
		{invoiceStatusDraft, invoiceStatusPaid, 1, webhookarrival.Accepted},
		{invoiceStatusVoid, invoiceStatusVoid, 0, webhookarrival.Refused},
	}
	for _, tc := range cases {
		t.Run("from "+tc.from, func(t *testing.T) {
			e := newEnv(t, connectAccountOnly())
			_, invoiceID := e.seedStripeInvoice("in_paid_from_"+tc.from, tc.from, 5000)
			srv := e.connectHook(newReferenceClient("pi_from_" + tc.from))

			deliverConnect(t, srv, invoicePaidPayload(t, "evt_paid_from_"+tc.from, e.AccountID, "in_paid_from_"+tc.from, time.Now()))

			if status, _ := invoiceStatusAndPaidAt(t, e.DB, invoiceID); status != tc.wantStatus {
				t.Fatalf("status = %q, want %q", status, tc.wantStatus)
			}
			if got := len(paymentsForInvoice(t, e.DB, invoiceID)); got != tc.wantPayments {
				t.Fatalf("payments = %d, want %d", got, tc.wantPayments)
			}
			if got := outcomeOf(t, e.DB, "evt_paid_from_"+tc.from); got != tc.wantOutcome {
				t.Fatalf("arrival outcome = %q, want %q", got, tc.wantOutcome)
			}
		})
	}
}

// TestPostConnectWebhookHandler_VoidedVoidsAnOpenInvoice is the handler
// the Connect route lacked: the Practice voids a Stripe-backed Invoice in
// her Stripe Dashboard, where DoulaCloud sends her to do it, and it stops
// being owed here. Any payment that was on its way is cleared with it, the
// row is DoulaCloud's with the Dashboard as its origin, and a replay of
// the same event applies nothing twice.
func TestPostConnectWebhookHandler_VoidedVoidsAnOpenInvoice(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	engagementID, invoiceID := e.seedStripeInvoice("in_voided", invoiceStatusOpen, 5000)
	markProcessing(t, e.DB, invoiceID)
	srv := e.connectHook(newReferenceClient(""))
	payload := invoiceVoidedPayload(t, "evt_voided", e.AccountID, "in_voided")

	deliverConnect(t, srv, payload)
	deliverConnect(t, srv, payload)

	if status, _ := invoiceStatusAndPaidAt(t, e.DB, invoiceID); status != invoiceStatusVoid {
		t.Fatalf("status = %q, want %q", status, invoiceStatusVoid)
	}
	if processing := testdb.Scalar[bool](t, e.DB.Admin, `SELECT payment_processing_since IS NOT NULL FROM invoices WHERE id = $1`, invoiceID); processing {
		t.Fatal("payment_processing_since still set on a void Invoice")
	}
	got := activityRows(t, e.DB, "invoice_voided", engagementID)
	if len(got) != 1 {
		t.Fatalf("invoice_voided rows = %d, want 1 after a replay", len(got))
	}
	if got[0].actorKind != "system" || got[0].diff["origin"] != "stripe_dashboard" {
		t.Fatalf("row = %+v, want DoulaCloud's, from the Stripe Dashboard", got[0])
	}
}

// TestPostConnectWebhookHandler_VoidedOnAVoidInvoiceIsARedelivery tells
// the two ways invoice.voided changes nothing apart: an Invoice already
// void is a redelivery under another event id, accepted; a paid one is a
// move the table refuses, counted as refused. Neither gains a row.
func TestPostConnectWebhookHandler_VoidedOnAVoidInvoiceIsARedelivery(t *testing.T) {
	cases := []struct {
		from        string
		wantOutcome webhookarrival.Outcome
	}{
		{invoiceStatusVoid, webhookarrival.Accepted},
		{invoiceStatusPaid, webhookarrival.Refused},
	}
	for _, tc := range cases {
		t.Run("from "+tc.from, func(t *testing.T) {
			e := newEnv(t, connectAccountOnly())
			engagementID, invoiceID := e.seedStripeInvoice("in_voided_"+tc.from, tc.from, 5000)
			srv := e.connectHook(newReferenceClient(""))

			deliverConnect(t, srv, invoiceVoidedPayload(t, "evt_voided_"+tc.from, e.AccountID, "in_voided_"+tc.from))

			if status, _ := invoiceStatusAndPaidAt(t, e.DB, invoiceID); status != tc.from {
				t.Fatalf("status = %q, want %q unchanged", status, tc.from)
			}
			if got := activityRows(t, e.DB, "invoice_voided", engagementID); len(got) != 0 {
				t.Fatalf("invoice_voided rows = %d, want 0", len(got))
			}
			if got := outcomeOf(t, e.DB, "evt_voided_"+tc.from); got != tc.wantOutcome {
				t.Fatalf("arrival outcome = %q, want %q", got, tc.wantOutcome)
			}
		})
	}
}
