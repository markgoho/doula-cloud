package client_test

import (
	"encoding/csv"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

var clientsExportColumns = []string{
	"Client", "Kind", "Status", "Contract", "Owed", "Invoices owed", "Days overdue", "Due or birth date", "Doula",
}

const clientsExportActivity = `SELECT diff::text FROM activity WHERE practice_id = $1 AND action = 'clients_exported' ORDER BY created_at`

func newOwnerEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	return newEnv(t, append([]envOption{signedInAs(fixture.Owner())}, opts...)...)
}

func (e *env) exportRequest(req client.ListRequest) *http.Response {
	e.T.Helper()
	return reqtest.Do(e.T, http.MethodPost, e.PracticeURL("/clients/export"), e.SignedIn, reqtest.JSON(req))
}

func (e *env) exportRows(req client.ListRequest) [][]string {
	e.T.Helper()
	resp := e.exportRequest(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		e.T.Fatalf("export status = %d, want 200", resp.StatusCode)
	}
	rows, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		e.T.Fatalf("read export: %v", err)
	}
	return rows
}

func (e *env) exportDiffs() []string {
	e.T.Helper()
	rows, err := e.DB.Admin.QueryContext(e.T.Context(), clientsExportActivity, e.Practice.ID)
	if err != nil {
		e.T.Fatalf("read export activity: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var diffs []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			e.T.Fatalf("scan export activity: %v", err)
		}
		diffs = append(diffs, d)
	}
	if err := rows.Err(); err != nil {
		e.T.Fatalf("read export activity: %v", err)
	}
	return diffs
}

func TestExportClients_WritesTheNineColumnsAndNothingElse(t *testing.T) {
	e := newOwnerEnv(t)
	testdb.SeedEngagementInStatus(t, e.DB, e.Practice.ID, "Amara Okafor", "amara@example.com", "active")

	rows := e.exportRows(client.ListRequest{})
	if !slices.Equal(rows[0], clientsExportColumns) {
		t.Fatalf("header = %v, want %v", rows[0], clientsExportColumns)
	}
	for _, row := range rows {
		if len(row) != len(clientsExportColumns) {
			t.Fatalf("row = %v, want %d cells", row, len(clientsExportColumns))
		}
		if strings.Contains(strings.Join(row, ","), "@") {
			t.Fatalf("row = %v holds an email address", row)
		}
	}
}

func TestExportClients_EveryMatchingClientNotJustThePageInTheListsOrder(t *testing.T) {
	e := newOwnerEnv(t)
	for i := range 35 {
		testdb.SeedEngagementInStatus(t, e.DB, e.Practice.ID, fmt.Sprintf("Client %02d", i), "", "active")
	}

	rows := e.exportRows(client.ListRequest{Order: orderName})
	if len(rows) != 36 {
		t.Fatalf("lines = %d, want a header and 35 Clients", len(rows))
	}
	for i, row := range rows[1:] {
		if want := fmt.Sprintf("Client %02d", i); !strings.HasPrefix(row[0], want) {
			t.Fatalf("row %d = %v, want %s in name order", i, row, want)
		}
	}

	var listed []string
	var cursor string
	for {
		resp := reqtest.Do(t, http.MethodPost, e.PracticeURL("/clients/list"), e.SignedIn, reqtest.JSON(client.ListRequest{Order: orderOwed, Cursor: cursor}))
		page := reqtest.Decode[client.ListResponse](t, resp)
		_ = resp.Body.Close()
		for _, item := range page.Items {
			listed = append(listed, item.Name)
		}
		if !page.HasMore {
			break
		}
		cursor = *page.NextCursor
	}
	exported := e.exportRows(client.ListRequest{Order: orderOwed})
	for i, name := range listed {
		if exported[i+1][0] != name {
			t.Fatalf("row %d = %q, want the list's %q", i, exported[i+1][0], name)
		}
	}
}

