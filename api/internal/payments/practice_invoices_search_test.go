package payments_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
)

// harper is part of a seeded Client's family name, in another case.
const (
	harper     = "harper"
	unpaidView = "unpaid"
)

// searchInvoices reads one page of the Invoice book through the POST that
// carries a search (#2746, #1829), as the Staff member at the screen.
func (e *env) searchInvoices(req payments.PracticeInvoicesListRequest) payments.PracticeInvoicesResponse {
	e.T.Helper()
	return reqtest.Expect[payments.PracticeInvoicesResponse](e.T, http.MethodPost, e.PracticeURL("/invoices/list"), http.StatusOK, e.SignedIn, reqtest.JSON(req))
}

// walkSearch follows nextCursor through every page of one search.
func (e *env) walkSearch(req payments.PracticeInvoicesListRequest) []payments.PracticeInvoiceView {
	e.T.Helper()
	page := e.searchInvoices(req)
	rows := page.Items
	for page.HasMore {
		req.Cursor = *page.NextCursor
		page = e.searchInvoices(req)
		rows = append(rows, page.Items...)
	}
	return rows
}

func rowIDs(rows []payments.PracticeInvoiceView) []string {
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return ids
}

// TestListPracticeInvoicesHandler_SearchFindsAReference is a check "for
// invoice INV-0012": the reference is matched anywhere in it and whatever
// its case, so the number alone, the whole reference and its prefix all
// find it, and nothing that only shares digits in another place does.
func TestListPracticeInvoicesHandler_SearchFindsAReference(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus("draft")
	base := time.Now().Add(-time.Hour)
	twelve := seedInvoice(t, e.DB, e.Practice.ID, contractID, "INV-0012", invoiceStatusOpen, 1000, base)
	oneTwenty := seedInvoice(t, e.DB, e.Practice.ID, contractID, "INV-0120", invoiceStatusOpen, 1000, base.Add(time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "INV-0003", invoiceStatusOpen, 1000, base.Add(2*time.Second))

	cases := map[string][]string{
		"0012":     {twelve},
		"inv-0012": {twelve},
		"INV-01":   {oneTwenty},
		"\t0012\n": {twelve},
		"%":        {},
		"INV_0012": {},
	}
	for search, want := range cases {
		t.Run(search, func(t *testing.T) {
			out := e.searchInvoices(payments.PracticeInvoicesListRequest{Search: search})
			if got := rowIDs(out.Items); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("search %q = %v, want %v", search, got, want)
			}
		})
	}
}

