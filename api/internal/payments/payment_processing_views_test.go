package payments_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
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
	now := time.Now()
	e := newEnv(t, atInstant(now), viaScene(fixture.WithClient(fixture.WithPortalLogin())))
	contractID := seedSignedContract(t, e.DB, e.Engagement.ID)
	onItsWay := seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_on_its_way", invoiceStatusOpen, 15000, now.Add(-time.Hour))
	markProcessing(t, e.DB, onItsWay)
	settled := seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_settled", invoiceStatusPaid, 5000, now.Add(-2*time.Hour))
	markProcessing(t, e.DB, settled)
	want := map[string]bool{onItsWay: true, settled: false}

	check := func(view, id, status string, got bool) {
		t.Helper()
		if got != want[id] {
			t.Errorf("%s: invoice %s paymentProcessing = %v, want %v", view, id, got, want[id])
		}
		if id == onItsWay && status != invoiceStatusOpen {
			t.Errorf("%s: status = %q, want %q -- Stripe keeps it open while it settles", view, status, invoiceStatusOpen)
		}
	}

	for _, it := range e.practiceInvoices().Items {
		check("Practice-wide list", it.ID, it.Status, it.PaymentProcessing)
	}
	for _, it := range e.listInvoices(e.Engagement.ID).Items {
		check("Engagement list", it.ID, it.Status, it.PaymentProcessing)
	}

	contractor := e.Colleague(fixture.Doula(), fixture.Contractor())
	testdb.SeedGrantedAttachment(t, e.DB, e.Engagement.ID, contractor.ID)
	rows := reqtest.Expect[payments.ContractorInvoicesResponse](t, http.MethodGet, e.PracticeURL(invoicesPath(e.Engagement.ID)), http.StatusOK, e.As(contractor))
	for _, it := range rows.Items {
		check("contractor's list", it.ID, it.Status, it.PaymentProcessing)
	}

	portalInvoices := e.URL(clientInvoicesPath(e.Engagement.ID, ""))
	for _, it := range reqtest.Expect[payments.ClientInvoicesResponse](t, http.MethodGet, portalInvoices, http.StatusOK, e.AsClient).Items {
		check("portal list", it.ID, it.Status, it.PaymentProcessing)
	}
	one := reqtest.Expect[payments.ClientInvoiceView](t, http.MethodGet, portalInvoices+"/"+onItsWay, http.StatusOK, e.AsClient)
	check("portal invoice", one.ID, one.Status, one.PaymentProcessing)
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