func TestExportClients_CellsAreTheListsOwnWordsOneRowPerClient(t *testing.T) {
	e := newOwnerEnv(t)
	practiceID := e.Practice.ID
	doulaA := testdb.SeedNamedStaffAtPractice(t, e.DB, practiceID, "export-doula-a", "Bea Alder", []string{doulaRole}, "employee")
	doulaB := testdb.SeedNamedStaffAtPractice(t, e.DB, practiceID, "export-doula-b", "Cy Birch", []string{doulaRole}, "employee")

	clientID, birth := testdb.SeedEngagementInStatus(t, e.DB, practiceID, "Amara Okafor", "amara@example.com", "active")
	postpartum := seedEngagementForClient(t, e.DB, clientID, practiceID, "intake", "postpartum")
	e.Exec(`UPDATE engagements SET due_date = '2026-12-01' WHERE id = $1`, birth)
	e.Exec(`UPDATE engagements SET pregnancy_ended_on = '2026-11-02', birth_outcome = 'live_birth' WHERE id = $1`, postpartum)
	testdb.SeedGrantedAttachment(t, e.DB, birth, doulaA)
	testdb.SeedGrantedAttachment(t, e.DB, postpartum, doulaA)
	testdb.SeedGrantedAttachment(t, e.DB, postpartum, doulaB)
	contract := seedClientContract(t, e.DB, birth, contractSigned)
	late := seedClientInvoice(t, e.DB, practiceID, contract, "open", 150050)
	seedClientInvoice(t, e.DB, practiceID, contract, "open", 2000)
	e.Exec(`UPDATE invoices SET due_at = now() - interval '10 days' WHERE id = $1`, late)

	testdb.SeedNamedClient(t, e.DB, practiceID, "Dana Idle", "")
	testdb.SeedEngagementInStatus(t, e.DB, practiceID, "Eve Finished", "", "completed")

	rows := e.exportRows(client.ListRequest{All: true, Order: orderName})
	byName := map[string][]string{}
	for _, row := range rows[1:] {
		byName[row[0]] = row
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %v, want a header and three Clients, one row each", rows)
	}

	amara := byName["Amara Okafor"]
	overdue, err := strconv.Atoi(amara[6])
	if err != nil || overdue < 9 || overdue > 10 {
		t.Fatalf("Days overdue = %q, want 9 or 10", amara[6])
	}
	want := []string{"Amara Okafor", "Birth; Postpartum", "Active; Intake", "Contract signed; No Contract yet", "1520.50", "2", amara[6], "2026-12-01; 2026-11-02", "Bea Alder; Cy Birch"}
	if !slices.Equal(amara, want) {
		t.Fatalf("Amara = %v, want %v", amara, want)
	}
	if got := byName["Dana Idle"]; got[2] != "Not yet in care" || got[1] != "" || got[4] != "0.00" || got[6] != "0" {
		t.Fatalf("Dana = %v, want not yet in care and nothing owed", got)
	}
	if got := byName["Eve Finished"]; got[2] != "Care ended" || got[1] != "" {
		t.Fatalf("Eve = %v, want care ended", got)
	}
}

func TestExportClients_APendingRequestIsSaidInTheStatus(t *testing.T) {
	e := newOwnerEnv(t)
	id := testdb.SeedNamedClient(t, e.DB, e.Practice.ID, "Pending Pia", "")
	seedPendingRequest(t, e.DB, e.Practice.ID, id, e.Staff.ID, "postpartum")

	rows := e.exportRows(client.ListRequest{})
	if len(rows) != 2 || rows[1][2] != "Postpartum request pending" {
		t.Fatalf("rows = %v, want the pending request in Status", rows)
	}
}

func TestExportClients_GuardsACellASpreadsheetWouldRun(t *testing.T) {
	e := newOwnerEnv(t)
	testdb.SeedEngagementInStatus(t, e.DB, e.Practice.ID, "=HYPERLINK(1) Quinn", "", "active")

	rows := e.exportRows(client.ListRequest{})
	if !strings.HasPrefix(rows[1][0], "'=") {
		t.Fatalf("row = %v, want the name guarded with a leading apostrophe", rows[1])
	}
}

func TestExportClients_HoldsTheListsNarrowing(t *testing.T) {
	e := newOwnerEnv(t)
	practiceID := e.Practice.ID
	doula := testdb.SeedNamedStaffAtPractice(t, e.DB, practiceID, "export-narrow-doula", "Dot Elm", []string{doulaRole}, "employee")
	_, mine := testdb.SeedEngagementInStatus(t, e.DB, practiceID, "Held By Dot", "", "active")
	testdb.SeedGrantedAttachment(t, e.DB, mine, doula)
	testdb.SeedEngagementInStatus(t, e.DB, practiceID, "Held By Nobody", "", "active")
	testdb.SeedNamedClient(t, e.DB, practiceID, "Held No Work", "")

	names := func(req client.ListRequest) []string {
		var out []string
		for _, row := range e.exportRows(req)[1:] {
			out = append(out, row[0])
		}
		return out
	}
	if got := names(client.ListRequest{Doula: doula}); !slices.Equal(got, []string{"Held By Dot"}) {
		t.Fatalf("doula = %v", got)
	}
	if got := names(client.ListRequest{Name: "nobody"}); !slices.Equal(got, []string{"Held By Nobody"}) {
		t.Fatalf("name = %v", got)
	}
	if got := names(client.ListRequest{All: true}); len(got) != 3 {
		t.Fatalf("all = %v, want three", got)
	}
	if got := names(client.ListRequest{}); len(got) != 2 {
		t.Fatalf("default = %v, want the two with work", got)
	}
	if got := names(client.ListRequest{Mine: true}); len(got) != 0 {
		t.Fatalf("mine = %v, want none held by the Owner", got)
	}
	if got := names(client.ListRequest{NoContract: true}); len(got) != 2 {
		t.Fatalf("noContract = %v", got)
	}
	if got := names(client.ListRequest{Owes: true}); len(got) != 0 {
		t.Fatalf("owes = %v", got)
	}
}

