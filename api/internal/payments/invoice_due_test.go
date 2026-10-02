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
	// And it is the default terms out from when the Invoice was raised,
	// by the one rule the handler itself spends (#1626) -- exactly, since
	// created_at and the due date are read off the same transaction's
	// now().
	wantDue := payments.DueAfterTerms(view.CreatedAt, payments.DefaultPaymentTermsDays)
	if !stored.Equal(wantDue) {
		t.Fatalf("due date = %v, want %v (%d days of 24 hours after it was raised)", stored, wantDue, payments.DefaultPaymentTermsDays)
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
	wantDue := payments.DueAfterTerms(view.CreatedAt, payments.DefaultPaymentTermsDays)
	if !stored.Equal(wantDue) {
		t.Fatalf("due date = %v, want %v", stored, wantDue)
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
	wantDue := payments.DueAfterTerms(view.CreatedAt, 7)
	if !stored.Equal(wantDue) {
		t.Fatalf("due date = %v, want %v (the Practice's own net-7 terms)", stored, wantDue)
	}

	// Changing the terms afterwards must not move an Invoice already
	// raised: the row carries the terms it was billed under, the same way
	// it carries the rail it was born on.
	readPaymentTerms(t, srv, session, practiceID, http.MethodPut, `{"netDays":60}`)
	if again := invoiceDueAtOf(t, db, view.ID); again.Unix() != stored.Unix() {
		t.Fatalf("due date after a terms change = %v, want the unchanged %v", again, stored)
	}
}

// TestDueAfterTerms_IsADurationAcrossADaylightSavingChange proves the
// rule itself (#1626): an Invoice falls due N x 24 hours after the
// instant it was raised, not N calendar days later in some zone. Each
// raise instant is a fixed value whose terms reach across one of 2026's
// two changes in America/New_York -- the 23-hour day of 2026-03-08 and
// the 25-hour day of 2026-11-01 -- which is where a calendar-day reading
// and a duration reading come apart by an hour. No wall clock and no
// database are read, so the answer is the same on every date and in
// every machine zone, which a handler test (it can only run now) is not.
//
// Each want is a literal instant worked out by hand in UTC, where there
// is no change to cross, rather than the rule computed a second time.
func TestDueAfterTerms_IsADurationAcrossADaylightSavingChange(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load America/New_York: %v", err)
	}

	cases := []struct {
		name     string
		raisedAt time.Time
		netDays  int
		want     time.Time
	}{
		{
			name:     "default terms across the fall-back night of 2026-11-01",
			raisedAt: time.Date(2026, time.October, 2, 9, 35, 20, 0, newYork),
			netDays:  payments.DefaultPaymentTermsDays,
			want:     time.Date(2026, time.November, 1, 13, 35, 20, 0, time.UTC),
		},
		{
			name:     "default terms across the spring-forward night of 2026-03-08",
			raisedAt: time.Date(2026, time.February, 20, 9, 0, 0, 0, newYork),
			netDays:  payments.DefaultPaymentTermsDays,
			want:     time.Date(2026, time.March, 22, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "net-7 terms across the fall-back night of 2026-11-01",
			raisedAt: time.Date(2026, time.October, 28, 23, 30, 0, 0, newYork),
			netDays:  7,
			want:     time.Date(2026, time.November, 5, 3, 30, 0, 0, time.UTC),
		},
		{
			name:     "net-7 terms across the spring-forward night of 2026-03-08",
			raisedAt: time.Date(2026, time.March, 5, 12, 0, 0, 0, newYork),
			netDays:  7,
			want:     time.Date(2026, time.March, 12, 17, 0, 0, 0, time.UTC),
		},
		{
			// The pair Stripe itself returned for days_until_due=30 on the
			// Sandbox on 2026-10-02 (created, due_date), a span that holds
			// the 25-hour day: the count ADR-0038 kept is this one.
			name:     "the instants Stripe gave for days_until_due=30",
			raisedAt: time.Unix(1790948207, 0),
			netDays:  30,
			want:     time.Unix(1793540207, 0),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := payments.DueAfterTerms(tc.raisedAt, tc.netDays)
			if !got.Equal(tc.want) {
				t.Fatalf("due = %v, want %v", got.UTC(), tc.want.UTC())
			}
			if elapsed, want := got.Sub(tc.raisedAt), time.Duration(tc.netDays)*24*time.Hour; elapsed != want {
				t.Fatalf("due is %v after the raise instant, want exactly %v", elapsed, want)
			}
		})
	}
}

// TestDueAfterTerms_IsAWholeSecond pins the resolution: the instant sent
// to Stripe is a Unix timestamp, so the stored one is cut to the same
// second and the two are literally one value.
func TestDueAfterTerms_IsAWholeSecond(t *testing.T) {
	raisedAt := time.Date(2026, time.October, 2, 13, 35, 20, 999_999_000, time.UTC)
	want := time.Date(2026, time.November, 1, 13, 35, 20, 0, time.UTC)
	if got := payments.DueAfterTerms(raisedAt, payments.DefaultPaymentTermsDays); !got.Equal(want) {
		t.Fatalf("due = %v, want %v", got, want)
	}
}
