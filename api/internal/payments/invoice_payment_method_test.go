package payments_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

const recordedNote = "Bank transfer from Imani Brooks, reference IB-0950"

// seedPaidByHand is one by-hand Invoice paid by a bank transfer with a
// note, as Record payment leaves it (#271).
func seedPaidByHand(t *testing.T, db *testdb.DB, practiceID, engagementID string) {
	t.Helper()
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	seedManualPayment(t, db, invoiceID, "bank_transfer", recordedNote)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE invoices SET status = 'paid', paid_at = now() WHERE id = $1`, invoiceID); err != nil {
		t.Fatalf("mark invoice paid: %v", err)
	}
}

// TestGetInvoicesHandler_StaffWhoReadMoneyReadHowItWasPaid proves #2241:
// an Owner, an Admin and an employee Doula, who read payment history under
// ADR-0008's money rule, read a by-hand Payment's method and note on the
// Invoice.
func TestGetInvoicesHandler_StaffWhoReadMoneyReadHowItWasPaid(t *testing.T) {
	cases := []struct {
		name       string
		role       string
		isAttached bool
	}{
		{"owner", ownerRole, false},
		{"admin", adminRole, false},
		{"employee doula", doulaRole, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "invoice-method-" + strings.ReplaceAll(tc.name, " ", "-")
			practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{tc.role}, "employee")
			_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Imani Brooks", "imani@example.com")
			if tc.isAttached {
				testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
			}
			seedPaidByHand(t, db, practiceID, engagementID)
			srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
			defer srv.Close()

			resp := getInvoices(t, srv, session, practiceID, engagementID, "")
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			var out payments.ListInvoicesResponse
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("decode: %v", err)
			}
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
	db := testdb.New(t)
	const uid = "invoice-method-card"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	seedStripePayment(t, db, invoiceID, seedByHandInvoiceAmountCents, "pi_method_card")
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.Contains(string(raw), "activePaymentMethod") || strings.Contains(string(raw), "activePaymentNote") {
		t.Fatalf("a card Payment carries a method or note: %s", raw)
	}
}

// TestGetInvoicesHandler_AContractorNeverReadsHowItWasPaid proves #2241's
// boundary: a contractor never reads a Payment against an Invoice
// (ADR-0008, #2094), so neither the method nor the note reaches her.
func TestGetInvoicesHandler_AContractorNeverReadsHowItWasPaid(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-method-contractor"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Imani Brooks", "imani@example.com")
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
	seedPaidByHand(t, db, practiceID, engagementID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)
	for _, leak := range []string{"bank_transfer", "IB-0950", "activePaymentMethod", "activePaymentNote"} {
		if strings.Contains(body, leak) {
			t.Fatalf("a contractor's Invoice list carries %q: %s", leak, body)
		}
	}
	if !strings.Contains(body, `"status":"paid"`) {
		t.Fatalf("want her paid Invoice in the list: %s", body)
	}
}
