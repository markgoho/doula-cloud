package contracts_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/csvexport/exporttest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

const (
	dayOct25   = "2026-10-25"
	exportPath = "/contracts/export"
)

var exportColumns = []string{
	"Client", "Care", "Status", "Status day", "Sent", "Days waiting", "Who", "Doula", "Fee", "Void requested",
}

func (e *env) exportRequest(req contracts.ListRequest) *http.Response {
	e.T.Helper()
	return reqtest.Do(e.T, http.MethodPost, e.PracticeURL(exportPath), e.SignedIn, reqtest.JSON(req))
}

// jscpd:ignore-start -- each download's test file reads its own file and its own Activity action through exporttest
func (e *env) exportRows(req contracts.ListRequest) [][]string {
	e.T.Helper()
	return exporttest.Read(e.T, func() *http.Response { return e.exportRequest(req) })
}

func (e *env) exportActivity() []string {
	e.T.Helper()
	return exporttest.Diffs(e.T, e.DB, e.Practice.ID, "contracts_exported")
}

// jscpd:ignore-end

func clients(rows [][]string) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows[1:] {
		names = append(names, row[0])
	}
	return names
}

// TestExportContracts_WritesTheDecidedColumnsAndNothingElse proves the
// header, in order, and that no row is wider.
func TestExportContracts_WritesTheDecidedColumnsAndNothingElse(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedEveryStatus()

	rows := e.exportRows(contracts.ListRequest{View: viewAll})
	if !slices.Equal(rows[0], exportColumns) || len(rows) != 5 {
		t.Fatalf("rows = %v, want the header and four Contracts", rows)
	}
	for _, row := range rows {
		if len(row) != len(exportColumns) {
			t.Fatalf("row = %v, want %d cells", row, len(exportColumns))
		}
	}
}

// TestExportContracts_EveryMatchingRowNotJustThePage proves the export is not
// capped at the 30 the list loads.
func TestExportContracts_EveryMatchingRowNotJustThePage(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	for i := range listPageSize + 5 {
		e.seedListed(statusSigned, listBase.Add(time.Duration(i)*time.Minute))
	}

	if rows := e.exportRows(contracts.ListRequest{View: statusSigned}); len(rows) != listPageSize+6 {
		t.Fatalf("lines = %d, want a header and %d Contracts", len(rows), listPageSize+5)
	}
}

// TestExportContracts_CellsAreTheListsOwnWords proves each cell: the Client,
// the care, the status and its day, the day sent, the wait, who took the
// step, the Doula, and the fee as decimal money.
func TestExportContracts_CellsAreTheListsOwnWords(t *testing.T) {
	now := time.Date(2026, 11, 14, 17, 0, 0, 0, time.UTC)
	e := newEnv(t, signedInAs(fixture.Owner()), atInstant(now))
	sentAt := time.Date(2026, 11, 4, 16, 0, 0, 0, time.UTC)
	e.seedListed(statusSent, sentAt, fixture.Kind("postpartum"), fixture.NewClient(fixture.ClientName("Amara Okafor")))
	if _, err := e.DB.Admin.ExecContext(t.Context(), `UPDATE contracts SET sent_by = $1`, e.Staff.ID); err != nil {
		t.Fatalf("set sender: %v", err)
	}

	rows := e.exportRows(contracts.ListRequest{View: statusSent})
	if len(rows) != 2 {
		t.Fatalf("rows = %v, want a header and one Contract", rows)
	}
	row := rows[1]
	if row[0] != "Amara Okafor" || row[1] != "Postpartum" || row[2] != "Sent" || row[3] != "2026-11-04" || row[4] != "2026-11-04" ||
		row[5] != "10" || row[6] != "Sent by "+e.Staff.Name || row[8] != "1500.00" || row[9] != "" {
		t.Fatalf("row = %v", row)
	}
}

// TestExportContracts_AVoidRequestedContractSaysSo proves the last column.
func TestExportContracts_AVoidRequestedContractSaysSo(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	engagement, contractID := e.seedListed(statusSigned, listBase)
	if _, err := e.DB.Admin.ExecContext(t.Context(),
		`INSERT INTO contract_void_requests (contract_id, engagement_id, practice_id, requested_by, reason) VALUES ($1, $2, $3, $4, 'Wrong fee')`,
		contractID, engagement.ID, e.Practice.ID, e.Staff.ID); err != nil {
		t.Fatalf("seed void request: %v", err)
	}

	rows := e.exportRows(contracts.ListRequest{View: statusSigned})
	if rows[1][9] != "yes" {
		t.Fatalf("row = %v, want void requested", rows[1])
	}
}

