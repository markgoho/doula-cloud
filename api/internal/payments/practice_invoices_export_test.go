package payments_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/csvexport/exporttest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

var exportColumns = []string{
	"Client", "Invoice", "Care", "Billed", "Amount", "Currency", "State",
	"Days overdue", "Sent", "Payment due", "Paid", "Refunded",
}

func (e *env) exportRequest(req payments.PracticeInvoicesExportRequest) *http.Response {
	e.T.Helper()
	return reqtest.Do(e.T, http.MethodPost, e.PracticeURL("/invoices/export"), e.SignedIn, reqtest.JSON(req))
}

// exportRows is the CSV the export answers with, header first.
func (e *env) exportRows(req payments.PracticeInvoicesExportRequest) [][]string {
	e.T.Helper()
	return exporttest.Read(e.T, func() *http.Response { return e.exportRequest(req) })
}

func (e *env) exportActivity() []string {
	e.T.Helper()
	return exporttest.Diffs(e.T, e.DB, e.Practice.ID, "invoices_exported")
}

// TestExportPracticeInvoices_WritesTheTwelveColumnsAndNothingElse proves
// the header is the decided twelve, in order, and that no row is wider.
func TestExportPracticeInvoices_WritesTheTwelveColumnsAndNothingElse(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus(invoiceStatusDraft)
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_cols", invoiceStatusOpen, 1000, time.Now().Add(-time.Hour))

	rows := e.exportRows(payments.PracticeInvoicesExportRequest{})
	if !slices.Equal(rows[0], exportColumns) {
		t.Fatalf("header = %v, want %v", rows[0], exportColumns)
	}
	for _, row := range rows {
		if len(row) != len(exportColumns) {
			t.Fatalf("row = %v, want %d cells", row, len(exportColumns))
		}
	}
}

// TestExportPracticeInvoices_EveryMatchingRowNotJustThePage proves the
// export is not capped at the 30 the list loads.
func TestExportPracticeInvoices_EveryMatchingRowNotJustThePage(t *testing.T) {
	e := newEnv(t)
	e.seedOpenInvoices(35)

	rows := e.exportRows(payments.PracticeInvoicesExportRequest{})
	if len(rows) != 36 {
		t.Fatalf("lines = %d, want a header and 35 Invoices", len(rows))
	}
}

// TestExportPracticeInvoices_CellsAreTheListsOwnWords proves each cell: the
// Client in full, the care, how it was billed, decimal money, the state in
// the list's word, and days in the Practice's zone.
func TestExportPracticeInvoices_CellsAreTheListsOwnWords(t *testing.T) {
	now := time.Now()
	e := newEnv(t, atInstant(now))
	sent := time.Date(2026, 10, 26, 3, 30, 0, 0, time.UTC)

	engagementID, contractID := e.contractWithStatus(invoiceStatusDraft, fixture.Kind("postpartum"), fixture.NewClient(fixture.ClientName("Amara Okafor")))
	_ = engagementID
	late := seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_late", invoiceStatusOpen, 150050, sent, dueOnPracticeDay(t, now, -5))
	_ = late

	_, byHand := e.byHandInvoice()
	if _, err := e.DB.Admin.ExecContext(t.Context(), `UPDATE invoices SET created_at = $2, reference = 'INV-0009' WHERE id = $1`, byHand, sent.Add(-time.Hour)); err != nil {
		t.Fatalf("shape by-hand invoice: %v", err)
	}
	paymentID := e.record(byHand)
	e.refund(byHand, paymentID, checkRefund(5025))

	rows := e.exportRows(payments.PracticeInvoicesExportRequest{Order: "sent-oldest"})
	if len(rows) != 3 {
		t.Fatalf("rows = %v, want a header and two Invoices", rows)
	}

	handRow, lateRow := rows[1], rows[2]
	if handRow[1] != "INV-0009" || handRow[3] != "By hand" || handRow[2] != "Birth" || handRow[4] != "150.00" || handRow[5] != "USD" ||
		handRow[6] != invoiceStatusPaid || handRow[7] != "0" || handRow[8] != dayOct25 || handRow[10] != todayInSeededZone(t) || handRow[11] != "50.25" {
		t.Fatalf("by-hand row = %v", handRow)
	}
	wantDue := practiceDayOf(t, dueOnPracticeDay(t, now, -5))
	if lateRow[0] != "Amara Okafor" || lateRow[1] != "in_late" || lateRow[2] != "Postpartum" || lateRow[3] != "Stripe" ||
		lateRow[4] != "1500.50" || lateRow[6] != viewOverdue || lateRow[7] != "5" || lateRow[8] != dayOct25 || lateRow[9] != wantDue || lateRow[10] != "" || lateRow[11] != "0.00" {
		t.Fatalf("overdue row = %v, want due %s", lateRow, wantDue)
	}
}

