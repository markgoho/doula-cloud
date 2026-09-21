package payments

import (
	"testing"

	"github.com/stripe/stripe-go/v86"
)

// refundCreditNoteParams and creditNoteReference are the two parts of
// IssueRefundCreditNote that can run without a real Stripe account --
// the same split accountStatusFrom makes for RetrieveAccount. These pin
// the two things they decide: which amount a credit note carries, and
// which id identifies the money it returned.

// TestRefundCreditNoteParams_CardRefundCarriesRefundAmount proves a
// Payment Stripe still holds is returned with refund_amount, so Stripe
// moves the money -- and never also with out_of_band_amount, which would
// claim the Practice returned it herself.
func TestRefundCreditNoteParams_CardRefundCarriesRefundAmount(t *testing.T) {
	p := refundCreditNoteParams("acct_1", "in_1", 2000, false)

	if p.StripeAccount == nil || *p.StripeAccount != "acct_1" {
		t.Fatalf("stripe account = %v, want acct_1 -- a credit note is made on the connected account", p.StripeAccount)
	}
	if p.Invoice == nil || *p.Invoice != "in_1" || p.Amount == nil || *p.Amount != 2000 {
		t.Fatalf("params = %+v, want invoice in_1 amount 2000", p)
	}
	if p.RefundAmount == nil || *p.RefundAmount != 2000 {
		t.Fatalf("refund_amount = %v, want 2000", p.RefundAmount)
	}
	if p.OutOfBandAmount != nil {
		t.Fatalf("out_of_band_amount = %d, want unset on a card refund", *p.OutOfBandAmount)
	}
}

// TestRefundCreditNoteParams_OutOfBandCarriesOutOfBandAmount is the
// mirror: money Stripe never held is recorded, not moved.
func TestRefundCreditNoteParams_OutOfBandCarriesOutOfBandAmount(t *testing.T) {
	p := refundCreditNoteParams("acct_1", "in_1", 50000, true)

	if p.OutOfBandAmount == nil || *p.OutOfBandAmount != 50000 {
		t.Fatalf("out_of_band_amount = %v, want 50000", p.OutOfBandAmount)
	}
	if p.RefundAmount != nil {
		t.Fatalf("refund_amount = %d, want unset -- Stripe holds no money to refund", *p.RefundAmount)
	}
}

// TestCreditNoteReference_PrefersTheRefundObject proves the reference is
// the Refund's id when the credit note made one -- the id refund.created
// also carries, which is what lets the webhook match the two -- and the
// credit note's own id when it made none.
func TestCreditNoteReference_PrefersTheRefundObject(t *testing.T) {
	withRefund := &stripe.CreditNote{ID: "cn_1", Refunds: []*stripe.CreditNoteRefund{
		nil,
		{Refund: nil},
		{Refund: &stripe.Refund{ID: "re_1"}},
	}}
	if got := creditNoteReference(withRefund); got != "re_1" {
		t.Fatalf("reference = %q, want re_1", got)
	}

	outOfBand := &stripe.CreditNote{ID: "cn_2"}
	if got := creditNoteReference(outOfBand); got != "cn_2" {
		t.Fatalf("reference = %q, want the credit note's own cn_2", got)
	}
}
