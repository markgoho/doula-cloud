package payments_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// #1965: the day a Payment came in and the day an Invoice falls due are
// days in the Practice's own zone, sent by the BFF as date-only fields,
// and an Invoice is overdue from the start of the day after its due day
// there. These tests prove both through the wire, at fixed instants read
// through clock.Now(ctx).

// dayOct25 is the day every paid-day case here lands on: 8pm in New York
// on Oct 25 for a card Payment, and Oct 25 typed for a by-hand one.
const dayOct25 = "2026-10-25"

// newPracticeInvoiceServerAt is newPracticeInvoiceServer with the
// request clock fixed at now, the way main.go seeds Deps.Now through
// clock.Middleware. The session is minted at the same instant, so the
// fixed clock does not read it as expired.
func newPracticeInvoiceServerAt(t *testing.T, db *testdb.DB, uid string, now time.Time) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	payments.Mount(g, ir, payments.NewFakeClient(), tasknudge.NoOpEnqueuer{}, db.App, testPublishableKey)
	handler := clock.Middleware(func() time.Time { return now })(mux)
	return httptest.NewServer(handler), authntest.SeedSessionAt(t, db.App, uid, now)
}

// TestGetPracticeInvoicesHandler_OverdueFromTheDayAfterDueOn is the
// boundary itself. The Invoice is due at 2026-11-24T00:00Z, which is the
// evening of Nov 23 in New York, so its dueOn is 2026-11-23. Under the
// old instant rule it was overdue from 7pm on Nov 23; a person reads Nov
// 23 as a whole day to pay in, so it is overdue from midnight starting
// Nov 24 in New York, and not a minute before.
func TestGetPracticeInvoicesHandler_OverdueFromTheDayAfterDueOn(t *testing.T) {
	dueAt := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 10, 25, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		now         time.Time
		wantOverdue int
		wantDays    int
	}{
		{"11:59pm on the due day in New York", time.Date(2026, 11, 24, 4, 59, 0, 0, time.UTC), 0, 0},
		{"midnight the day after in New York", time.Date(2026, 11, 24, 5, 0, 0, 0, time.UTC), 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testdb.New(t)
			const uid = "practice-invoices-due-on-boundary"
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
			_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
			contractID := seedDraftContract(t, db, engagementID)
			invoiceID := seedInvoiceDue(t, db, practiceID, contractID, "in_due_on_boundary", invoiceStatusOpen, 15000, createdAt, dueAt)

			srv, session := newPracticeInvoiceServerAt(t, db, uid, tt.now)
			defer srv.Close()

			whole := readPracticeInvoices(t, srv, session, practiceID, "", false)
			if whole.OverdueCount != tt.wantOverdue || whole.OverdueCents != int64(tt.wantOverdue)*15000 {
				t.Fatalf("overdue totals = (%d, %d), want (%d, %d)", whole.OverdueCents, whole.OverdueCount, int64(tt.wantOverdue)*15000, tt.wantOverdue)
			}
			if len(whole.Items) != 1 || whole.Items[0].DueOn != "2026-11-23" {
				t.Fatalf("items = %+v, want one Invoice with dueOn 2026-11-23", whole.Items)
			}
			if whole.Items[0].DaysOverdue != tt.wantDays {
				t.Fatalf("daysOverdue = %d, want %d", whole.Items[0].DaysOverdue, tt.wantDays)
			}

			// The narrowing spends the same boundary as the figure above it.
			narrowed := readPracticeInvoicesNarrowed(t, srv, session, practiceID, "", "overdue=true")
			if len(narrowed.Items) != tt.wantOverdue {
				t.Fatalf("overdue items = %d, want %d", len(narrowed.Items), tt.wantOverdue)
			}
			if tt.wantOverdue == 1 && narrowed.Items[0].ID != invoiceID {
				t.Fatalf("overdue item = %q, want %q", narrowed.Items[0].ID, invoiceID)
			}
		})
	}
}

