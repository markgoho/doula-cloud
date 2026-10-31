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