func TestExportClients_MineIsTheSignedInDoulasOwnClients(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula()))
	_, mine := testdb.SeedEngagementInStatus(t, e.DB, e.Practice.ID, "Mine Mira", "", "active")
	testdb.SeedGrantedAttachment(t, e.DB, mine, e.Staff.ID)
	testdb.SeedEngagementInStatus(t, e.DB, e.Practice.ID, "Not Mine", "", "active")

	rows := e.exportRows(client.ListRequest{Mine: true})
	if len(rows) != 2 || rows[1][0] != "Mine Mira" {
		t.Fatalf("rows = %v, want only her own Client", rows)
	}
}

func TestExportClients_AnswersAFileNamedForThePracticesDayAndCountsItsRows(t *testing.T) {
	e := newOwnerEnv(t, fixture.At(time.Date(2026, 11, 15, 2, 0, 0, 0, time.UTC)))
	testdb.SeedEngagementInStatus(t, e.DB, e.Practice.ID, "Counted Cora", "", "active")

	resp := e.exportRequest(client.ListRequest{})
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("Content-Disposition"); got != `attachment; filename="clients-2026-11-14.csv"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := resp.Header.Get("Client-Count"); got != "1" {
		t.Fatalf("Client-Count = %q, want 1", got)
	}
	if resp.Header.Get("RateLimit-Limit") != "30" || resp.Header.Get("RateLimit-Remaining") != "29" {
		t.Fatalf("headers = %v, want 30 and 29", resp.Header)
	}
}

func TestExportClients_RecordsWhoWhenWhatAndNeverTheSearch(t *testing.T) {
	now := time.Date(2026, 11, 14, 17, 0, 0, 0, time.UTC)
	e := newOwnerEnv(t, fixture.At(now))
	doula := testdb.SeedNamedStaffAtPractice(t, e.DB, e.Practice.ID, "export-activity-doula", "Fay Oak", []string{doulaRole}, "employee")
	_, eng := testdb.SeedEngagementInStatus(t, e.DB, e.Practice.ID, "Quinn Reyes", "", "active")
	testdb.SeedGrantedAttachment(t, e.DB, eng, doula)
	e.Exec(`UPDATE engagements SET due_date = '2026-12-10' WHERE id = $1`, eng)

	e.exportRows(client.ListRequest{Name: "quinn", Doula: doula, Owes: false, NoContract: true, DueFrom: "2026-12-01", DueTo: "2026-12-31", Order: orderName})

	var subjectKind, subjectID, actorStaff, diff string
	var actorIsStaff bool
	var at time.Time
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT subject_kind, subject_id::text, (actor_kind = 'staff'), actor_staff_id::text, created_at, diff::text
		   FROM activity WHERE practice_id = $1 AND action = 'clients_exported'`, e.Practice.ID,
	).Scan(&subjectKind, &subjectID, &actorIsStaff, &actorStaff, &at, &diff); err != nil {
		t.Fatalf("read the export's Activity row: %v", err)
	}
	if subjectKind != "practice" || subjectID != e.Practice.ID || !actorIsStaff || actorStaff != e.Staff.ID || !at.Equal(now) {
		t.Fatalf("row = %s %s by %s at %s", subjectKind, subjectID, actorStaff, at)
	}
	var scope map[string]any
	if err := json.Unmarshal([]byte(diff), &scope); err != nil {
		t.Fatalf("diff %s: %v", diff, err)
	}
	want := map[string]any{"noContract": true, "doulaId": doula, "dueFrom": "2026-12-01", "dueTo": "2026-12-31", "order": orderName, "rows": 1.0, "searched": true}
	if len(scope) != len(want) {
		t.Fatalf("diff = %s, want exactly %v", diff, want)
	}
	for k, v := range want {
		if scope[k] != v {
			t.Fatalf("diff[%s] = %v, want %v (diff %s)", k, scope[k], v, diff)
		}
	}
	if strings.Contains(strings.ToLower(diff), "quinn") {
		t.Fatalf("diff %s holds the search's text", diff)
	}
}