func practiceDayOf(t *testing.T, at time.Time) string {
	t.Helper()
	zone, err := time.LoadLocation(testdb.SeededPracticeZoneName)
	if err != nil {
		t.Fatalf("load zone: %v", err)
	}
	return at.In(zone).Format("2006-01-02")
}

// TestExportPracticeInvoices_StateWords proves every state the list names
// reads as its own word in the State column.
func TestExportPracticeInvoices_StateWords(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus(invoiceStatusDraft)
	base := time.Now().Add(-time.Hour)
	owed := seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_s_owed", invoiceStatusOpen, 100, base)
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_s_paid", invoiceStatusPaid, 100, base.Add(time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_s_void", invoiceStatusVoid, 100, base.Add(2*time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_s_off", invoiceStatusUncollectible, 100, base.Add(3*time.Second))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_s_draft", invoiceStatusDraft, 100, base.Add(4*time.Second))
	onItsWay := seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_s_way", invoiceStatusOpen, 100, base.Add(5*time.Second))
	markProcessing(t, e.DB, onItsWay)
	_ = owed

	got := map[string]string{}
	for _, row := range e.exportRows(payments.PracticeInvoicesExportRequest{})[1:] {
		got[row[1]] = row[6]
	}
	want := map[string]string{
		"in_s_owed": "owed", "in_s_paid": invoiceStatusPaid, "in_s_void": invoiceStatusVoid,
		"in_s_off": "written off", "in_s_draft": invoiceStatusDraft, "in_s_way": "payment on its way",
	}
	if len(got) != len(want) {
		t.Fatalf("states = %v, want %v", got, want)
	}
	for ref, state := range want {
		if got[ref] != state {
			t.Fatalf("%s state = %q, want %q", ref, got[ref], state)
		}
	}
}

// TestExportPracticeInvoices_GuardsACellASpreadsheetWouldRun proves a
// Client's typed name goes through the one CSV writer.
func TestExportPracticeInvoices_GuardsACellASpreadsheetWouldRun(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus(invoiceStatusDraft, fixture.NewClient(fixture.ClientName("=HYPERLINK(1) Quinn")))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "=cmd", invoiceStatusOpen, 1000, time.Now().Add(-time.Hour))

	rows := e.exportRows(payments.PracticeInvoicesExportRequest{})
	if !strings.HasPrefix(rows[1][0], "'=") || rows[1][1] != "'=cmd" {
		t.Fatalf("row = %v, want the name and the reference guarded with a leading apostrophe", rows[1])
	}
}

