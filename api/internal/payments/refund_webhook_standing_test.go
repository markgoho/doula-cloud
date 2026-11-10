package payments_test

import (
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/webhookarrival"
)

// newUnreferencedRefundFixture is newRefundWebhookFixture as invoice.paid
// leaves it when Stripe would not say which PaymentIntent paid: a card
// Payment whose reference is empty.
func newUnreferencedRefundFixture(t *testing.T, name string) refundWebhookFixture {
	t.Helper()
	db := testdb.New(t)
	f := refundWebhookFixture{db: db, accountID: "acct_" + name, stripeInv: "in_" + name, pi: "pi_" + name}
	practiceID := seedConnectedPractice(t, db, name, f.accountID)
	_, f.engagementID = testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, f.engagementID)
	f.invoiceID = seedInvoice(t, db, practiceID, contractID, f.stripeInv, invoiceStatusOpen, 50000, time.Now())
	f.paymentID = seedStripePayment(t, db, f.invoiceID, 50000, "")
	return f
}

// TestConnectWebhook_RefundFindsAPaymentWhoseReferenceWasNeverLearned is
// the bug the audit named: invoice.paid stores an empty reference when
// its Stripe lookup fails, and refund.created found its Payment only by
// that reference, so a Refund of such a Payment was dropped as belonging
// to nothing. It is now found through the Invoice its PaymentIntent pays.
func TestConnectWebhook_RefundFindsAPaymentWhoseReferenceWasNeverLearned(t *testing.T) {
	f := newUnreferencedRefundFixture(t, "unreferenced")
	client := newProcessingClient(payments.InvoicePaymentIntent{StripeInvoiceID: f.stripeInv}, nil)

	deliverThrough(t, f, client, refundPayload(t, "evt_unref", f.accountID, "re_unref", f.pi, "succeeded", 20000), http.StatusOK)

	got := refundRowsFor(t, f.db, f.invoiceID)
	if len(got) != 1 || got[0].amountCents != -20000 || *got[0].targetPaymentID != f.paymentID {
		t.Fatalf("refund rows = %+v, want one -20000 row against %s", got, f.paymentID)
	}
}

// TestConnectWebhook_RefundNeverTakesAnotherPaymentIntentsPayment: the
// fallback matches only a card Payment whose reference was never learned,
// so a PaymentIntent Stripe says pays this Invoice, but which is not the
// one its Payment recorded, attaches to nothing.
func TestConnectWebhook_RefundNeverTakesAnotherPaymentIntentsPayment(t *testing.T) {
	f := newRefundWebhookFixture(t, "other_pi")
	client := newProcessingClient(payments.InvoicePaymentIntent{StripeInvoiceID: f.stripeInv}, nil)

	deliverThrough(t, f, client, refundPayload(t, "evt_other_pi", f.accountID, "re_other_pi", "pi_someone_else", "succeeded", 100), http.StatusOK)

	if got := len(refundRowsFor(t, f.db, f.invoiceID)); got != 0 {
		t.Fatalf("refund rows = %d, want 0", got)
	}
	if client.calls != 1 {
		t.Fatalf("Stripe asked %d times, want once", client.calls)
	}
}

// TestConnectWebhook_RefundWhoseInvoiceStripeWillNotNameIsRetried: when
// Stripe cannot be asked which Invoice the PaymentIntent pays, the event
// is answered 500 and nothing is claimed, so Stripe redelivers it rather
// than the Refund being dropped as belonging to nothing.
func TestConnectWebhook_RefundWhoseInvoiceStripeWillNotNameIsRetried(t *testing.T) {
	f := newUnreferencedRefundFixture(t, "lookup_failed")
	client := newProcessingClient(payments.InvoicePaymentIntent{}, errors.New("stripe unavailable"))

	deliverThrough(t, f, client, refundPayload(t, "evt_lookup_failed", f.accountID, "re_lookup_failed", f.pi, "succeeded", 100),
		http.StatusInternalServerError)

	if got := len(refundRowsFor(t, f.db, f.invoiceID)); got != 0 {
		t.Fatalf("refund rows = %d, want 0", got)
	}
	if n := testdb.Scalar[int](t, f.db.Admin, `SELECT count(*) FROM stripe_webhook_events WHERE event_id = 'evt_lookup_failed'`); n != 0 {
		t.Fatalf("event claimed %d times, want unclaimed so a redelivery is processed", n)
	}
}

// TestConnectWebhook_RefundNamingNoPaymentIntentMatchesNothing: a Refund
// with no PaymentIntent must not match a Payment whose reference is the
// empty string, which would be any unreferenced card Payment's.
func TestConnectWebhook_RefundNamingNoPaymentIntentMatchesNothing(t *testing.T) {
	f := newUnreferencedRefundFixture(t, "no_pi")

	deliver(t, f, refundPayload(t, "evt_no_pi", f.accountID, "re_no_pi", "", "succeeded", 100))

	if got := len(refundRowsFor(t, f.db, f.invoiceID)); got != 0 {
		t.Fatalf("refund rows = %d, want 0", got)
	}
}

// TestConnectWebhook_RefundOnAnInvoiceThatIsNotPaidIsRefused is the
// declared divergence closed: the Staff Refund refuses an Invoice that is
// not paid, and the webhook path now asks TransitionStripeRefunded the
// same question. The two records already disagree, so the Refund is not
// written; it is acknowledged, and counted as refused on the Platform's
// webhook screen for a person to reconcile.
func TestConnectWebhook_RefundOnAnInvoiceThatIsNotPaidIsRefused(t *testing.T) {
	f := newRefundWebhookFixture(t, "not_paid")
	if _, err := f.db.Admin.ExecContext(t.Context(), `UPDATE invoices SET status = 'uncollectible', paid_at = NULL WHERE id = $1`, f.invoiceID); err != nil {
		t.Fatalf("write the Invoice off: %v", err)
	}

	deliver(t, f, creditNotePayload(t, "evt_not_paid", f.accountID, "cn_not_paid", f.stripeInv, 0, map[string]int64{"re_not_paid": 100}))

	if got := len(refundRowsFor(t, f.db, f.invoiceID)); got != 0 {
		t.Fatalf("refund rows = %d, want 0", got)
	}
	want := []arrival{{webhookarrival.Refused, "evt_not_paid", stripeEventTypeCreditNoteCreated}}
	if got := arrivalsOn(t, f.db, "stripe_connect"); !slices.Equal(got, want) {
		t.Fatalf("arrivals = %v, want %v", got, want)
	}
}
