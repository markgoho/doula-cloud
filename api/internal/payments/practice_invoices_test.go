package payments_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// practiceInvoicesPath is the Practice-wide Invoice book, narrowed by the
// query parameters given: ?unpaid=true (#427), ?overdue=true (#768) and
// ?cursor=.
func practiceInvoicesPath(query ...string) string {
	path := "/invoices"
	for i, q := range query {
		sep := "&"
		if i == 0 {
			sep = "?"
		}
		path += sep + q
	}
	return path
}

// practiceInvoices reads one page of the Practice's Invoice book as the
// Staff member at the screen.
func (e *env) practiceInvoices(query ...string) payments.PracticeInvoicesResponse {
	e.T.Helper()
	return reqtest.Expect[payments.PracticeInvoicesResponse](e.T, http.MethodGet, e.PracticeURL(practiceInvoicesPath(query...)), http.StatusOK, e.SignedIn)
}

// TestGetPracticeInvoicesHandler_ListsEveryEngagementNewestFirst is gap
// RA-G7 itself: one list answering "who owes us money" without opening
// every Engagement in turn. Each row names the Client and her Engagement,
// so the list is a way in rather than a wall of amounts.
func TestGetPracticeInvoicesHandler_ListsEveryEngagementNewestFirst(t *testing.T) {
	e := newEnv(t)
	engagementA, contractA := e.contractWithStatus("draft", fixture.NewClient(fixture.ClientName("Ada Client")))
	bea := fixture.Engagement(t, e.DB, e.Practice, fixture.NewClient(fixture.ClientName("Bea Client")))
	// #2114: a list of Clients names each in full, so the preferred name
	// is followed by the family name.
	testdb.NameClient(t, e.DB, bea.Client.ID, "Bea", "Okafor")
	contractB := seedDraftContract(t, e.DB, bea.ID)
	base := time.Now().Add(-time.Hour)
	older := seedInvoice(t, e.DB, e.Practice.ID, contractA, "in_prac_a", invoiceStatusOpen, 15000, base)
	newer := seedInvoice(t, e.DB, e.Practice.ID, contractB, "in_prac_b", invoiceStatusOpen, 25000, base.Add(time.Minute))

	out := e.practiceInvoices()

	if len(out.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(out.Items))
	}
	if out.Items[0].ID != newer || out.Items[1].ID != older {
		t.Fatalf("items = [%q %q], want [%q %q] (newest first)", out.Items[0].ID, out.Items[1].ID, newer, older)
	}
	if out.Items[0].ClientName != "Bea Okafor" || out.Items[1].ClientName != "Ada Client" {
		t.Fatalf("client names = [%q %q], want [Bea Okafor Ada Client]", out.Items[0].ClientName, out.Items[1].ClientName)
	}
	if out.Items[0].EngagementID != bea.ID || out.Items[1].EngagementID != engagementA {
		t.Fatalf("engagement ids = [%q %q], want [%q %q]", out.Items[0].EngagementID, out.Items[1].EngagementID, bea.ID, engagementA)
	}
	if out.HasMore {
		t.Fatal("hasMore = true, want false")
	}
}

