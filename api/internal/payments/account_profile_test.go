package payments_test

import (
	"testing"

	"doula-cloud/api/internal/payments"
)

// TestStatementDescriptor covers the rules Stripe enforces, each one
// established by a live create call against the Sandbox rather than read
// off a doc page (#442). The empty results are not failures: an account
// created without a descriptor is where every Practice stands today, and
// Stripe asks her for one in its own flow.
// ordinaryName is the name that passes through untouched, named once
// because it is both the input and the expected output.
const ordinaryName = "Rochester Doulas"

func TestStatementDescriptor(t *testing.T) {
	for _, tc := range []struct {
		name         string
		practiceName string
		want         string
	}{
		{"an ordinary name passes through", ordinaryName, ordinaryName},
		{"a name at the ceiling is kept whole", "Rochester Birth Care", "Rochester Birth Care"},
		{
			// Stripe refuses anything over 22 characters.
			"a long name is cut to the ceiling, not refused",
			"Rochester Birth and Postpartum Partners",
			"Rochester Birth and",
		},
		{
			// The cut can land on a space, and a descriptor ending in one
			// reads as a truncation on a card statement.
			"a cut landing on a space does not leave one",
			"Rochester Birth Partner Collective",
			"Rochester Birth",
		},
		{"the characters Stripe names are dropped", `Maya's "Birth" Doulas*`, "Mayas Birth Doulas"},
		{"dropping a character does not leave a double space", `Rochester * Doulas`, "Rochester Doulas"},
		{"a run of whitespace collapses", "Rochester   Doulas", "Rochester Doulas"},
		// Stripe: "must be at least 5 characters".
		{"a name too short for Stripe yields nothing", "Doe", ""},
		{"a name left too short by the drop yields nothing", `Ma'am`, ""},
		// Stripe: "must contain at least one Latin character".
		{"a name with no Latin letter yields nothing", "12345678", ""},
		{"a name in another script yields nothing", "助産師のケア", ""},
		{"an empty name yields nothing", "   ", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := payments.StatementDescriptor(tc.practiceName); got != tc.want {
				t.Fatalf("StatementDescriptor(%q) = %q, want %q", tc.practiceName, got, tc.want)
			}
		})
	}
}