// TestExportPracticeInvoices_HoldsTheViewSearchPeriodAndOrder proves the
// export is the list's own narrowing: the same rows in the same order.
func TestExportPracticeInvoices_HoldsTheViewSearchPeriodAndOrder(t *testing.T) {
	e := newEnv(t)
	_, contractID := e.contractWithStatus(invoiceStatusDraft)
	in := time.Date(2026, 10, 25, 16, 0, 0, 0, time.UTC)
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "INV-A1", invoiceStatusOpen, 300, in)
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "INV-A2", invoiceStatusOpen, 100, in.Add(time.Hour))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "INV-A3", invoiceStatusPaid, 200, in.Add(2*time.Hour))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "INV-B1", invoiceStatusOpen, 400, in.Add(3*time.Hour))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "INV-A4", invoiceStatusOpen, 500, in.Add(48*time.Hour))

	req := payments.PracticeInvoicesExportRequest{View: unpaidView, Search: "inv-a", SentFrom: dayOct25, SentTo: dayOct25, Order: orderLarge}
	rows := e.exportRows(req)
	var refs []string
	for _, row := range rows[1:] {
		refs = append(refs, row[1])
	}
	if strings.Join(refs, ",") != "INV-A1,INV-A2" {
		t.Fatalf("references = %v, want INV-A1,INV-A2 -- unpaid, matching inv-a, sent on Oct 25, largest first", refs)
	}

	listed := e.searchInvoices(payments.PracticeInvoicesListRequest{View: req.View, Search: req.Search, SentFrom: req.SentFrom, SentTo: req.SentTo, Order: req.Order})
	if strings.Join(rowRefs(listed.Items), ",") != strings.Join(refs, ",") {
		t.Fatalf("list = %v, export = %v, want the same rows", rowRefs(listed.Items), refs)
	}
}

func rowRefs(items []payments.PracticeInvoiceView) []string {
	refs := make([]string, len(items))
	for i, it := range items {
		refs[i] = it.Reference
	}
	return refs
}

// TestExportPracticeInvoices_AnswersAFileNamedForThePracticesDay proves the
// download headers: the type, and a filename carrying the day in the
// Practice's zone, which is the 14th at 9pm in New York though it is the
// 15th in UTC.
func TestExportPracticeInvoices_AnswersAFileNamedForThePracticesDay(t *testing.T) {
	e := newEnv(t, atInstant(time.Date(2026, 11, 15, 2, 0, 0, 0, time.UTC)))

	resp := e.exportRequest(payments.PracticeInvoicesExportRequest{})
	defer func() { _ = resp.Body.Close() }()

	exporttest.RequireFile(t, resp, "invoices-2026-11-14.csv", "Invoice-Count", "0")
}

// TestExportPracticeInvoices_RecordsWhoWhenWhatAndNeverTheSearch proves the
// one Activity row a download writes: the Practice as its subject, the
// Staff member as its actor, the slice of the book and its size, and
// whether a search was applied -- never the search's text.
func TestExportPracticeInvoices_RecordsWhoWhenWhatAndNeverTheSearch(t *testing.T) {
	now := time.Date(2026, 11, 14, 17, 0, 0, 0, time.UTC)
	e := newEnv(t, atInstant(now))
	_, contractID := e.contractWithStatus(invoiceStatusDraft)
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "INV-Quinn1", invoiceStatusOpen, 100, time.Date(2026, 10, 25, 16, 0, 0, 0, time.UTC))
	seedInvoice(t, e.DB, e.Practice.ID, contractID, "INV-Quinn2", invoiceStatusOpen, 100, time.Date(2026, 10, 25, 17, 0, 0, 0, time.UTC))

	e.exportRows(payments.PracticeInvoicesExportRequest{View: unpaidView, Search: "quinn", SentFrom: dayOct25, SentTo: dayOct25, Order: orderLarge})

	want := map[string]any{"view": unpaidView, "sentFrom": dayOct25, "sentTo": dayOct25, "order": orderLarge, "rows": 2.0, "searched": true}
	exporttest.RequireRecorded(t, e.DB, e.Practice.ID, e.Staff.ID, "invoices_exported", now, want, "quinn")
}

// TestExportPracticeInvoices_NoSearchNoPeriodRecordsTheBareView proves a
// download with no search says so, and carries no days.
func TestExportPracticeInvoices_NoSearchNoPeriodRecordsTheBareView(t *testing.T) {
	e := newEnv(t)
	e.exportRows(payments.PracticeInvoicesExportRequest{})

	diffs := e.exportActivity()
	if len(diffs) != 1 || diffs[0] != `{"rows": 0, "view": "all", "searched": false}` {
		t.Fatalf("activity = %v, want one row of the bare view", diffs)
	}
}