// TestGetPracticeInvoicesHandler_TotalsCoverTheWholeBook proves the
// outstanding and paid totals are of every Invoice at the Practice, not
// of the page -- and that a draft, void or uncollectible Invoice counts
// as neither (a draft never reached the Client, a void was canceled, an
// uncollectible was written off).
func TestGetPracticeInvoicesHandler_TotalsCoverTheWholeBook(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus("draft")
	base := time.Now().Add(-time.Hour)
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_tot_open1", invoiceStatusOpen, 15000, base)
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_tot_open2", invoiceStatusOpen, 5000, base.Add(time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_tot_paid", invoiceStatusPaid, 30000, base.Add(2*time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_tot_draft", invoiceStatusDraft, 99900, base.Add(3*time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_tot_void", invoiceStatusVoid, 88800, base.Add(4*time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_tot_unc", invoiceStatusUncollectible, 77700, base.Add(5*time.Second))

	out := e.practiceInvoices()

	if out.OutstandingCents != 20000 {
		t.Fatalf("outstandingCents = %d, want 20000", out.OutstandingCents)
	}
	if out.OutstandingCount != 2 {
		t.Fatalf("outstandingCount = %d, want 2", out.OutstandingCount)
	}
	if out.PaidCents != 30000 {
		t.Fatalf("paidCents = %d, want 30000", out.PaidCents)
	}
}

// TestGetPracticeInvoicesHandler_ExcludesOtherPractices proves the app
// layer's own practice_id filter holds on top of the RLS policy
// 00024_invoices.sql already enforces, so a bug in either one alone
// cannot leak another Practice's book.
func TestGetPracticeInvoicesHandler_ExcludesOtherPractices(t *testing.T) {
	e := newEnv(t)
	_, mineContract := e.contractWithStatus("draft")
	mine := seedInvoice(t, e.DB, e.Practice.ID, mineContract, "in_mine", invoiceStatusOpen, 1000, time.Now())

	other := fixture.Practice(t, e.DB)
	theirs := fixture.Engagement(t, e.DB, other)
	seedInvoice(t, e.DB, other.ID, seedDraftContract(t, e.DB, theirs.ID), "in_theirs", invoiceStatusOpen, 500000, time.Now())

	out := e.practiceInvoices()

	if len(out.Items) != 1 || out.Items[0].ID != mine {
		t.Fatalf("items = %+v, want only %q", out.Items, mine)
	}
	if out.OutstandingCents != 1000 {
		t.Fatalf("outstandingCents = %d, want 1000", out.OutstandingCents)
	}
}

// TestGetPracticeInvoicesHandler_EmptyBookIsAnEmptyList proves a Practice
// that has never billed gets a 200 with zero totals, not a 404.
func TestGetPracticeInvoicesHandler_EmptyBookIsAnEmptyList(t *testing.T) {
	e := newEnv(t)

	out := e.practiceInvoices()

	if len(out.Items) != 0 {
		t.Fatalf("items = %d, want 0", len(out.Items))
	}
	if out.OutstandingCents != 0 || out.OutstandingCount != 0 || out.PaidCents != 0 {
		t.Fatalf("totals = (%d, %d, %d), want all zero", out.OutstandingCents, out.OutstandingCount, out.PaidCents)
	}
}

// TestGetPracticeInvoicesHandler_ClientsCanPayIsTheStoredFact proves the
// envelope's clientsCanPay (#2311) is the stored column the Practice
// session reads, on an empty book as on any other.
func TestGetPracticeInvoicesHandler_ClientsCanPayIsTheStoredFact(t *testing.T) {
	e := newEnv(t)

	if out := e.practiceInvoices(); out.ClientsCanPay {
		t.Fatal("clientsCanPay = true before the column is active, want false")
	}

	testdb.SeedClientsCanPay(t, e.DB, e.Practice.ID)

	if out := e.practiceInvoices(); !out.ClientsCanPay {
		t.Fatal("clientsCanPay = false once the column is active, want true")
	}
}

// TestGetPracticeInvoicesHandler_ByHandPaidOnIsTheTypedDay proves a paid
// Invoice carries the day it was paid, so the list can say when rather
// than only naming the status -- and that for a by-hand Payment it is the
// day the recorder typed (#1965), not that day's UTC midnight read in New
// York, which is the day before.
func TestGetPracticeInvoicesHandler_ByHandPaidOnIsTheTypedDay(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus("draft")
	invoiceID := seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_paid_at", invoiceStatusPaid, 4200, time.Now())
	seedManualPayment(t, e.DB, invoiceID, methodCheck, "")
	if _, err := e.DB.Admin.ExecContext(t.Context(), `UPDATE invoices SET paid_at = '2026-10-25T00:00:00Z' WHERE id = $1`, invoiceID); err != nil {
		t.Fatalf("set paid_at: %v", err)
	}

	out := e.practiceInvoices()

	if len(out.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(out.Items))
	}
	if out.Items[0].PaidOn == nil || *out.Items[0].PaidOn != dayOct25 {
		t.Fatalf("paidOn = %v, want 2026-10-25", out.Items[0].PaidOn)
	}
}

// seedOpenInvoices seeds n open Invoices against one Contract at the
// Practice, oldest first, and returns the Contract and the Invoice ids in
// that order.
func (e *env) seedOpenInvoices(n int) (contractID string, invoiceIDs []string) {
	e.T.Helper()
	_, contractID = e.contractWithStatus("draft")
	base := time.Now().Add(-time.Hour)
	for i := range n {
		invoiceIDs = append(invoiceIDs, seedInvoice(e.T, e.DB, e.Practice.ID, contractID, "in_prac_page_"+strconv.Itoa(i),
			invoiceStatusOpen, int64(1000+i), base.Add(time.Duration(i)*time.Second)))
	}
	return contractID, invoiceIDs
}

// TestGetPracticeInvoicesHandler_PaginatesWithCursor proves a book larger
// than one page sets hasMore/nextCursor, that the cursor resumes exactly
// where the first page stopped, and that the totals stay whole-book on
// every page.
func TestGetPracticeInvoicesHandler_PaginatesWithCursor(t *testing.T) {
	e := newEnv(t)
	const total = 31
	_, ids := e.seedOpenInvoices(total)

	first := e.practiceInvoices()
	if !first.HasMore || first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatalf("first page hasMore = %v, nextCursor = %v; want true and a cursor", first.HasMore, first.NextCursor)
	}
	if len(first.Items) != 30 {
		t.Fatalf("first page items = %d, want 30", len(first.Items))
	}
	if first.Items[0].ID != ids[total-1] {
		t.Fatalf("first page items[0] = %q, want %q (newest)", first.Items[0].ID, ids[total-1])
	}

	second := e.practiceInvoices("cursor=" + *first.NextCursor)
	if second.HasMore {
		t.Fatal("second page hasMore = true, want false")
	}
	if len(second.Items) != 1 || second.Items[0].ID != ids[0] {
		t.Fatalf("second page items = %+v, want only %q (oldest)", second.Items, ids[0])
	}
	if second.OutstandingCount != total {
		t.Fatalf("second page outstandingCount = %d, want %d", second.OutstandingCount, total)
	}
}

