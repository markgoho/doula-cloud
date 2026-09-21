package engagement

import (
	"database/sql"
	"testing"
)

// TestEntryDetail_RefundSaysWhereItCameFrom pins #1009's sentence: a
// Refund the Connect webhook recorded from the Practice's Stripe
// Dashboard says so, because its Who column reads "Doula Cloud" and the
// generic "Payment refunded" would read as Doula Cloud returning the
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
		{"an unreadable diff", `not json`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entryDetail("payment_refunded", []byte(tc.diff), none, none); got != tc.want {
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