// TestExportContracts_GuardsACellASpreadsheetWouldRun proves a Client's typed
// name goes through the one CSV writer.
func TestExportContracts_GuardsACellASpreadsheetWouldRun(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedListed(statusSent, listBase, fixture.NewClient(fixture.ClientName("=HYPERLINK(1) Quinn")))

	rows := e.exportRows(contracts.ListRequest{View: statusSent})
	if !strings.HasPrefix(rows[1][0], "'=") {
		t.Fatalf("row = %v, want the name guarded with a leading apostrophe", rows[1])
	}
}

// TestExportContracts_HoldsTheViewSearchPeriodAndOrder proves the export is
// the list's own narrowing: the same rows in the same order.
func TestExportContracts_HoldsTheViewSearchPeriodAndOrder(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	in := time.Date(2026, 10, 25, 16, 0, 0, 0, time.UTC)
	e.seedListed(statusSigned, in, fixture.NewClient(fixture.ClientName("Ann Alder")))
	e.seedListed(statusSigned, in.Add(time.Hour), fixture.NewClient(fixture.ClientName("Ann Birch")))
	e.seedListed(statusSent, in.Add(2*time.Hour), fixture.NewClient(fixture.ClientName("Ann Cedar")))
	e.seedListed(statusSigned, in.Add(3*time.Hour), fixture.NewClient(fixture.ClientName("Bea Ash")))
	e.seedListed(statusSigned, in.Add(48*time.Hour), fixture.NewClient(fixture.ClientName("Ann Elm")))

	req := contracts.ListRequest{View: statusSigned, Search: "ann", SentFrom: dayOct25, SentTo: dayOct25, Order: orderClient}
	got := clients(e.exportRows(req))
	if strings.Join(got, ",") != "Ann Alder,Ann Birch" {
		t.Fatalf("clients = %v, want Ann Alder,Ann Birch -- signed, matching ann, sent on Oct 25, by Client", got)
	}

	listed := e.search(req)
	var listedNames []string
	for _, item := range listed.Items {
		listedNames = append(listedNames, item.ClientName)
	}
	if strings.Join(listedNames, ",") != strings.Join(got, ",") {
		t.Fatalf("list = %v, export = %v, want the same rows", listedNames, got)
	}
}

// TestExportContracts_AnAdminDoesNotSeeAnotherPracticesContracts proves the
// file holds this Practice's book alone.
func TestExportContracts_AnotherPracticesContractsStayOut(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	other := newEnv(t, signedInAs(fixture.Owner()))
	other.seedEveryStatus()

	if rows := e.exportRows(contracts.ListRequest{View: viewAll}); len(rows) != 1 {
		t.Fatalf("rows = %v, want the header alone", rows)
	}
}

// TestExportContracts_AnswersAFileNamedForThePracticesDay proves the download
// headers: a filename carrying the day in the Practice's zone, the 14th at
// 9pm in New York though it is the 15th in UTC.
func TestExportContracts_AnswersAFileNamedForThePracticesDay(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), atInstant(time.Date(2026, 11, 15, 2, 0, 0, 0, time.UTC)))

	resp := e.exportRequest(contracts.ListRequest{})
	defer func() { _ = resp.Body.Close() }()

	exporttest.RequireFile(t, resp, "contracts-2026-11-14.csv", "Contract-Count", "0")
}

