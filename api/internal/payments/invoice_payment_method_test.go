package payments_test

import (
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

const recordedNote = "Bank transfer from Imani Brooks, reference IB-0950"

// paidByHandEngagement is one by-hand Invoice paid by a bank transfer with
// a note, as Record payment leaves it (#271), on a new Engagement.
func (e *env) paidByHandEngagement(opts ...fixture.EngagementOption) (engagementID string) {
	e.T.Helper()
	engagementID, invoiceID := e.byHandInvoiceFor(opts...)
	seedManualPayment(e.T, e.DB, invoiceID, "bank_transfer", recordedNote)
	if _, err := e.DB.Admin.ExecContext(e.T.Context(),
		`UPDATE invoices SET status = 'paid', paid_at = now() WHERE id = $1`, invoiceID); err != nil {
		e.T.Fatalf("mark invoice paid: %v", err)
	}
	return engagementID
}

// TestGetInvoicesHandler_StaffWhoReadMoneyReadHowItWasPaid proves #2241:
// an Owner, an Admin and an employee Doula, who read payment history under
// ADR-0008's money rule, read a by-hand Payment's method and note on the
// Invoice.
func TestGetInvoicesHandler_StaffWhoReadMoneyReadHowItWasPaid(t *testing.T) {
	cases := []struct {
		name       string
		who        fixture.StaffOption
		isAttached bool
	}{
		{"owner", fixture.Owner(), false},
		{adminRole, fixture.Admin(), false},
		{"employee doula", fixture.Doula(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.who))
			var opts []fixture.EngagementOption
			if tc.isAttached {
				opts = append(opts, fixture.WithDoula(e.Staff))
			}
			engagementID := e.paidByHandEngagement(opts...)

			out := e.listInvoices(engagementID)

			if len(out.Items) != 1 {
				t.Fatalf("items = %d, want 1", len(out.Items))
			}
			got := out.Items[0]
			if got.ActivePaymentMethod != "bank_transfer" || got.ActivePaymentNote != recordedNote {
				t.Fatalf("method %q note %q, want bank_transfer and %q", got.ActivePaymentMethod, got.ActivePaymentNote, recordedNote)
			}
		})
	}
}

// TestGetInvoicesHandler_ACardPaymentCarriesNoMethod proves the method and
// note belong to a by-hand Payment only: a card Payment Stripe collected
// has neither.
func TestGetInvoicesHandler_ACardPaymentCarriesNoMethod(t *testing.T) {
	e := newEnv(t)
	engagementID, invoiceID := e.byHandInvoice()
	seedStripePayment(t, e.DB, invoiceID, seedByHandInvoiceAmountCents, "pi_method_card")

	raw := string(reqtest.ExpectBody(t, http.MethodGet, e.PracticeURL(invoicesPath(engagementID)), http.StatusOK, e.SignedIn))

	if strings.Contains(raw, "activePaymentMethod") || strings.Contains(raw, "activePaymentNote") {
		t.Fatalf("a card Payment carries a method or note: %s", raw)
	}
}

// TestGetInvoicesHandler_AContractorNeverReadsHowItWasPaid proves #2241's
// boundary: a contractor never reads a Payment against an Invoice
// (ADR-0008, #2094), so neither the method nor the note reaches her.
func TestGetInvoicesHandler_AContractorNeverReadsHowItWasPaid(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	engagementID := e.paidByHandEngagement(fixture.WithDoula(e.Staff))

	body := string(reqtest.ExpectBody(t, http.MethodGet, e.PracticeURL(invoicesPath(engagementID)), http.StatusOK, e.SignedIn))

	for _, leak := range []string{"bank_transfer", "IB-0950", "activePaymentMethod", "activePaymentNote"} {
		if strings.Contains(body, leak) {
			t.Fatalf("a contractor's Invoice list carries %q: %s", leak, body)
		}
	}
	if !strings.Contains(body, `"status":"paid"`) {
		t.Fatalf("want her paid Invoice in the list: %s", body)
	}
}
