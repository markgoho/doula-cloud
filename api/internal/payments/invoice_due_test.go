package payments_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// raiseInvoice raises an Invoice and returns the view the caller was
// told about, so the due date in the response can be compared against
// the one the row and Stripe both hold.
func raiseInvoice(t *testing.T, srv *httptest.Server, session, practiceID, engagementID string) payments.InvoiceView {
	t.Helper()
	resp := postInvoice(t, srv, session, practiceID, engagementID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out payments.InvoiceView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode invoice: %v", err)
	}
	return out
}

// TestPostInvoiceHandler_StripeRailSendsTheStoredDueDate is #768's first
// acceptance criterion on the Stripe rail: one due date, computed once,
// sent to Stripe as an instant and stored unchanged. Before this, Stripe
// got days_until_due=30 and derived its own date, which never came home
// to the row -- two dates, either of which could be a day off the other.
func TestPostInvoiceHandler_StripeRailSendsTheStoredDueDate(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-due-stripe"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	seedConnectAccount(t, db, practiceID, "acct_due_stripe")
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	view := raiseInvoice(t, srv, session, practiceID, engagementID)

	stored := invoiceDueAtOf(t, db, view.ID)
	if len(client.CreateInvoiceCalls) != 1 {
		t.Fatalf("CreateInvoice calls = %d, want 1", len(client.CreateInvoiceCalls))
	}
	// Stripe is told the same instant the row holds, to the second --
	// StripeAPIClient sends it as a Unix timestamp, which is the
	// resolution Stripe's due_date carries.
	sent := client.CreateInvoiceCalls[0].DueAt
	if sent.Unix() != stored.Unix() {
		t.Fatalf("due date sent to Stripe = %v, stored = %v; want the same instant", sent, stored)
	}
	if view.DueAt.Unix() != stored.Unix() {
		t.Fatalf("returned dueAt = %v, stored = %v; want the same instant", view.DueAt, stored)
	}
	// And it is the default terms out from when the Invoice was raised.
	wantDue := view.CreatedAt.AddDate(0, 0, payments.DefaultPaymentTermsDays)
	if stored.Sub(wantDue).Abs() > time.Minute {
		t.Fatalf("due date = %v, want about %v (%d days after it was raised)", stored, wantDue, payments.DefaultPaymentTermsDays)
	}
}

// TestPostInvoiceHandler_ByHandRailStoresTheDueDate covers the rail where
// the column is the only place the fact exists at all: a by-hand Invoice
// never reaches Stripe, so before #768 it had no due date anywhere, in
// any system.
func TestPostInvoiceHandler_ByHandRailStoresTheDueDate(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-due-by-hand"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeByHand))
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	view := raiseInvoice(t, srv, session, practiceID, engagementID)

	stored := invoiceDueAtOf(t, db, view.ID)
	if view.DueAt.Unix() != stored.Unix() {
		t.Fatalf("returned dueAt = %v, stored = %v; want the same instant", view.DueAt, stored)
	}
	if len(client.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0 on the by-hand rail", len(client.CreateInvoiceCalls))
	}
	wantDue := view.CreatedAt.AddDate(0, 0, payments.DefaultPaymentTermsDays)
	if stored.Sub(wantDue).Abs() > time.Minute {
		t.Fatalf("due date = %v, want about %v", stored, wantDue)
	}
}

// TestPostInvoiceHandler_UsesThePracticesOwnTerms proves the Practice's
// setting is what an Invoice is actually raised against, rather than the
// 30-day default being applied everywhere regardless.
func TestPostInvoiceHandler_UsesThePracticesOwnTerms(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-due-own-terms"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeByHand))

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	readPaymentTerms(t, srv, session, practiceID, http.MethodPut, `{"netDays":7}`)

	view := raiseInvoice(t, srv, session, practiceID, engagementID)
	stored := invoiceDueAtOf(t, db, view.ID)
	wantDue := view.CreatedAt.AddDate(0, 0, 7)
	if stored.Sub(wantDue).Abs() > time.Minute {
		t.Fatalf("due date = %v, want about %v (the Practice's own net-7 terms)", stored, wantDue)
	}

	// Changing the terms afterwards must not move an Invoice already
	// raised: the row carries the terms it was billed under, the same way
	// it carries the rail it was born on.
	readPaymentTerms(t, srv, session, practiceID, http.MethodPut, `{"netDays":60}`)
	if again := invoiceDueAtOf(t, db, view.ID); again.Unix() != stored.Unix() {
		t.Fatalf("due date after a terms change = %v, want the unchanged %v", again, stored)
	}
}
