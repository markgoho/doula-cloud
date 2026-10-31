package engagement

import (
	"database/sql"
	"testing"
)

// unreadableDiff names the case every detail test shares: a diff that is
// not JSON gets no sentence.
const (
	unreadableDiff = "an unreadable diff"
	notJSON        = "not json"
)

// TestEntryDetail_RefundSaysWhereItCameFrom pins #1009's sentence: a
// Refund the Connect webhook recorded from the Practice's Stripe
// Dashboard says so, because its Who column reads "DoulaCloud" and the
// generic "Payment refunded" would read as DoulaCloud returning the
// money itself. A Refund an Owner or Admin issued here -- no origin --
// gets no sentence; her name is already in the Who column.
func TestEntryDetail_RefundSaysWhereItCameFrom(t *testing.T) {
	none := sql.NullString{}
	cases := []struct {
		name string
		diff string
		want string
	}{
		{"from the Stripe Dashboard", `{"amountCents":-2000,"origin":"stripe_dashboard"}`, "Payment refunded from the Practice's Stripe Dashboard"},
		{"issued here by Staff", `{"amountCents":-2000,"method":"check"}`, ""},
		{unreadableDiff, notJSON, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entryDetail("payment_refunded", []byte(tc.diff), none, none); got != tc.want {
				t.Fatalf("detail = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEntryDetail_FailedPaymentNamesTheMethod pins #2224's sentence: a
// failed attempt says which method was tried, in the Practice's words,
// and says nothing where Stripe named none or one it has no words for.
func TestEntryDetail_FailedPaymentNamesTheMethod(t *testing.T) {
	none := sql.NullString{}
	cases := []struct {
		name string
		diff string
		want string
	}{
		{"a declined card", `{"paymentMethodType":"card"}`, "Tried to pay by card"},
		{"a failed bank debit", `{"paymentMethodType":"us_bank_account"}`, "Tried to pay by bank account"},
		{"no method", `{}`, ""},
		{"a method with no words here", `{"paymentMethodType":"sepa_debit"}`, ""},
		{unreadableDiff, notJSON, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entryDetail("invoice_payment_failed", []byte(tc.diff), none, none); got != tc.want {
				t.Fatalf("detail = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEntryDetail_PaymentOnItsWayNamesTheMethod pins #2225's sentence: a
// payment on its way says which method it is coming by, in the words a
// failed attempt uses, so a bank payment that will take days reads as one.
func TestEntryDetail_PaymentOnItsWayNamesTheMethod(t *testing.T) {
	none := sql.NullString{}
	cases := []struct {
		name string
		diff string
		want string
	}{
		{"a bank payment", `{"paymentMethodType":"us_bank_account"}`, "Paying by bank account"},
		{"no method", `{}`, ""},
		{unreadableDiff, notJSON, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entryDetail("invoice_payment_processing", []byte(tc.diff), none, none); got != tc.want {
				t.Fatalf("detail = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEntryDetail_WrittenOffSaysWhereItWasDone pins #2224's sentence for
// an Invoice Stripe marked uncollectible: its Who column reads
// "DoulaCloud", so the row says it was done in the Practice's Stripe
// account. One written off here by an Owner or Admin gets none.
func TestEntryDetail_WrittenOffSaysWhereItWasDone(t *testing.T) {
	none := sql.NullString{}
	cases := []struct {
		name string
		diff string
		want string
	}{
		{"marked in Stripe", `{"origin":"stripe_dashboard"}`, "Marked as uncollectible in the Practice's Stripe account"},
		{"written off here", `{}`, ""},
		{unreadableDiff, notJSON, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entryDetail("invoice_written_off", []byte(tc.diff), none, none); got != tc.want {
				t.Fatalf("detail = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEntryDetail_OtherActionsFallThrough proves the dispatch leaves
// every other action to reassignmentDetail, which returns nothing for an
// action it does not own.
func TestEntryDetail_OtherActionsFallThrough(t *testing.T) {
	if got := entryDetail("invoice_raised", []byte(`{"origin":"stripe_dashboard"}`), sql.NullString{}, sql.NullString{}); got != "" {
		t.Fatalf("detail = %q, want none for an action that is not a Refund", got)
	}
}