// TestGetPracticeInvoicesHandler_OverdueBoundaryIsThePracticesZone moves
// the same Invoice to a Practice in Los Angeles. Its dueOn there is
// still Nov 23 (4pm), and at midnight New York time it is only 9pm on
// Nov 23 in Los Angeles: not overdue. The zone that decides is the
// Practice's, not New York's, UTC's or the server's.
func TestGetPracticeInvoicesHandler_OverdueBoundaryIsThePracticesZone(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-due-on-zone"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	testdb.SetPracticeTimezone(t, db, practiceID, "America/Los_Angeles")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	seedInvoiceDue(t, db, practiceID, contractID, "in_due_on_zone", invoiceStatusOpen, 15000,
		time.Date(2026, 10, 25, 15, 0, 0, 0, time.UTC), time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC))

	srv, session := newPracticeInvoiceServerAt(t, db, uid, time.Date(2026, 11, 24, 5, 0, 0, 0, time.UTC))
	defer srv.Close()

	out := readPracticeInvoices(t, srv, session, practiceID, "", false)
	if out.OverdueCount != 0 || len(out.Items) != 1 || out.Items[0].DaysOverdue != 0 || out.Items[0].DueOn != "2026-11-23" {
		t.Fatalf("response = %+v, want one Invoice due 2026-11-23 and not yet overdue in Los Angeles", out)
	}
}

