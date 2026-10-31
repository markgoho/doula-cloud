package payments_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

const (
	stripeEventTypePaymentIntentProcessing = "payment_intent.processing"
	stripeObjectPaymentIntent              = "payment_intent"
	paymentIntentStatusProcessing          = "processing"
	actionInvoicePaymentProcessing         = "invoice_payment_processing"
	statusKey                              = "status"
)

// processingClient answers RetrievePaymentIntentInvoice -- the lookup
// from a PaymentIntent to the Invoice it pays -- over referenceClient's
// real signature verification and its stubbed invoice.paid and
// invoice.payment_failed lookups, so one server can carry a payment from
// on its way to paid or failed.
type processingClient struct {
	*referenceClient
	intent payments.InvoicePaymentIntent
	err    error
	calls  int
}

func (c *processingClient) RetrievePaymentIntentInvoice(_ context.Context, _, _ string) (payments.InvoicePaymentIntent, error) {
	c.calls++
	if c.err != nil {
		return payments.InvoicePaymentIntent{}, c.err
	}
	return c.intent, nil
}

func newProcessingClient(intent payments.InvoicePaymentIntent, err error) *processingClient {
	return &processingClient{
		referenceClient: &referenceClient{StripeAPIClient: payments.NewStripeAPIClient("sk_test_unused", "https://app.test")},
		intent:          intent,
		err:             err,
	}
}

// paymentIntentProcessingPayload builds a raw payment_intent.processing
// event body. Under API version 2026-07-29.dahlia a PaymentIntent names
// no Invoice, so the fixture carries none either: the handler has to ask
// Stripe which Invoice it pays, and a fixture that handed it one would
// let it pass a test it could not pass against Stripe.
func paymentIntentProcessingPayload(t *testing.T, eventID, accountID, paymentIntentID string) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypePaymentIntentProcessing, accountID, map[string]any{
		"id":      paymentIntentID,
		objectKey: stripeObjectPaymentIntent,
		statusKey: paymentIntentStatusProcessing,
	})
}