// TestListPracticeInvoicesHandler_SearchFindsAClient is a bank transfer that
// names a Client: part of her name, in any case, or her full name, finds
// every Invoice of hers and no other Client's. The totals stay the whole
// book's.
func TestListPracticeInvoicesHandler_SearchFindsAClient(t *testing.T) {
	e := newEnv(t)
	_, quinnContract := e.contractWithStatus("draft", fixture.NewClient(fixture.ClientName("Quinn Harper")))
	_, adaContract := e.contractWithStatus("draft", fixture.NewClient(fixture.ClientName("Ada Moss")))
	base := time.Now().Add(-time.Hour)
	first := seedInvoice(t, e.DB, e.Practice.ID, quinnContract, "in_quinn_1", invoiceStatusOpen, 1000, base)
	second := seedInvoice(t, e.DB, e.Practice.ID, quinnContract, "in_quinn_2", invoiceStatusPaid, 2000, base.Add(time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, adaContract, "in_ada", invoiceStatusOpen, 4000, base.Add(2*time.Second))

	for _, search := range []string{"harp", "QUINN", "Quinn Harper"} {
		t.Run(search, func(t *testing.T) {
			out := e.searchInvoices(payments.PracticeInvoicesListRequest{Search: search})
			if got := rowIDs(out.Items); strings.Join(got, ",") != second+","+first {
				t.Fatalf("search %q = %v, want [%s %s] (newest first)", search, got, second, first)
			}
			if out.InvoiceCount != 3 || out.OutstandingCents != 5000 {
				t.Fatalf("totals = (%d, %d), want the whole book's (3, 5000)", out.InvoiceCount, out.OutstandingCents)
			}
		})
	}
}

// TestListPracticeInvoicesHandler_SearchNarrowsAView proves a search is
// ANDed with the view: under "unpaid" a matching paid Invoice is left out,
// and under "overdue" a matching Invoice not yet due is too.
func TestListPracticeInvoicesHandler_SearchNarrowsAView(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus("draft", fixture.NewClient(fixture.ClientName("Quinn Harper")))
	now := time.Now()
	late := seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_view_late", invoiceStatusOpen, 1000, now.Add(-40*24*time.Hour), now.Add(-10*24*time.Hour))
	notYet := seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_view_soon", invoiceStatusOpen, 1000, now.Add(-time.Hour), now.Add(10*24*time.Hour))
	seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_view_paid", invoiceStatusPaid, 1000, now, now.Add(-20*24*time.Hour))

	unpaid := e.searchInvoices(payments.PracticeInvoicesListRequest{View: unpaidView, Search: harper})
	if got := rowIDs(unpaid.Items); strings.Join(got, ",") != late+","+notYet {
		t.Fatalf("unpaid = %v, want [%s %s] (payment due first)", got, late, notYet)
	}
	overdue := e.searchInvoices(payments.PracticeInvoicesListRequest{View: "overdue", Search: harper})
	if got := rowIDs(overdue.Items); strings.Join(got, ",") != late {
		t.Fatalf("overdue = %v, want [%s]", got, late)
	}
	all := e.searchInvoices(payments.PracticeInvoicesListRequest{View: "all", Search: harper})
	if len(all.Items) != 3 {
		t.Fatalf("all = %d rows, want 3", len(all.Items))
	}
}

// TestListPracticeInvoicesHandler_SearchPages proves the cursor keeps the
// search under every view: a Client with more matching Invoices than a page
// holds is read in full across the boundary, each once, and another
// Client's Invoices raised between hers never appear.
func TestListPracticeInvoicesHandler_SearchPages(t *testing.T) {
	e := newEnv(t)
	_, quinnContract := e.contractWithStatus("draft", fixture.NewClient(fixture.ClientName("Quinn Harper")))
	_, adaContract := e.contractWithStatus("draft", fixture.NewClient(fixture.ClientName("Ada Moss")))
	now := time.Now()
	const theirs = 32
	for i := range theirs {
		raised := now.Add(-time.Duration(60-i) * 24 * time.Hour)
		seedInvoiceDue(t, e.DB, e.Practice.ID, quinnContract, "in_page_q_"+strconv.Itoa(i), invoiceStatusOpen, 1000, raised, raised.Add(24*time.Hour))
		seedInvoiceDue(t, e.DB, e.Practice.ID, adaContract, "in_page_a_"+strconv.Itoa(i), invoiceStatusOpen, 1000, raised.Add(time.Minute), raised.Add(24*time.Hour+time.Minute))
	}

	for _, view := range []string{"all", unpaidView, "overdue"} {
		t.Run(view, func(t *testing.T) {
			rows := e.walkSearch(payments.PracticeInvoicesListRequest{View: view, Search: "quinn"})
			if len(rows) != theirs {
				t.Fatalf("rows = %d, want %d", len(rows), theirs)
			}
			seen := map[string]bool{}
			for _, row := range rows {
				if row.ClientName != "Quinn Harper" || seen[row.ID] {
					t.Fatalf("row %q for %q: want each of Quinn Harper's once", row.ID, row.ClientName)
				}
				seen[row.ID] = true
			}
		})
	}
}

// TestListPracticeInvoicesHandler_NoSearchIsTheGET proves the POST with no
// search answers what the GET answers for the same view.
func TestListPracticeInvoicesHandler_NoSearchIsTheGET(t *testing.T) {
	e := newEnv(t)
	e.seedOpenInvoices(3)

	posted := e.searchInvoices(payments.PracticeInvoicesListRequest{View: unpaidView})
	got := e.practiceInvoices("unpaid=true")
	if strings.Join(rowIDs(posted.Items), ",") != strings.Join(rowIDs(got.Items), ",") {
		t.Fatalf("POST = %v, GET = %v, want the same rows", rowIDs(posted.Items), rowIDs(got.Items))
	}
}

// TestListPracticeInvoicesHandler_RefusesWhatItCannotRead proves each part
// of the body is checked: an unknown view, a search past 100 characters, a
// cursor that cannot decode or was minted under another view, and a body
// that is not the request's shape are each a 400.
func TestListPracticeInvoicesHandler_RefusesWhatItCannotRead(t *testing.T) {
	e := newEnv(t)
	e.seedOpenInvoices(31)
	whole := e.searchInvoices(payments.PracticeInvoicesListRequest{})

	bodies := map[string]any{
		"an unknown view":            payments.PracticeInvoicesListRequest{View: "late"},
		"a long search":              payments.PracticeInvoicesListRequest{Search: strings.Repeat("a", 101)},
		"a malformed cursor":         payments.PracticeInvoicesListRequest{Cursor: "not-a-cursor"},
		"another view's cursor":      payments.PracticeInvoicesListRequest{View: unpaidView, Cursor: *whole.NextCursor},
		"a body of the wrong shape":  "not an object",
		"a search of the wrong type": map[string]any{"search": 12},
	}
	for name, body := range bodies {
		t.Run(name, func(*testing.T) {
			e.expect(http.MethodPost, "/invoices/list", http.StatusBadRequest, reqtest.JSON(body))
		})
	}
	e.expect(http.MethodPost, "/invoices/list", http.StatusOK, reqtest.JSON(payments.PracticeInvoicesListRequest{Search: strings.Repeat("a", 100)}))
}

// TestGetPracticeInvoicesHandler_RefusesASearchInTheAddress proves a search
// sent in a URL is refused rather than answered with the whole book (#1829).
func TestGetPracticeInvoicesHandler_RefusesASearchInTheAddress(t *testing.T) {
	e := newEnv(t)
	e.expect(http.MethodGet, practiceInvoicesPath("search=Quinn"), http.StatusBadRequest)
}

// TestListPracticeInvoicesHandler_RolesReachTheBook proves the search is
// refused to whoever the list is: a contractor Doula reads no Practice-wide
// book, by name or otherwise.
func TestListPracticeInvoicesHandler_RolesReachTheBook(t *testing.T) {
	for _, tc := range bookReaders {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.who...))
			e.expect(http.MethodPost, "/invoices/list", tc.want, reqtest.JSON(payments.PracticeInvoicesListRequest{Search: "Quinn"}))
		})
	}
}