// TestInvoicePaid_8pmEasternIsPaidOnThatDay is #1965's Stripe case: a
// card Payment at 8pm on Oct 25 in New York reaches the webhook as
// 2026-10-26T00:00Z. Both Staff lists send paidOn 2026-10-25, and the
// row still records the instant, which is what the audit trail keeps.
func TestInvoicePaid_8pmEasternIsPaidOnThatDay(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-paid-8pm-eastern"
	const accountID = "acct_paid_8pm"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = $1 WHERE id = $2`, accountID, practiceID,
	); err != nil {
		t.Fatalf("seed connect account id: %v", err)
	}
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_paid_8pm", invoiceStatusOpen, 5000,
		time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC))

	hook := newConnectWebhookServerWith(db, &referenceClient{
		StripeAPIClient: payments.NewStripeAPIClient("sk_test_unused", "https://app.test"),
		reference:       "pi_paid_8pm",
	})
	defer hook.Close()
	paidAt := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC) // 8pm EDT, Oct 25
	resp := postConnectWebhook(t, hook, invoicePaidPayload(t, "evt_paid_8pm", accountID, "in_paid_8pm", paidAt), stripeConnectWebhookSecret)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if _, stored := invoiceStatusAndPaidAt(t, db, invoiceID); stored == nil || !stored.Equal(paidAt) {
		t.Fatalf("stored paid_at = %v, want the instant %v", stored, paidAt)
	}

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()
	book := readPracticeInvoices(t, srv, session, practiceID, "", false)
	if len(book.Items) != 1 || book.Items[0].PaidOn == nil || *book.Items[0].PaidOn != dayOct25 {
		t.Fatalf("Practice list items = %+v, want paidOn 2026-10-25", book.Items)
	}

	listResp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer listResp.Body.Close()
	var section payments.ListInvoicesResponse
	if err := json.NewDecoder(listResp.Body).Decode(&section); err != nil {
		t.Fatalf("decode engagement invoices: %v", err)
	}
	if len(section.Items) != 1 || section.Items[0].PaidOn == nil || *section.Items[0].PaidOn != dayOct25 {
		t.Fatalf("Engagement list items = %+v, want paidOn 2026-10-25", section.Items)
	}
}

// TestClientInvoices_PaidOnIsThePracticesDay proves the Client portal
// prints the same day the Practice's own screens do, for both writers of
// paid_at: a card Payment at 8pm in New York on Oct 25, and a by-hand
// Payment the Practice recorded for Oct 25.
func TestClientInvoices_PaidOnIsThePracticesDay(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-paid-on"
	me := seedClientMoney(t, db, uid, "Jordan Client", "jordan@example.com")
	base := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)

	card := seedInvoice(t, db, me.practiceID, me.contractID, "in_cl_card", invoiceStatusPaid, 2000, base)
	seedPayment(t, db, card, "pi_cl_card", 2000, time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC))
	byHand := seedInvoice(t, db, me.practiceID, me.contractID, "in_cl_by_hand", invoiceStatusPaid, 3000, base.Add(time.Minute))
	seedManualPayment(t, db, byHand, methodCheck, "")
	for id, paidAt := range map[string]string{card: "2026-10-26T00:00:00Z", byHand: "2026-10-25T00:00:00Z"} {
		if _, err := db.Admin.ExecContext(t.Context(), `UPDATE invoices SET paid_at = $1 WHERE id = $2`, paidAt, id); err != nil {
			t.Fatalf("stamp paid_at: %v", err)
		}
	}

	srv := newClientMoneyServer(t, db)
	session := portalSession(t, db, uid)
	list := readClientInvoices(t, srv, session, me.engagementID, "")
	if len(list.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(list.Items))
	}
	for _, it := range list.Items {
		if it.PaidOn == nil || *it.PaidOn != dayOct25 {
			t.Fatalf("item %s paidOn = %v, want 2026-10-25", it.Reference, it.PaidOn)
		}
	}

	status, body := getPortal(t, srv, session, clientInvoicesPath(me.engagementID, "/"+card))
	if status != http.StatusOK {
		t.Fatalf("detail status = %d, want 200; body %s", status, body)
	}
	var view payments.ClientInvoiceView
	if err := json.Unmarshal([]byte(body), &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if view.PaidOn == nil || *view.PaidOn != dayOct25 {
		t.Fatalf("detail paidOn = %v, want 2026-10-25", view.PaidOn)
	}
}

// TestInvoiceReads_RefuseAZoneThatWillNotLoad holds every reader of an
// Invoice's days to ADR-0036: a Practice zone that will not load is a
// 500, never a quiet fall back to UTC's day.
func TestInvoiceReads_RefuseAZoneThatWillNotLoad(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-days-bad-zone"
	me := seedClientMoney(t, db, "invoice-days-bad-zone-client", "Jordan Client", "jordan@example.com")
	testdb.SeedStaffAtPractice(t, db, me.practiceID, uid, []string{ownerRole}, "employee")
	testdb.SeedBillingMode(t, db, me.practiceID, string(payments.BillingModeByHand))
	invoiceID := seedInvoice(t, db, me.practiceID, me.contractID, "in_bad_zone", invoiceStatusOpen, 2000, time.Now())
	testdb.SetPracticeTimezone(t, db, me.practiceID, "Nowhere/Atlantis")

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	book := getPracticeInvoicesNarrowed(t, srv, session, me.practiceID, "", "")
	_ = book.Body.Close()
	section := getInvoices(t, srv, session, me.practiceID, me.engagementID, "")
	_ = section.Body.Close()
	raised := postInvoice(t, srv, session, me.practiceID, me.engagementID)
	_ = raised.Body.Close()
	for name, got := range map[string]int{"practice list": book.StatusCode, "engagement list": section.StatusCode, "raise": raised.StatusCode} {
		if got != http.StatusInternalServerError {
			t.Errorf("%s status = %d, want 500", name, got)
		}
	}

	portal := newClientMoneyServer(t, db)
	clientSession := portalSession(t, db, "invoice-days-bad-zone-client")
	for _, path := range []string{"", "/" + invoiceID} {
		if status, body := getPortal(t, portal, clientSession, clientInvoicesPath(me.engagementID, path)); status != http.StatusInternalServerError {
			t.Errorf("portal %q status = %d, want 500; body %s", path, status, body)
		}
	}
}
