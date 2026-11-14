package client_test

import (
	"fmt"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

const (
	dueMarch   = "2027-03"
	marchFirst = "2027-03-01"
	marchLast  = "2027-03-31"
)

func seedDueClient(t *testing.T, db *testdb.DB, practiceID, name, dueDate string) (clientID, engagementID string) {
	t.Helper()
	clientID, engagementID = testdb.SeedEngagementInStatus(t, db, practiceID, name, "", "active")
	if dueDate != "" {
		setDueDate(t, db, engagementID, dueDate)
	}
	return clientID, engagementID
}

func TestListHandler_DueNarrowsToClientsWithAnOpenEngagementDueInTheRange(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-due", []string{ownerRole}, "employee")

	first, _ := seedDueClient(t, db, practiceID, "First Day", marchFirst)
	last, _ := seedDueClient(t, db, practiceID, "Last Day", marchLast)
	seedDueClient(t, db, practiceID, "Day Before", "2027-02-28")
	seedDueClient(t, db, practiceID, "Day After", "2027-04-01")
	seedDueClient(t, db, practiceID, "No Due Date", "")
	leap, _ := seedDueClient(t, db, practiceID, "Leap Day", "2028-02-29")
	seedDueClient(t, db, practiceID, "Day After Leap", "2028-03-01")

	oneInside, birth := seedDueClient(t, db, practiceID, "One Inside", "2027-03-15")
	seedEngagementForClient(t, db, oneInside, practiceID, "active", "postpartum")
	setDueDate(t, db, birth, "2027-03-15")
	outsideOnly, _ := seedDueClient(t, db, practiceID, "Both Outside", "2027-05-01")
	outsidePostpartum := seedEngagementForClient(t, db, outsideOnly, practiceID, "active", "postpartum")
	setDueDate(t, db, outsidePostpartum, "2027-06-01")

	closed, closedEngagement := seedDueClient(t, db, practiceID, "Closed Engagement", "")
	setDueDate(t, db, closedEngagement, "2027-03-10")
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE engagements SET status = 'completed', ending_reason = $2, birth_outcome = $3 WHERE id = $1`,
		closedEngagement, testdb.EndingReasonForStatus("completed"), testdb.BirthOutcomeForStatus("completed")); err != nil {
		t.Fatalf("complete engagement: %v", err)
	}
	seedEngagementForClient(t, db, closed, practiceID, "active", "postpartum")

	srv, session := newServer(t, db, "staff-owner-due")
	defer srv.Close()

	march := []string{first, last, oneInside}
	wantExactly(t, "due month", listWith(t, srv.URL, session, practiceID, "?due="+dueMarch), march...)
	wantExactly(t, "days", listWith(t, srv.URL, session, practiceID, "?dueFrom=2027-03-01&dueTo=2027-03-31"), march...)
	wantExactly(t, "one day", listWith(t, srv.URL, session, practiceID, "?dueFrom=2027-03-31&dueTo=2027-03-31"), last)
	wantExactly(t, "first half", listWith(t, srv.URL, session, practiceID, "?dueFrom=2027-03-01&dueTo=2027-03-15"), first, oneInside)
	wantExactly(t, "leap month", listWith(t, srv.URL, session, practiceID, "?due=2028-02"), leap)
	wantExactly(t, "by name body", listByName(t, srv.URL, session, practiceID, client.ListRequest{Due: dueMarch}), march...)
	wantExactly(t, "days body", listByName(t, srv.URL, session, practiceID, client.ListRequest{DueFrom: marchFirst, DueTo: "2027-03-15"}), first, oneInside)
	wantExactly(t, "with a name", listByName(t, srv.URL, session, practiceID, client.ListRequest{Due: dueMarch, Name: "first day"}), first)
	wantExactly(t, "with all", listWith(t, srv.URL, session, practiceID, "?due="+dueMarch+"&all=true"), march...)

	for _, order := range []string{"", "name", "name-desc", "owed"} {
		page := readClients(t, srv.URL, session, practiceID, "?due="+dueMarch+"&order="+order)
		wantExactly(t, "order "+order, page.Items, march...)
		wantClientCount(t, page, len(page.Items))
	}
	wantClientCount(t, readClients(t, srv.URL, session, practiceID, "?due="+dueMarch), 3)
	wantClientCount(t, readClients(t, srv.URL, session, practiceID, "?due=2027-12"), 0)
}

func TestListHandler_DueComposesWithMineAndADoula(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-due-compose", []string{ownerRole}, "employee")
	const priyaUID = "employee-priya-due-compose"
	priya := testdb.SeedStaffAtPractice(t, db, practiceID, priyaUID, []string{doulaRole}, "employee")
	other := testdb.SeedStaffAtPractice(t, db, practiceID, "employee-other-due-compose", []string{doulaRole}, "employee")

	hersInMarch, hersEngagement := seedDueClient(t, db, practiceID, "Hers In March", "2027-03-10")
	testdb.SeedGrantedAttachment(t, db, hersEngagement, priya)
	hersInMay, mayEngagement := seedDueClient(t, db, practiceID, "Hers In May", "2027-05-10")
	testdb.SeedGrantedAttachment(t, db, mayEngagement, priya)
	theirsInMarch, theirsEngagement := seedDueClient(t, db, practiceID, "Theirs In March", "2027-03-20")
	testdb.SeedGrantedAttachment(t, db, theirsEngagement, other)

	ownerSrv, ownerSession := newServer(t, db, "staff-owner-due-compose")
	defer ownerSrv.Close()
	wantExactly(t, "owner, March", listWith(t, ownerSrv.URL, ownerSession, practiceID, "?due="+dueMarch), hersInMarch, theirsInMarch)
	wantExactly(t, "owner, her Doula", listWith(t, ownerSrv.URL, ownerSession, practiceID, "?due="+dueMarch+"&doula="+priya), hersInMarch)
	wantExactly(t, "owner, other Doula", listWith(t, ownerSrv.URL, ownerSession, practiceID, "?due=2027-05&doula="+other))
	wantExactly(t, "owner, her Doula in May", listWith(t, ownerSrv.URL, ownerSession, practiceID, "?due=2027-05&doula="+priya), hersInMay)
	wantExactly(t, "doula by body", listByName(t, ownerSrv.URL, ownerSession, practiceID, client.ListRequest{Due: dueMarch, Doula: other}), theirsInMarch)

	priyaSrv, priyaSession := newServer(t, db, priyaUID)
	defer priyaSrv.Close()
	wantExactly(t, "mine, March", listWith(t, priyaSrv.URL, priyaSession, practiceID, "?mine=true&due=2027-03"), hersInMarch)
	wantExactly(t, "mine, May by name", listWith(t, priyaSrv.URL, priyaSession, practiceID, "?mine=true&order=name&due=2027-05"), hersInMay)
	wantExactly(t, "mine, a Doula who is not her", listWith(t, priyaSrv.URL, priyaSession, practiceID, "?mine=true&due=2027-03&doula="+other))
	wantExactly(t, "March for her, unattached", listWith(t, priyaSrv.URL, priyaSession, practiceID, "?due="+dueMarch), hersInMarch, theirsInMarch)
	mine := readClients(t, priyaSrv.URL, priyaSession, practiceID, "?mine=true&due=2027-03")
	wantClientCount(t, mine, 1)
}

func TestListHandler_DueIsKeptForAContractorWhoseListIsAlreadyNarrowed(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-due-contractor", []string{ownerRole}, "employee")
	const contractorUID = "contractor-due"
	contractor := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	inMarch, marchEngagement := seedDueClient(t, db, practiceID, "Contractor March", "2027-03-05")
	testdb.SeedGrantedAttachment(t, db, marchEngagement, contractor)
	inApril, aprilEngagement := seedDueClient(t, db, practiceID, "Contractor April", "2027-04-05")
	testdb.SeedGrantedAttachment(t, db, aprilEngagement, contractor)
	seedDueClient(t, db, practiceID, "Not Hers", "2027-03-06")

	srv, _ := newServer(t, db, "staff-owner-due-contractor")
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, contractorUID)

	wantExactly(t, "contractor, any time", listWith(t, srv.URL, session, practiceID, ""), inMarch, inApril)
	wantExactly(t, "contractor, March", listWith(t, srv.URL, session, practiceID, "?due="+dueMarch), inMarch)
	wantExactly(t, "contractor, days", listWith(t, srv.URL, session, practiceID, "?dueFrom=2027-04-01&dueTo=2027-04-30"), inApril)
	wantExactly(t, "contractor, by name body", listByName(t, srv.URL, session, practiceID, client.ListRequest{Due: dueMarch}), inMarch)
	wantClientCount(t, readClients(t, srv.URL, session, practiceID, "?due="+dueMarch), 1)
	wantClientCount(t, readClients(t, srv.URL, session, practiceID, ""), 2)
}

func TestListHandler_DueCarriesACursorAcrossPagesInEveryOrder(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-due-pages", []string{ownerRole}, "employee")
	for i := range 35 {
		seedDueClient(t, db, practiceID, fmt.Sprintf("Paged %02d", i), fmt.Sprintf("2027-03-%02d", i%28+1))
	}
	for i := range 3 {
		seedDueClient(t, db, practiceID, fmt.Sprintf("Elsewhere %d", i), "2027-07-01")
	}

	srv, session := newServer(t, db, "staff-owner-due-pages")
	defer srv.Close()

	for _, order := range []string{"", "name", "name-desc", "owed"} {
		query := "?due=" + dueMarch + "&order=" + order
		wantClientCount(t, readClients(t, srv.URL, session, practiceID, query), 35)
		if got := pagedClients(t, srv.URL, session, practiceID, query); got != 35 {
			t.Fatalf("order %q: pages hold %d rows, want 35", order, got)
		}
	}
}

func TestListHandler_DueRefusesWhatIsNotADayOrAMonth(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-due-refuse", []string{ownerRole}, "employee")
	srv, session := newServer(t, db, "staff-owner-due-refuse")
	defer srv.Close()

	for name, query := range map[string]string{
		"month not a month":    "?due=2027-13",
		"month with a day":     "?due=" + dueMarch + "-01",
		"month one digit":      "?due=2027-3",
		"month and days":       "?due=" + dueMarch + "&dueFrom=2027-03-01&dueTo=2027-03-31",
		"month and one day":    "?due=" + dueMarch + "&dueTo=2027-03-31",
		"from without to":      "?dueFrom=2027-03-01",
		"to without from":      "?dueTo=2027-03-31",
		"from malformed":       "?dueFrom=March&dueTo=2027-03-31",
		"to malformed":         "?dueFrom=2027-03-01&dueTo=31",
		"from one digit":       "?dueFrom=2027-3-1&dueTo=2027-03-31",
		"from not a real day":  "?dueFrom=2027-02-30&dueTo=2027-03-31",
		"to before from":       "?dueFrom=2027-03-31&dueTo=2027-03-01",
		"with an order beside": "?due=soon&order=name",
	} {
		if got := listStatus(t, srv.URL, session, practiceID, query); got != http.StatusBadRequest {
			t.Errorf("%s (%s): status = %d, want 400", name, query, got)
		}
	}
	for name, req := range map[string]client.ListRequest{
		"body month":    {Due: "2027-13"},
		"body mixed":    {Due: dueMarch, DueTo: marchLast},
		"body no to":    {DueFrom: marchFirst},
		"body backward": {DueFrom: marchLast, DueTo: marchFirst},
	} {
		if got := listByNameStatus(t, srv.URL, session, practiceID, req); got != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, got)
		}
	}
	if got := listStatus(t, srv.URL, session, practiceID, "?dueFrom=2027-03-01&dueTo=2027-03-01"); got != http.StatusOK {
		t.Errorf("one day: status = %d, want 200", got)
	}
}
