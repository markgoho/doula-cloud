package payments_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// markProcessing stands in for payment_intent.processing having landed on
// invoiceID (#2225).
func markProcessing(t *testing.T, db *testdb.DB, invoiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE invoices SET payment_processing_since = now() WHERE id = $1`, invoiceID,
	); err != nil {
		t.Fatalf("mark payment processing: %v", err)
	}
}

// TestAPaymentOnItsWayReadsInEveryView is #2225's second box: while a
// payment settles, every reader of the Invoice -- the Practice-wide list,
// the Engagement's list as an Owner and as a contractor Doula (a status,
// so hers to see under ADR-0008's #2094 amendment), and the Client's own
// list and page -- is told a payment is on its way, and the status stays
// open. A value left on an Invoice that has since been paid reads as
// nothing on its way: only an open Invoice can be waiting on one.
func TestAPaymentOnItsWayReadsInEveryView(t *testing.T) {
	db := testdb.New(t)
	const clientUID = "on-its-way-client"
	me := seedClientMoney(t, db, clientUID, "Imani Client", "imani@example.com")
	now := time.Now()
	onItsWay := seedInvoice(t, db, me.practiceID, me.contractID, "in_on_its_way", invoiceStatusOpen, 15000, now.Add(-time.Hour))
	markProcessing(t, db, onItsWay)
	settled := seedInvoice(t, db, me.practiceID, me.contractID, "in_settled", invoiceStatusPaid, 5000, now.Add(-2*time.Hour))
	markProcessing(t, db, settled)
	want := map[string]bool{onItsWay: true, settled: false}

	const ownerUID = "on-its-way-owner"
	testdb.SeedStaffAtPractice(t, db, me.practiceID, ownerUID, []string{ownerRole}, "employee")
	srv, ownerSession := newPracticeInvoiceServerAt(t, db, ownerUID, now)
	defer srv.Close()

	check := func(view, id, status string, got bool) {
		t.Helper()
		if got != want[id] {
			t.Errorf("%s: invoice %s paymentProcessing = %v, want %v", view, id, got, want[id])
		}
		if id == onItsWay && status != invoiceStatusOpen {
			t.Errorf("%s: status = %q, want %q -- Stripe keeps it open while it settles", view, status, invoiceStatusOpen)
		}
	}

	for _, it := range readPracticeInvoices(t, srv, ownerSession, me.practiceID, "", false).Items {
		check("Practice-wide list", it.ID, it.Status, it.PaymentProcessing)
	}

	var section payments.ListInvoicesResponse
	sectionResp := getInvoices(t, srv, ownerSession, me.practiceID, me.engagementID, "")
	defer sectionResp.Body.Close()
	decodeInvoices(t, sectionResp, &section)
	for _, it := range section.Items {
		check("Engagement list", it.ID, it.Status, it.PaymentProcessing)
	}

	const contractorUID = "on-its-way-contractor"
	contractorID := testdb.SeedStaffAtPractice(t, db, me.practiceID, contractorUID, []string{doulaRole}, "contractor")
	testdb.SeedGrantedAttachment(t, db, me.engagementID, contractorID)
	var rows payments.ContractorInvoicesResponse
	rowsResp := getInvoices(t, srv, authntest.SeedSessionAt(t, db.App, contractorUID, now), me.practiceID, me.engagementID, "")
	defer rowsResp.Body.Close()
	decodeInvoices(t, rowsResp, &rows)
	for _, it := range rows.Items {
		check("contractor's list", it.ID, it.Status, it.PaymentProcessing)
	}

	portal := newClientMoneyServer(t, db)
	session := portalSession(t, db, clientUID)
	for _, it := range readClientInvoices(t, portal, session, me.engagementID, "").Items {
		check("portal list", it.ID, it.Status, it.PaymentProcessing)
	}
	status, body := getPortal(t, portal, session, clientInvoicesPath(me.engagementID, "/"+onItsWay))
	if status != http.StatusOK {
		t.Fatalf("portal invoice status = %d; body %s", status, body)
	}
	var one payments.ClientInvoiceView
	if err := json.Unmarshal([]byte(body), &one); err != nil {
		t.Fatalf("decode portal invoice: %v", err)
	}
	check("portal invoice", one.ID, one.Status, one.PaymentProcessing)
}

// decodeInvoices reads a 200 into into. The caller closes the body.
func decodeInvoices(t *testing.T, resp *http.Response, into any) {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// TestClientGetPayment_RefusesWhileAPaymentIsOnItsWay is the boundary
// behind the Invoice page's pending notice (#2225): while her payment
// settles there is nothing more for her to pay, and Stripe would refuse
// a second confirm of a processing PaymentIntent anyway, so no secret is
// handed out and Stripe is never asked for one.
func TestClientGetPayment_RefusesWhileAPaymentIsOnItsWay(t *testing.T) {
	w := newPayWorld(t, "pay-on-its-way")
	invoiceID := seedInvoice(t, w.db, w.me.practiceID, w.me.contractID, "in_pay_on_its_way", invoiceStatusOpen, 100, time.Now())
	markProcessing(t, w.db, invoiceID)

	status, body := w.get(t, invoiceID)
	if status != http.StatusConflict || !strings.Contains(body, "Your payment is on its way.") {
		t.Fatalf("status %d body %s, want 409 saying the payment is on its way", status, body)
	}
	if len(w.stripe.ClientSecretCalls) != 0 {
		t.Fatalf("an Invoice with a payment on its way reached Stripe: %+v", w.stripe.ClientSecretCalls)
	}
}
