package client_test

import (
	"cmp"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

func setDueDate(t *testing.T, db *testdb.DB, engagementID, dueDate string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET due_date = $2::date WHERE id = $1`, engagementID, dueDate,
	); err != nil {
		t.Fatalf("set due date: %v", err)
	}
}

func idsInOrder(items []client.ListItem) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ClientID
	}
	return ids
}

// walkList follows nextCursor from the first page of query to the last and
// returns each page's items.
func walkList(t *testing.T, srvURL, session, practiceID, query string) [][]client.ListItem {
	t.Helper()
	var pages [][]client.ListItem
	cursor := ""
	for {
		url := srvURL + "/api/practices/" + practiceID + "/clients" + query
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		resp := authedGet(t, session, url)
		var out client.ListResponse
		err := json.UnmarshalRead(resp.Body, &out)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status = %d, err = %v", url, resp.StatusCode, err)
		}
		pages = append(pages, out.Items)
		if !out.HasMore || out.NextCursor == nil {
			return pages
		}
		cursor = *out.NextCursor
	}
}

func flatten(pages [][]client.ListItem) []client.ListItem {
	var all []client.ListItem
	for _, page := range pages {
		all = append(all, page...)
	}
	return all
}

func TestListHandler_OrdersBySoonestOpenDueDateWithUndatedLastAndPastFirst(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Due Order Practice")
	const ownerUID = "staff-due-order-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	// Seeded out of due order, so only the ORDER BY can put them right.
	later, laterEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Later", "later@example.com", "active")
	setDueDate(t, db, laterEngagement, "2027-03-01")

	undated, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Undated", "undated@example.com", "active")

	soon, soonEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Soon", "soon@example.com", "active")
	setDueDate(t, db, soonEngagement, "2027-01-10")

	past, pastEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Past", "past@example.com", "active")
	setDueDate(t, db, pastEngagement, "2020-01-01")

	// Two open Engagements: the sooner date places her, whichever kind holds it.
	two, birthEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Two Open", "two@example.com", "active")
	setDueDate(t, db, birthEngagement, "2027-02-01")
	setDueDate(t, db, seedEngagementForClient(t, db, two, practiceID, "active", "postpartum"), "2026-12-01")

	// A completed Engagement's earlier date does not place her: her open one does.
	doneEarlier, openEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Done Earlier", "done-earlier@example.com", "active")
	setDueDate(t, db, openEngagement, "2027-04-01")
	setDueDate(t, db, seedEngagementForClient(t, db, doneEarlier, practiceID, "completed", "birth"), "2019-01-01")

	// No open Engagement at all: undated, however old her completed one is.
	onlyDone, onlyDoneEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Only Done", "only-done@example.com", "completed")
	setDueDate(t, db, onlyDoneEngagement, "2018-01-01")

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	got := listWith(t, srv.URL, session, practiceID, "?all=true")
	dated := idsInOrder(got[:5])
	want := []string{past, two, soon, later, doneEarlier}
	if !slices.Equal(dated, want) {
		t.Fatalf("dated order = %v, want %v", dated, want)
	}
	tail := []string{undated, onlyDone}
	slices.Sort(tail)
	if !slices.Equal(idsInOrder(got[5:]), tail) {
		t.Fatalf("undated tail = %v, want %v (by id)", idsInOrder(got[5:]), tail)
	}

	// Each line carries its own date, and a line without one carries none.
	byClient := map[string]client.ListItem{}
	for _, item := range got {
		byClient[item.ClientID] = item
	}
	if lines := byClient[soon].OpenEngagements; len(lines) != 1 || lines[0].DueDate == nil || *lines[0].DueDate != "2027-01-10" {
		t.Fatalf("soon lines = %+v, want dueDate 2027-01-10", lines)
	}
	if lines := byClient[undated].OpenEngagements; len(lines) != 1 || lines[0].DueDate != nil {
		t.Fatalf("undated lines = %+v, want no dueDate", lines)
	}
	if lines := byClient[two].OpenEngagements; len(lines) != 2 {
		t.Fatalf("two lines = %+v, want 2", lines)
	}
}

func TestListHandler_DueOrderHoldsAcrossPagesAtADatedBoundary(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Due Paging Practice")
	const ownerUID = "staff-due-paging-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	// 31 dated Clients, seeded newest-due first, so the page break falls
	// between two dated Clients; then three undated ones.
	const dated = 31
	want := make([]string, dated)
	for i := range dated {
		id, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Dated", fmt.Sprintf("dated-%d@example.com", i), "active")
		due := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, dated-1-i)
		setDueDate(t, db, engagementID, due.Format(time.DateOnly))
		want[dated-1-i] = id
	}
	var undated []string
	for i := range 3 {
		id, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Undated", fmt.Sprintf("undated-%d@example.com", i), "active")
		undated = append(undated, id)
	}
	slices.Sort(undated)
	want = append(want, undated...)

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	pages := walkList(t, srv.URL, session, practiceID, "?all=true")
	if len(pages) != 2 || len(pages[0]) != 30 || len(pages[1]) != 4 {
		t.Fatalf("pages = %d (%d, ...), want 2 pages of 30 and 4", len(pages), len(pages[0]))
	}
	if got := idsInOrder(flatten(pages)); !slices.Equal(got, want) {
		t.Fatalf("order across pages = %v, want %v", got, want)
	}
}

func TestListHandler_DueOrderHoldsAcrossPagesInTheUndatedRun(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Undated Paging Practice")
	const ownerUID = "staff-undated-paging-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	dated, datedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Dated", "dated@example.com", "active")
	setDueDate(t, db, datedEngagement, "2027-01-10")
	var undated []string
	for i := range 31 {
		id, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Undated", fmt.Sprintf("undated-%d@example.com", i), "active")
		undated = append(undated, id)
	}
	slices.Sort(undated)
	want := append([]string{dated}, undated...)

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	pages := walkList(t, srv.URL, session, practiceID, "?all=true")
	if len(pages) != 2 || len(pages[0]) != 30 || len(pages[1]) != 2 {
		t.Fatalf("pages = %d, want 2 pages of 30 and 2", len(pages))
	}
	if got := idsInOrder(flatten(pages)); !slices.Equal(got, want) {
		t.Fatalf("order across pages = %v, want %v", got, want)
	}
}

func TestListHandler_DueOrderHoldsUnderTheFiltersAndForAnAttachedReader(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-due-filters", []string{ownerRole}, "employee")
	const doulaUID = "employee-doula-due-filters"
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, doulaUID, []string{doulaRole}, "employee")

	type seeded struct{ id, due string }
	owesMine := []seeded{{due: "2027-03-01"}, {due: "2027-01-05"}, {due: ""}, {due: "2026-11-30"}}
	for i := range owesMine {
		id, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Mine Owes", fmt.Sprintf("mine-owes-%d@example.com", i), "active")
		owesMine[i].id = id
		testdb.SeedGrantedAttachment(t, db, engagementID, doulaID)
		seedClientInvoice(t, db, practiceID, seedClientContract(t, db, engagementID, "signed"), "open", 150000)
		if owesMine[i].due != "" {
			setDueDate(t, db, engagementID, owesMine[i].due)
		}
	}
	// Owes but not hers, and due soonest of all: only the unnarrowed list shows her.
	theirs, theirsEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Their Owes", "their-owes@example.com", "active")
	setDueDate(t, db, theirsEngagement, "2020-06-01")
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, theirsEngagement, "signed"), "open", 150000)

	// Past Intake with no Contract: hers, and due between her others.
	noContract, noContractEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Mine No Contract", "mine-none@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, noContractEngagement, doulaID)
	setDueDate(t, db, noContractEngagement, "2027-02-01")

	sorted := slices.Clone(owesMine)
	slices.SortFunc(sorted, func(a, b seeded) int {
		switch {
		case a.due == "" && b.due == "":
			return cmp.Compare(a.id, b.id)
		case a.due == "":
			return 1
		case b.due == "":
			return -1
		}
		return cmp.Compare(a.due, b.due)
	})
	wantOwesMine := make([]string, len(sorted))
	for i, s := range sorted {
		wantOwesMine[i] = s.id
	}

	srv, session := newServer(t, db, doulaUID)
	defer srv.Close()

	check := func(what, query string, want ...string) {
		t.Helper()
		if got := idsInOrder(flatten(walkList(t, srv.URL, session, practiceID, query))); !slices.Equal(got, want) {
			t.Fatalf("%s: order = %v, want %v", what, got, want)
		}
	}
	check("mine+owes", "?mine=true&owes=true", wantOwesMine...)
	check("owes", "?owes=true", append([]string{theirs}, wantOwesMine...)...)
	check("mine+noContract", "?mine=true&noContract=true", noContract)
	check("mine", "?mine=true", owesMine[3].id, owesMine[1].id, noContract, owesMine[0].id, owesMine[2].id)
}