func processingSince(t *testing.T, db *testdb.DB, invoiceID string) sql.NullTime {
	t.Helper()
	var since sql.NullTime
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT payment_processing_since FROM invoices WHERE id = $1`, invoiceID,
	).Scan(&since); err != nil {
		t.Fatalf("read payment_processing_since: %v", err)
	}
	return since
}

// processingFixture is one Stripe-backed open Invoice on a connected
// Practice, ready for a payment to start on it.
type processingFixture struct {
	db           *testdb.DB
	account      string
	stripeID     string
	invoiceID    string
	clientID     string
	engagementID string
}

func seedProcessingFixture(t *testing.T, name, status string) processingFixture {
	t.Helper()
	db := testdb.New(t)
	f := processingFixture{db: db, account: "acct_" + name, stripeID: "in_" + name}
	practiceID := seedConnectedPractice(t, db, name+" Practice", f.account)
	f.clientID, f.engagementID = testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, f.engagementID)
	f.invoiceID = seedInvoice(t, db, practiceID, contractID, f.stripeID, status, 5000, time.Now())
	return f
}

func (f processingFixture) intent(status string) payments.InvoicePaymentIntent {
	return payments.InvoicePaymentIntent{StripeInvoiceID: f.stripeID, Status: status, PaymentMethodType: methodUSBankAccount}
}

// deliverConnect posts one signed Connect event and expects it acknowledged.
func deliverConnect(t *testing.T, srv *httptest.Server, payload []byte) {
	t.Helper()
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestPostConnectWebhookHandler_ProcessingMarksThePaymentOnItsWay is
// #2225. A bank debit takes days to clear and Stripe keeps the Invoice
// open meanwhile, so the status stays open and the payment on its way is
// held beside it, with the attempt -- hers, by the method she used --
// recorded on the Engagement's ledger.
func TestPostConnectWebhookHandler_ProcessingMarksThePaymentOnItsWay(t *testing.T) {
	f := seedProcessingFixture(t, "processing_marks", invoiceStatusOpen)
	srv := newConnectWebhookServerWith(f.db, newProcessingClient(f.intent(paymentIntentStatusProcessing), nil))
	defer srv.Close()

	deliverConnect(t, srv, paymentIntentProcessingPayload(t, "evt_processing_marks", f.account, "pi_processing_marks"))

	if status, _ := invoiceStatusAndPaidAt(t, f.db, f.invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want %q -- Stripe keeps it open while it settles", status, invoiceStatusOpen)
	}
	if since := processingSince(t, f.db, f.invoiceID); !since.Valid {
		t.Fatal("payment_processing_since is NULL, want the moment the payment started on its way")
	}
	if rows := paymentsForInvoice(t, f.db, f.invoiceID); len(rows) != 0 {
		t.Fatalf("payments for invoice = %d, want 0 -- no money has arrived yet", len(rows))
	}
	got := activityRows(t, f.db, actionInvoicePaymentProcessing, f.engagementID)
	if len(got) != 1 {
		t.Fatalf("%s rows = %d, want 1", actionInvoicePaymentProcessing, len(got))
	}
	if got[0].actorKind != "client" || got[0].clientID.String != f.clientID {
		t.Fatalf("actor = %s/%v, want the Client %s -- the payment was hers", got[0].actorKind, got[0].clientID, f.clientID)
	}
	if got[0].diff["paymentMethodType"] != methodUSBankAccount {
		t.Fatalf("diff = %v, want paymentMethodType %q", got[0].diff, methodUSBankAccount)
	}
}

// TestPostConnectWebhookHandler_ProcessingWithoutAMethodStillRecorded
// proves a PaymentIntent that names no method still marks the payment
// and records it, with no method in the diff rather than an empty one.
func TestPostConnectWebhookHandler_ProcessingWithoutAMethodStillRecorded(t *testing.T) {
	f := seedProcessingFixture(t, "processing_no_method", invoiceStatusOpen)
	intent := f.intent(paymentIntentStatusProcessing)
	intent.PaymentMethodType = ""
	srv := newConnectWebhookServerWith(f.db, newProcessingClient(intent, nil))
	defer srv.Close()

	deliverConnect(t, srv, paymentIntentProcessingPayload(t, "evt_processing_no_method", f.account, "pi_no_method"))

	got := activityRows(t, f.db, actionInvoicePaymentProcessing, f.engagementID)
	if len(got) != 1 {
		t.Fatalf("%s rows = %d, want 1", actionInvoicePaymentProcessing, len(got))
	}
	if _, ok := got[0].diff["paymentMethodType"]; ok {
		t.Fatalf("diff = %v, want no method", got[0].diff)
	}
}

// TestPostConnectWebhookHandler_ProcessingThatHasSinceSettledIsLeft
// covers a delivery that arrives after the payment already finished. The
// handler asks Stripe for the PaymentIntent as it is now, so a payment
// that has since succeeded or failed is never marked on its way after
// the event that ended it.
func TestPostConnectWebhookHandler_ProcessingThatHasSinceSettledIsLeft(t *testing.T) {
	for _, now := range []string{"succeeded", "requires_payment_method"} {
		t.Run(now, func(t *testing.T) {
			f := seedProcessingFixture(t, "processing_settled_"+now, invoiceStatusOpen)
			srv := newConnectWebhookServerWith(f.db, newProcessingClient(f.intent(now), nil))
			defer srv.Close()

			deliverConnect(t, srv, paymentIntentProcessingPayload(t, "evt_settled_"+now, f.account, "pi_settled"))

			if since := processingSince(t, f.db, f.invoiceID); since.Valid {
				t.Fatalf("payment_processing_since = %v, want NULL for a PaymentIntent now %s", since.Time, now)
			}
			if got := activityRows(t, f.db, actionInvoicePaymentProcessing, f.engagementID); len(got) != 0 {
				t.Fatalf("%s rows = %d, want 0", actionInvoicePaymentProcessing, len(got))
			}
		})
	}
}

// TestPostConnectWebhookHandler_ProcessingOnASettledInvoiceIsLeft covers
// an Invoice that is no longer open here -- paid by hand, say -- when a
// payment on it is reported on its way: nothing moves.
func TestPostConnectWebhookHandler_ProcessingOnASettledInvoiceIsLeft(t *testing.T) {
	f := seedProcessingFixture(t, "processing_paid_invoice", invoiceStatusPaid)
	srv := newConnectWebhookServerWith(f.db, newProcessingClient(f.intent(paymentIntentStatusProcessing), nil))
	defer srv.Close()

	deliverConnect(t, srv, paymentIntentProcessingPayload(t, "evt_processing_paid_invoice", f.account, "pi_paid_invoice"))

	if since := processingSince(t, f.db, f.invoiceID); since.Valid {
		t.Fatalf("payment_processing_since = %v, want NULL on a paid Invoice", since.Time)
	}
	if got := activityRows(t, f.db, actionInvoicePaymentProcessing, f.engagementID); len(got) != 0 {
		t.Fatalf("%s rows = %d, want 0", actionInvoicePaymentProcessing, len(got))
	}
}

// TestPostConnectWebhookHandler_ProcessingForNoInvoiceDropped covers a
// PaymentIntent that pays no Invoice of ours -- Stripe lists no
// InvoicePayment for it, or names an Invoice DoulaCloud does not hold --
// which is logged and acknowledged, the rule every Invoice event follows.
func TestPostConnectWebhookHandler_ProcessingForNoInvoiceDropped(t *testing.T) {
	cases := map[string]string{"no invoice payment": "", "an unknown invoice": "in_not_ours"}
	for name, stripeID := range cases {
		t.Run(name, func(t *testing.T) {
			f := seedProcessingFixture(t, "processing_unknown", invoiceStatusOpen)
			intent := f.intent(paymentIntentStatusProcessing)
			intent.StripeInvoiceID = stripeID
			srv := newConnectWebhookServerWith(f.db, newProcessingClient(intent, nil))
			defer srv.Close()

			deliverConnect(t, srv, paymentIntentProcessingPayload(t, "evt_processing_unknown", f.account, "pi_unknown"))

			if since := processingSince(t, f.db, f.invoiceID); since.Valid {
				t.Fatal("payment_processing_since set on an Invoice the payment does not pay")
			}
		})
	}
}

// TestPostConnectWebhookHandler_ProcessingLookupFailureIsRetried proves
// that a failure to ask Stripe which Invoice a payment is for answers 500
// and claims nothing, so Stripe's retry is applied rather than swallowed
// as a replay -- unlike invoice.payment_failed's method, the Invoice
// itself cannot be done without.
func TestPostConnectWebhookHandler_ProcessingLookupFailureIsRetried(t *testing.T) {
	f := seedProcessingFixture(t, "processing_retry", invoiceStatusOpen)
	client := newProcessingClient(f.intent(paymentIntentStatusProcessing), errRetrieveFailed)
	srv := newConnectWebhookServerWith(f.db, client)
	defer srv.Close()
	payload := paymentIntentProcessingPayload(t, "evt_processing_retry", f.account, "pi_retry")

	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	client.err = nil
	deliverConnect(t, srv, payload)
	if since := processingSince(t, f.db, f.invoiceID); !since.Valid {
		t.Fatal("payment_processing_since is NULL after the retry, want it set")
	}
}

// TestPostConnectWebhookHandler_ProcessingReplayIsNoOp proves replaying
// one payment_intent.processing event records the payment once.
func TestPostConnectWebhookHandler_ProcessingReplayIsNoOp(t *testing.T) {
	f := seedProcessingFixture(t, "processing_replay", invoiceStatusOpen)
	srv := newConnectWebhookServerWith(f.db, newProcessingClient(f.intent(paymentIntentStatusProcessing), nil))
	defer srv.Close()
	payload := paymentIntentProcessingPayload(t, "evt_processing_replay", f.account, "pi_replay")

	for range 2 {
		deliverConnect(t, srv, payload)
	}
	if got := activityRows(t, f.db, actionInvoicePaymentProcessing, f.engagementID); len(got) != 1 {
		t.Fatalf("%s rows = %d, want 1 after a replay", actionInvoicePaymentProcessing, len(got))
	}
}

// TestPostConnectWebhookHandler_ProcessingMalformedObjectRejected proves
// a payment_intent.processing body whose object will not parse is a 500,
// not a silent acknowledgment.
func TestPostConnectWebhookHandler_ProcessingMalformedObjectRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServerWith(db, newProcessingClient(payments.InvoicePaymentIntent{}, nil))
	defer srv.Close()
	payload := buildConnectEventPayload(t, "evt_processing_malformed", stripeEventTypePaymentIntentProcessing, "acct_malformed", map[string]any{
		"id": 42,
	})

	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// TestPostConnectWebhookHandler_APaymentOnItsWayEnds proves the pending
// state ends in paid or failed (#2225's third box): invoice.paid,
// invoice.payment_failed and invoice.marked_uncollectible each clear it.
func TestPostConnectWebhookHandler_APaymentOnItsWayEnds(t *testing.T) {
	cases := []struct {
		name    string
		payload func(t *testing.T, f processingFixture) []byte
		status  string
	}{
		{"paid", func(t *testing.T, f processingFixture) []byte {
			return invoicePaidPayload(t, "evt_end_paid", f.account, f.stripeID, time.Now())
		}, invoiceStatusPaid},
		{"failed", func(t *testing.T, f processingFixture) []byte {
			return invoicePaymentFailedPayload(t, "evt_end_failed", f.account, f.stripeID)
		}, invoiceStatusOpen},
		{"written off", func(t *testing.T, f processingFixture) []byte {
			return invoiceMarkedUncollectiblePayload(t, "evt_end_written_off", f.account, f.stripeID)
		}, "uncollectible"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := seedProcessingFixture(t, "processing_ends", invoiceStatusOpen)
			srv := newConnectWebhookServerWith(f.db, newProcessingClient(f.intent(paymentIntentStatusProcessing), nil))
			defer srv.Close()

			deliverConnect(t, srv, paymentIntentProcessingPayload(t, "evt_end_start", f.account, "pi_end"))
			deliverConnect(t, srv, tc.payload(t, f))

			if since := processingSince(t, f.db, f.invoiceID); since.Valid {
				t.Fatalf("payment_processing_since = %v after %s, want NULL", since.Time, tc.name)
			}
			if status, _ := invoiceStatusAndPaidAt(t, f.db, f.invoiceID); status != tc.status {
				t.Fatalf("invoice status = %q, want %q", status, tc.status)
			}
		})
	}
}