// TestExportPracticeInvoices_ARefusedRequestWritesNoRow proves a request
// the list would refuse is a 400 and is not recorded as a download.
func TestExportPracticeInvoices_ARefusedRequestWritesNoRow(t *testing.T) {
	cases := map[string]payments.PracticeInvoicesExportRequest{
		"a view the list does not offer": {View: invoiceStatusPaid},
		"a search over 100 characters":   {Search: strings.Repeat("a", 101)},
		"an order the list does not":     {Order: "random"},
		"a period with only a From":      {SentFrom: dayOct25},
		"a To before the From":           {SentFrom: "2026-10-26", SentTo: dayOct25},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.postRefused("/invoices/export", http.StatusBadRequest, reqtest.JSON(req))
			if got := e.exportActivity(); len(got) != 0 {
				t.Fatalf("activity = %v, want none", got)
			}
		})
	}
	t.Run(caseBodyNotJSON, func(t *testing.T) {
		e := newEnv(t)
		e.postRefused("/invoices/export", http.StatusBadRequest, reqtest.RawJSON([]byte("{")))
	})
}

// TestExportPracticeInvoices_OnlyAnOwnerOrAnAdmin proves who may download:
// an Owner and an Admin, and neither a Doula nor a contractor Doula, who
// get a 403 and no row.
func TestExportPracticeInvoices_OnlyAnOwnerOrAnAdmin(t *testing.T) {
	exporttest.RequireOwnerAndAdminOnly(t, func(t *testing.T, role fixture.RoleCase) (int, int) {
		e := newEnv(t, signedInAs(role.Opts...))
		resp := e.exportRequest(payments.PracticeInvoicesExportRequest{})
		_ = resp.Body.Close()
		return resp.StatusCode, len(e.exportActivity())
	})
}

// TestExportPracticeInvoices_ContractorAdminIsRefusedAsTheListRefuses proves
// a contractor who also holds the Admin role is refused, as the list
// refuses her.
func TestExportPracticeInvoices_ContractorAdminIsRefusedAsTheListRefuses(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin(), fixture.Doula(), fixture.Contractor()))
	list := reqtest.Do(t, http.MethodGet, e.PracticeURL(practiceInvoicesPath()), e.SignedIn)
	_ = list.Body.Close()
	export := e.exportRequest(payments.PracticeInvoicesExportRequest{})
	_ = export.Body.Close()
	if export.StatusCode != list.StatusCode {
		t.Fatalf("export status = %d, list status = %d, want the same refusal", export.StatusCode, list.StatusCode)
	}
}

// TestExportPracticeInvoices_AnotherPracticesInvoicesStayOut proves the
// file holds this Practice's book alone.
func TestExportPracticeInvoices_AnotherPracticesInvoicesStayOut(t *testing.T) {
	e := newEnv(t)
	other := newEnv(t)
	_, contractID := other.contractWithStatus(invoiceStatusDraft)
	seedInvoice(t, other.DB, other.Practice.ID, contractID, "in_elsewhere", invoiceStatusOpen, 100, time.Now().Add(-time.Hour))

	if rows := e.exportRows(payments.PracticeInvoicesExportRequest{}); len(rows) != 1 {
		t.Fatalf("rows = %v, want the header alone", rows)
	}
}

// TestExportPracticeInvoices_RateLimitedAsAHeavyEndpoint proves the 31st
// download in an hour is a 429 with the standard headers, and that it is
// not recorded.
func TestExportPracticeInvoices_RateLimitedAsAHeavyEndpoint(t *testing.T) {
	e := newEnv(t)
	exporttest.RequireRateLimited(t, func() *http.Response {
		return e.exportRequest(payments.PracticeInvoicesExportRequest{})
	}, func() int { return len(e.exportActivity()) })
}