// TestGetPracticeInvoicesHandler_InvalidCursorReturns400 proves a
// caller-supplied cursor that cannot decode is refused rather than
// silently returning the wrong page.
func TestGetPracticeInvoicesHandler_InvalidCursorReturns400(t *testing.T) {
	e := newEnv(t)

	for name, cursor := range invalidCursors {
		t.Run(name, func(*testing.T) {
			e.expect(http.MethodGet, practiceInvoicesPath("cursor="+cursor), http.StatusBadRequest)
		})
	}
}

// TestGetPracticeInvoicesHandler_UnpaidFilterKeepsOnlyOpen proves
// ?unpaid=true (#427) narrows the list to status 'open' -- the same
// definition the outstanding totals already use -- while a paid, draft,
// void or uncollectible Invoice is left out and the totals stay
// whole-book regardless of the filter.
func TestGetPracticeInvoicesHandler_UnpaidFilterKeepsOnlyOpen(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus("draft")
	base := time.Now().Add(-time.Hour)
	openID := seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_unpaid_open", invoiceStatusOpen, 15000, base)
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_unpaid_paid", invoiceStatusPaid, 30000, base.Add(time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_unpaid_draft", invoiceStatusDraft, 99900, base.Add(2*time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_unpaid_void", invoiceStatusVoid, 88800, base.Add(3*time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_unpaid_unc", invoiceStatusUncollectible, 77700, base.Add(4*time.Second))

	out := e.practiceInvoices("unpaid=true")

	if len(out.Items) != 1 || out.Items[0].ID != openID {
		t.Fatalf("items = %+v, want only %q", out.Items, openID)
	}
	if out.OutstandingCents != 15000 || out.OutstandingCount != 1 {
		t.Fatalf("totals = (%d, %d), want (15000, 1)", out.OutstandingCents, out.OutstandingCount)
	}
	if out.PaidCents != 30000 {
		t.Fatalf("paidCents = %d, want 30000 (whole-book, unaffected by the filter)", out.PaidCents)
	}
}

// TestGetPracticeInvoicesHandler_UnpaidFilterPaginates proves ?unpaid=true
// combines with cursor pagination: a book with more open Invoices than
// one page holds still resumes exactly where the first unpaid page
// stopped, and a paid Invoice interleaved among them never appears on
// either page.
func TestGetPracticeInvoicesHandler_UnpaidFilterPaginates(t *testing.T) {
	e := newEnv(t)
	const totalOpen = 31
	contractID, ids := e.seedOpenInvoices(totalOpen)
	paidID := seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_unpaid_page_paid", invoiceStatusPaid, 5000, time.Now())

	first := e.practiceInvoices("unpaid=true")
	if !first.HasMore || first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatalf("first page hasMore = %v, nextCursor = %v; want true and a cursor", first.HasMore, first.NextCursor)
	}
	if len(first.Items) != 30 {
		t.Fatalf("first page items = %d, want 30", len(first.Items))
	}
	if first.Items[0].ID != ids[totalOpen-1] {
		t.Fatalf("first page items[0] = %q, want %q (newest open)", first.Items[0].ID, ids[totalOpen-1])
	}
	for _, item := range first.Items {
		if item.ID == paidID {
			t.Fatal("first page includes the paid Invoice, want unpaid only")
		}
	}

	second := e.practiceInvoices("unpaid=true", "cursor="+*first.NextCursor)
	if second.HasMore {
		t.Fatal("second page hasMore = true, want false")
	}
	if len(second.Items) != 1 || second.Items[0].ID != ids[0] {
		t.Fatalf("second page items = %+v, want only %q (oldest open)", second.Items, ids[0])
	}
}

// TestGetPracticeInvoicesHandler_RolesReachTheBook proves ADR-0008's money
// row as amended by #282: an employed Doula reaches the Practice-wide
// Invoice book the same as an Owner or Admin. A contractor is refused by
// the handler's own staffauth.RequireNotAmbientContractor check, not just
// by the role the GatedRouter mount declares, which
// api/gate_guardrail_test.go already covers but cannot behaviorally test.
func TestGetPracticeInvoicesHandler_RolesReachTheBook(t *testing.T) {
	cases := []struct {
		name string
		who  []fixture.StaffOption
		want int
	}{
		{"an Owner", []fixture.StaffOption{fixture.Owner()}, http.StatusOK},
		{"an employed Doula", []fixture.StaffOption{fixture.Doula()}, http.StatusOK},
		{"a contractor Doula", []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.who...))
			e.expect(http.MethodGet, practiceInvoicesPath(), tc.want)
		})
	}
}
