package payments_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// forbiddenClientFields are the names that must never cross to a Client
// (#1011), checked against the raw JSON so a struct that grew one fails.
var forbiddenClientFields = []string{
	"stripe_invoice_id", "stripeInvoiceId", "stripe_customer_id", "stripeCustomerId",
	"stripe_payment_reference", "stripePaymentReference", "contract_id", "contractId", "note",
}

func TestClientGetInvoice_PaidByHandCarriesMethodAndRefundNeverTheNote(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-detail"
	me := seedClientMoney(t, db, uid, "Jordan Client", "jordan@example.com")
	invoiceID := seedInvoice(t, db, me.practiceID, me.contractID, "in_cd_paid", invoiceStatusPaid, 5000, time.Now())
	paymentID := seedManualPayment(t, db, invoiceID, methodCheck, "SECRET-RECORDER-NOTE")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, method)
		 VALUES ($1, -1200, now(), 'refund', $2, 'check')`, invoiceID, paymentID); err != nil {
		t.Fatalf("seed refund: %v", err)
	}

	srv := newClientMoneyServer(t, db)
	session := portalSession(t, db, uid)
	status, body := getPortal(t, srv, session, clientInvoicesPath(me.engagementID, "/"+invoiceID))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, body)
	}
	var view payments.ClientInvoiceView
	if err := json.Unmarshal([]byte(body), &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if view.ID != invoiceID || view.Status != invoiceStatusPaid || view.AmountCents != 5000 || view.Reference != "in_cd_paid" || view.BillingMode != string(payments.BillingModeStripe) {
		t.Fatalf("view = %+v", view)
	}
	if view.PaidMethod == nil || *view.PaidMethod != methodCheck {
		t.Fatalf("paidMethod = %v, want check", view.PaidMethod)
	}
	if view.RefundedCents != 1200 {
		t.Fatalf("refundedCents = %d, want 1200", view.RefundedCents)
	}
	for _, f := range forbiddenClientFields {
		if strings.Contains(body, `"`+f+`"`) || strings.Contains(body, "SECRET-RECORDER-NOTE") {
			t.Fatalf("body leaks %q: %s", f, body)
		}
	}

	// The list carries the same row.
	list := readClientInvoices(t, srv, session, me.engagementID, "")
	if len(list.Items) != 1 || list.Items[0].RefundedCents != 1200 || list.Items[0].PaidMethod == nil {
		t.Fatalf("list row differs from detail: %+v", list.Items)
	}
}

func TestClientGetInvoice_ReversedManualPaymentShowsNoMethod(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-reversed"
	me := seedClientMoney(t, db, uid, "Jordan Client", "jordan@example.com")
	invoiceID := seedInvoice(t, db, me.practiceID, me.contractID, "in_cd_rev", invoiceStatusPaid, 5000, time.Now())
	paymentID := seedManualPayment(t, db, invoiceID, "cash", "n")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, reason)
		 VALUES ($1, -5000, now(), 'reversal', $2, 'entered in error')`, invoiceID, paymentID); err != nil {
		t.Fatalf("seed reversal: %v", err)
	}

	srv := newClientMoneyServer(t, db)
	list := readClientInvoices(t, srv, portalSession(t, db, uid), me.engagementID, "")
	if len(list.Items) != 1 || list.Items[0].PaidMethod != nil {
		t.Fatalf("items = %+v, want one row with no paidMethod", list.Items)
	}
}

// TestClientGetInvoice_RefusesWhatIsNotHers covers every way to ask for
// an Invoice that is not the caller's: all read as the same 404.
func TestClientGetInvoice_RefusesWhatIsNotHers(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-refuse"
	me := seedClientMoney(t, db, uid, "Jordan Client", "jordan@example.com")
	other := seedClientMoney(t, db, "client-refuse-other", "Other Client", "other@example.com")
	// A second Engagement of her own: same Client, different Engagement.
	var second string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		me.clientID, me.practiceID).Scan(&second); err != nil {
		t.Fatalf("seed second engagement: %v", err)
	}
	secondContract := seedSignedContract(t, db, second)
	draft := seedInvoice(t, db, me.practiceID, me.contractID, "in_rf_draft", "draft", 100, time.Now())
	theirs := seedInvoice(t, db, other.practiceID, other.contractID, "in_rf_theirs", invoiceStatusOpen, 100, time.Now())
	elsewhere := seedInvoice(t, db, me.practiceID, secondContract, "in_rf_second", invoiceStatusOpen, 100, time.Now())

	srv := newClientMoneyServer(t, db)
	session := portalSession(t, db, uid)
	for name, id := range map[string]string{invoiceStatusDraft: draft, "another Client's": theirs, "another Engagement's": elsewhere, "malformed": "not-a-uuid"} {
		status, body := getPortal(t, srv, session, clientInvoicesPath(me.engagementID, "/"+id))
		if status != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404; body %s", name, status, body)
		}
	}
	// Another Client's Engagement in the path never resolves for her.
	if status, _ := getPortal(t, srv, session, clientInvoicesPath(other.engagementID, "")); status == http.StatusOK {
		t.Errorf("list on another Client's Engagement returned 200")
	}
}

// TestClientInvoiceRoutes_RefuseAStaffSession proves the gate is the
// Client's own portal access: a Staff session, Owner included, reads none.
func TestClientInvoiceRoutes_RefuseAStaffSession(t *testing.T) {
	db := testdb.New(t)
	me := seedClientMoney(t, db, "client-staff-gate", "Jordan Client", "jordan@example.com")
	invoiceID := seedInvoice(t, db, me.practiceID, me.contractID, "in_sg", invoiceStatusOpen, 100, time.Now())
	const staffUID = "owner-not-a-client"
	testdb.SeedStaffAtPractice(t, db, me.practiceID, staffUID, []string{ownerRole}, "employee")

	srv, session := newPracticeInvoiceServer(t, db, staffUID)
	defer srv.Close()
	for _, suffix := range []string{"", "/" + invoiceID} {
		if status, body := getPortal(t, srv, session, clientInvoicesPath(me.engagementID, suffix)); status == http.StatusOK {
			t.Errorf("Staff session read %q: status 200; body %s", suffix, body)
		}
	}
}