// TestExportContracts_RecordsWhoWhenWhatAndNeverTheSearch proves the one
// Activity row a download writes: the Practice as its subject, the Staff
// member as its actor, the slice of the book and its size, and whether a
// search was applied, never its text.
func TestExportContracts_RecordsWhoWhenWhatAndNeverTheSearch(t *testing.T) {
	now := time.Date(2026, 11, 14, 17, 0, 0, 0, time.UTC)
	e := newEnv(t, signedInAs(fixture.Owner()), atInstant(now))
	in := time.Date(2026, 10, 25, 16, 0, 0, 0, time.UTC)
	e.seedListed(statusSigned, in, fixture.NewClient(fixture.ClientName("Quinn One")))
	e.seedListed(statusSigned, in.Add(time.Hour), fixture.NewClient(fixture.ClientName("Quinn Two")))

	e.exportRows(contracts.ListRequest{View: statusSigned, Search: "quinn", SentFrom: dayOct25, SentTo: dayOct25, Order: orderNewest})

	want := map[string]any{"view": statusSigned, "sentFrom": dayOct25, "sentTo": dayOct25, "order": orderNewest, "rows": 2.0, "searched": true}
	exporttest.RequireRecorded(t, e.DB, e.Practice.ID, e.Staff.ID, "contracts_exported", now, want, "quinn")
}

// TestExportContracts_NoSearchNoPeriodRecordsTheBareView proves a download
// with no search says so, and carries no days.
func TestExportContracts_NoSearchNoPeriodRecordsTheBareView(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.exportRows(contracts.ListRequest{})

	diffs := e.exportActivity()
	if len(diffs) != 1 || diffs[0] != `{"rows": 0, "view": "not-signed", "searched": false}` {
		t.Fatalf("activity = %v, want one row of the bare view", diffs)
	}
}

// TestExportContracts_ARefusedRequestWritesNoRow proves a request the list
// would refuse is a 400 and is not recorded as a download.
func TestExportContracts_ARefusedRequestWritesNoRow(t *testing.T) {
	// jscpd:ignore-start -- the refusals the Invoices export makes, in this list's request
	cases := map[string]contracts.ListRequest{
		"a view the list does not offer": {View: "paid"},
		"a search over 100 characters":   {Search: strings.Repeat("a", 101)},
		"an order the list does not":     {Order: "random"},
		"a period with only a From":      {SentFrom: dayOct25},
		"a To before the From":           {SentFrom: "2026-10-26", SentTo: dayOct25},
	}
	// jscpd:ignore-end
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()))
			apierrtest.Expect(t, http.MethodPost, e.PracticeURL(exportPath), http.StatusBadRequest, e.SignedIn, reqtest.JSON(req))
			if got := e.exportActivity(); len(got) != 0 {
				t.Fatalf("activity = %v, want none", got)
			}
		})
	}
	t.Run("a body that is not JSON", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Owner()))
		apierrtest.Expect(t, http.MethodPost, e.PracticeURL(exportPath), http.StatusBadRequest, e.SignedIn, reqtest.RawJSON([]byte("{")))
	})
}

// TestExportContracts_OnlyAnOwnerOrAnAdmin proves who may download: an Owner
// and an Admin, and neither a Doula nor a contractor Doula, who get a 403
// and no row.
func TestExportContracts_OnlyAnOwnerOrAnAdmin(t *testing.T) {
	exporttest.RequireOwnerAndAdminOnly(t, func(t *testing.T, role fixture.RoleCase) (int, int) {
		e := newEnv(t, signedInAs(role.Opts...))
		resp := e.exportRequest(contracts.ListRequest{})
		_ = resp.Body.Close()
		return resp.StatusCode, len(e.exportActivity())
	})
}

// TestExportContracts_FeeIsThereForAReaderWhoMayReadIt proves a contractor
// who also holds the Admin role reads the Fee in the file as the list gives
// it to her, since the Admin role is what opens the Practice's money.
func TestExportContracts_FeeIsThereForAReaderWhoMayReadIt(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin(), fixture.Doula(), fixture.Contractor()))
	e.seedListed(statusSent, listBase)

	if fee := e.exportRows(contracts.ListRequest{View: statusSent})[1][8]; fee != "1500.00" {
		t.Fatalf("fee = %q, want 1500.00", fee)
	}
	if listed := e.list("?view=sent"); listed.Items[0].AmountCents == nil {
		t.Fatal("the list withholds the amount from the reader the export gave it to")
	}
}

// TestExportContracts_RateLimitedAsAHeavyEndpoint proves the 31st download in
// an hour is a 429 with the standard headers, and that it is not recorded.
func TestExportContracts_RateLimitedAsAHeavyEndpoint(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	exporttest.RequireRateLimited(t, func() *http.Response {
		return e.exportRequest(contracts.ListRequest{})
	}, func() int { return len(e.exportActivity()) })
}
