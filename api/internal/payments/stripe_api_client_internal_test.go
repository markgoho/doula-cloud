package payments

import (
	"testing"

	"github.com/stripe/stripe-go/v86"
)

// TestFailedPaymentMethodType reads the method off an InvoicePayment's
// expanded PaymentIntent the way Stripe reports a failed attempt, and
// reads nothing where any link in that chain is missing (#2224).
func TestFailedPaymentMethodType(t *testing.T) {
	withError := func(e *stripe.Error) *stripe.InvoicePayment {
		return &stripe.InvoicePayment{Payment: &stripe.InvoicePaymentPayment{
			PaymentIntent: &stripe.PaymentIntent{LastPaymentError: e},
		}}
	}
	cases := []struct {
		name    string
		payment *stripe.InvoicePayment
		want    string
	}{
		{"a failed bank debit", withError(&stripe.Error{PaymentMethod: &stripe.PaymentMethod{Type: stripe.PaymentMethodTypeUSBankAccount}}), "us_bank_account"},
		{"no payment", &stripe.InvoicePayment{}, ""},
		{"no PaymentIntent", &stripe.InvoicePayment{Payment: &stripe.InvoicePaymentPayment{}}, ""},
		{"no failure", withError(nil), ""},
		{"a failure naming no method", withError(&stripe.Error{}), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := failedPaymentMethodType(tc.payment); got != tc.want {
				t.Fatalf("failedPaymentMethodType = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFailedAttemptOf tells an attempt waiting on her from one that
// failed (#2225): the newest PaymentIntent requiring action is a
// redirect method she has not finished, and the method comes from the
// most recent payment that failed.
func TestFailedAttemptOf(t *testing.T) {
	withIntent := func(intent *stripe.PaymentIntent) *stripe.InvoicePayment {
		return &stripe.InvoicePayment{Payment: &stripe.InvoicePaymentPayment{PaymentIntent: intent}}
	}
	declined := withIntent(&stripe.PaymentIntent{
		Status:           stripe.PaymentIntentStatusRequiresPaymentMethod,
		LastPaymentError: &stripe.Error{PaymentMethod: &stripe.PaymentMethod{Type: stripe.PaymentMethodTypeCard}},
	})
	awaiting := withIntent(&stripe.PaymentIntent{Status: stripe.PaymentIntentStatusRequiresAction})
	cases := []struct {
		name     string
		payments []*stripe.InvoicePayment
		want     FailedAttempt
	}{
		{"a declined card", []*stripe.InvoicePayment{declined}, FailedAttempt{PaymentMethodType: "card"}},
		{"a redirect she has not finished", []*stripe.InvoicePayment{awaiting}, FailedAttempt{AwaitingAction: true}},
		{"a redirect after a decline", []*stripe.InvoicePayment{awaiting, declined}, FailedAttempt{PaymentMethodType: "card", AwaitingAction: true}},
		{"no payments", nil, FailedAttempt{}},
		{"a payment with no PaymentIntent", []*stripe.InvoicePayment{{}}, FailedAttempt{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := failedAttemptOf(tc.payments); got != tc.want {
				t.Fatalf("failedAttemptOf = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestInvoicePaymentIntentOf reads the Invoice, the PaymentIntent's
// current status and its method off one InvoicePayment as the
// processing webhook's lookup returns it (#2225), and leaves blank
// whatever link in that chain is missing.
func TestInvoicePaymentIntentOf(t *testing.T) {
	const invoiceID = "in_bank"
	invoice := &stripe.Invoice{ID: invoiceID}
	withIntent := func(intent *stripe.PaymentIntent) *stripe.InvoicePayment {
		return &stripe.InvoicePayment{Invoice: invoice, Payment: &stripe.InvoicePaymentPayment{PaymentIntent: intent}}
	}
	cases := []struct {
		name    string
		payment *stripe.InvoicePayment
		want    InvoicePaymentIntent
	}{
		{"a bank debit on its way", withIntent(&stripe.PaymentIntent{
			Status:        stripe.PaymentIntentStatusProcessing,
			PaymentMethod: &stripe.PaymentMethod{Type: stripe.PaymentMethodTypeUSBankAccount},
		}), InvoicePaymentIntent{StripeInvoiceID: invoiceID, Status: "processing", PaymentMethodType: "us_bank_account"}},
		{"no method", withIntent(&stripe.PaymentIntent{Status: stripe.PaymentIntentStatusSucceeded}),
			InvoicePaymentIntent{StripeInvoiceID: invoiceID, Status: "succeeded"}},
		{"no PaymentIntent", &stripe.InvoicePayment{Invoice: invoice, Payment: &stripe.InvoicePaymentPayment{}},
			InvoicePaymentIntent{StripeInvoiceID: invoiceID}},
		{"nothing", &stripe.InvoicePayment{}, InvoicePaymentIntent{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := invoicePaymentIntentOf(tc.payment); got != tc.want {
				t.Fatalf("invoicePaymentIntentOf = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestPaymentIntentInvoiceParams pins the reverse lookup verified in the
// Sandbox (#2225): the InvoicePayment list filtered by one PaymentIntent
// on the connected account, with the PaymentIntent's method expanded.
func TestPaymentIntentInvoiceParams(t *testing.T) {
	params := paymentIntentInvoiceParams("acct_x", "pi_x")
	if *params.StripeAccount != "acct_x" || *params.Payment.Type != "payment_intent" || *params.Payment.PaymentIntent != "pi_x" {
		t.Fatalf("params = %+v, want the PaymentIntent filter on acct_x", params)
	}
	if len(params.Expand) != 1 || *params.Expand[0] != "data.payment.payment_intent.payment_method" {
		t.Fatalf("expand = %v, want the PaymentIntent's method", params.Expand)
	}
}