func TestExportClients_NoNarrowingRecordsTheBareBook(t *testing.T) {
	e := newOwnerEnv(t)
	e.exportRows(client.ListRequest{})
	if diffs := e.exportDiffs(); len(diffs) != 1 || diffs[0] != `{"rows": 0, "searched": false}` {
		t.Fatalf("activity = %v, want one row of the bare book", diffs)
	}
}

func TestExportClients_ARefusedRequestWritesNoRow(t *testing.T) {
	cases := map[string]client.ListRequest{
		"an order the list does not offer": {Order: "random"},
		"a Doula id that is no id":         {Doula: "not-an-id"},
		"a name over 100 characters":       {Name: strings.Repeat("a", 101)},
		"a due day that is no day":         {DueFrom: "tomorrow", DueTo: "later"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			e := newOwnerEnv(t)
			reqtest.ExpectStatus(t, http.MethodPost, e.PracticeURL("/clients/export"), http.StatusBadRequest, e.SignedIn, reqtest.JSON(req))
			if got := e.exportDiffs(); len(got) != 0 {
				t.Fatalf("activity = %v, want none", got)
			}
		})
	}
	t.Run("a body that is not JSON", func(t *testing.T) {
		e := newOwnerEnv(t)
		reqtest.ExpectStatus(t, http.MethodPost, e.PracticeURL("/clients/export"), http.StatusBadRequest, e.SignedIn, reqtest.RawJSON([]byte("{")))
	})
}

func TestExportClients_OnlyAnOwnerOrAnAdmin(t *testing.T) {
	cases := []struct {
		role fixture.RoleCase
		want int
	}{
		{fixture.OwnerCase, http.StatusOK},
		{fixture.AdminCase, http.StatusOK},
		{fixture.EmployeeDoulaCase, http.StatusForbidden},
		{fixture.ContractorDoulaCase, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.role.Name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.role.Opts...))
			reqtest.ExpectStatus(t, http.MethodPost, e.PracticeURL("/clients/export"), tc.want, e.SignedIn, reqtest.JSON(client.ListRequest{}))
			wantRows := 0
			if tc.want == http.StatusOK {
				wantRows = 1
			}
			if got := len(e.exportDiffs()); got != wantRows {
				t.Fatalf("activity rows = %d, want %d", got, wantRows)
			}
		})
	}
}

func TestExportClients_ContractorAdminIsAnsweredAsTheListAnswersHer(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin(), fixture.Doula(), fixture.Contractor()))
	list := reqtest.Do(t, http.MethodPost, e.PracticeURL("/clients/list"), e.SignedIn, reqtest.JSON(client.ListRequest{}))
	_ = list.Body.Close()
	export := e.exportRequest(client.ListRequest{})
	_ = export.Body.Close()
	if export.StatusCode != list.StatusCode {
		t.Fatalf("export status = %d, list status = %d, want the same answer", export.StatusCode, list.StatusCode)
	}
}

func TestExportClients_AnotherPracticesClientsStayOut(t *testing.T) {
	e := newOwnerEnv(t)
	other := newOwnerEnv(t)
	testdb.SeedEngagementInStatus(t, other.DB, other.Practice.ID, "Elsewhere Elle", "", "active")

	if rows := e.exportRows(client.ListRequest{All: true}); len(rows) != 1 {
		t.Fatalf("rows = %v, want the header alone", rows)
	}
}

func TestExportClients_RateLimitedAsAHeavyEndpoint(t *testing.T) {
	e := newOwnerEnv(t)
	for range 30 {
		resp := e.exportRequest(client.ListRequest{})
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 inside the limit", resp.StatusCode)
		}
	}

	resp := e.exportRequest(client.ListRequest{})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("31st status = %d, want 429", resp.StatusCode)
	}
	if resp.Header.Get("RateLimit-Limit") != "30" || resp.Header.Get("RateLimit-Remaining") != "0" {
		t.Fatalf("headers = %v, want RateLimit-Limit 30 and RateLimit-Remaining 0", resp.Header)
	}
	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err != nil || secs <= 0 {
		t.Fatalf("Retry-After = %q, want seconds to wait", resp.Header.Get("Retry-After"))
	}
	var body apierr.APIError
	if err := json.UnmarshalRead(resp.Body, &body); err != nil || body.Code != apierr.CodeRateLimited {
		t.Fatalf("body = %+v (%v), want RATE_LIMITED", body, err)
	}
	if got := len(e.exportDiffs()); got != 30 {
		t.Fatalf("activity rows = %d, want the 30 that were served", got)
	}
}
