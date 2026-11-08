package client_test

import (
	"encoding/json/v2"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

func listStatus(t *testing.T, srvURL, session, practiceID, query string) int {
	t.Helper()
	resp := authedGet(t, session, srvURL+"/api/practices/"+practiceID+"/clients"+query)
	defer resp.Body.Close()
	return resp.StatusCode
}

// listByName reads the Clients list the only way a name can be sent: a
// POST whose body holds the filters (#1829).
func listByName(t *testing.T, srvURL, session, practiceID string, req client.ListRequest) []client.ListItem {
	t.Helper()
	resp := authedJSON(t, session, http.MethodPost, srvURL+"/api/practices/"+practiceID+"/clients/list", req)
	defer resp.Body.Close()
	var out client.ListResponse
	if err := json.UnmarshalRead(resp.Body, &out); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("%+v: status = %d, err = %v", req, resp.StatusCode, err)
	}
	return out.Items
}

func listByNameStatus(t *testing.T, srvURL, session, practiceID string, req client.ListRequest) int {
	t.Helper()
	resp := authedJSON(t, session, http.MethodPost, srvURL+"/api/practices/"+practiceID+"/clients/list", req)
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestListHandler_DoulaNarrowsToClientsWithAnOpenGrantedAttachmentOnAnOpenEngagement(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-doula-filter", []string{ownerRole}, "employee")
	const kimikoUID = "employee-kimiko-doula-filter"
	kimiko := testdb.SeedStaffAtPractice(t, db, practiceID, kimikoUID, []string{doulaRole}, "employee")
	other := testdb.SeedStaffAtPractice(t, db, practiceID, "employee-other-doula-filter", []string{doulaRole}, "employee")

	hers, hersEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Hers", "hers@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, hersEngagement, kimiko)
	_, theirsEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Theirs", "theirs@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, theirsEngagement, other)
	_, completedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Completed", "completed-doula@example.com", "completed")
	testdb.SeedGrantedAttachment(t, db, completedEngagement, kimiko)
	_, endedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Ended", "ended-doula@example.com", "active")
	testdb.SeedAttachment(t, db, endedEngagement, kimiko, "granted", true)
	_, accruedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Accrued", "accrued-doula@example.com", "active")
	testdb.SeedAttachment(t, db, accruedEngagement, kimiko, "accrued", false)
	testdb.SeedEngagementInStatus(t, db, practiceID, "Unattached", "unattached-doula@example.com", "active")

	srv, session := newServer(t, db, kimikoUID)
	defer srv.Close()

	wantExactly(t, "doula", listWith(t, srv.URL, session, practiceID, "?doula="+kimiko), hers)
	wantExactly(t, "doula ignores all", listWith(t, srv.URL, session, practiceID, "?doula="+kimiko+"&all=true"), hers)
	wantExactly(t, "upper-case id", listWith(t, srv.URL, session, practiceID, "?doula="+strings.ToUpper(kimiko)), hers)
	wantExactly(t, "mine+doula for herself", listWith(t, srv.URL, session, practiceID, "?mine=true&doula="+kimiko), hers)
	wantExactly(t, "mine+doula for another", listWith(t, srv.URL, session, practiceID, "?mine=true&doula="+other))
	wantExactly(t, "no match", listWith(t, srv.URL, session, practiceID, "?doula=00000000-0000-0000-0000-000000000000"))
}

func TestListHandler_DoulaAndNameRefuseABadValue(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-bad-narrow", []string{ownerRole}, "employee")
	srv, session := newServer(t, db, "staff-owner-bad-narrow")
	defer srv.Close()

	for name, query := range map[string]string{
		"not a uuid":  "?doula=kimiko",
		"braced uuid": "?doula=%7B00000000-0000-0000-0000-000000000000%7D",
		// A name in the address is refused, never ignored (#1829): ignoring it
		// would answer with the unfiltered list.
		"name in the address":       "?q=Camille",
		"blank name in the address": "?q=",
	} {
		if got := listStatus(t, srv.URL, session, practiceID, query); got != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, got)
		}
	}
	for name, req := range map[string]client.ListRequest{
		"not a uuid":        {Doula: "kimiko"},
		"braced uuid":       {Doula: "{00000000-0000-0000-0000-000000000000}"},
		"name too long":     {Name: strings.Repeat("a", 101)},
		"name too long, ṅ":  {Name: strings.Repeat("ṅ", 101)},
		"padded past limit": {Name: strings.Repeat("a", 101) + " "},
		"bad cursor":        {Cursor: "not-a-cursor"},
	} {
		if got := listByNameStatus(t, srv.URL, session, practiceID, req); got != http.StatusBadRequest {
			t.Errorf("POST %s: status = %d, want 400", name, got)
		}
	}
	for name, req := range map[string]client.ListRequest{
		"name at the limit":      {Name: strings.Repeat("a", 100)},
		"multibyte at the limit": {Name: strings.Repeat("ṅ", 100)},
		"blank name":             {Name: "  "},
	} {
		if got := listByNameStatus(t, srv.URL, session, practiceID, req); got != http.StatusOK {
			t.Errorf("POST %s: status = %d, want 200", name, got)
		}
	}
}

func TestListHandler_PostRefusesAMalformedBody(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-bad-body", []string{ownerRole}, "employee")
	srv, session := newServer(t, db, "staff-owner-bad-body")
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/list", "not an object")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestListHandler_NameMatchesAnyOfHerNamesIgnoringCaseAndWildcards(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-name-filter", []string{ownerRole}, "employee")
	const ownerUID = "staff-owner-name-filter"

	camille, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Camille", "camille-q@example.com", "active")
	testdb.NameClient(t, db, camille, "Cami", "Boyd")
	hannah, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Hannah", "hannah-q@example.com", "active")
	testdb.NameClient(t, db, hannah, "", "Sorensen")
	percent, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "100% Real", "percent-q@example.com", "active")
	underscore, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Ann_Marie", "underscore-q@example.com", "active")
	slash, _ := testdb.SeedEngagementInStatus(t, db, practiceID, `Back\slash`, "slash-q@example.com", "active")
	plain, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "AnnXMarie", "plain-q@example.com", "active")
	noWork := testdb.SeedNamedClient(t, db, practiceID, "Camilla", "no-work-q@example.com")

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	q := func(text string) client.ListRequest { return client.ListRequest{Name: text} }
	list := func(req client.ListRequest) []client.ListItem {
		return listByName(t, srv.URL, session, practiceID, req)
	}
	wantExactly(t, "given name, any case", list(q("CAMILLE")), camille)
	wantExactly(t, "preferred name", list(q("cami")), camille)
	wantExactly(t, "family name", list(q("sorensen")), hannah)
	wantExactly(t, "full name as listed", list(q("Cami Boyd")), camille)
	wantExactly(t, "full name as recorded", list(q("Camille Boyd")), camille)
	wantExactly(t, "trimmed", list(q("  hannah  ")), hannah)
	wantExactly(t, "no match", list(q("Zelda")))
	wantExactly(t, "percent is literal", list(q("%")), percent)
	wantExactly(t, "underscore is literal", list(q("n_m")), underscore)
	wantExactly(t, "backslash is literal", list(q(`k\s`)), slash)
	wantExactly(t, "quote is data", list(q("'; DROP TABLE clients; --")))
	wantExactly(t, "blank is no filter", list(q("  ")), camille, hannah, percent, underscore, slash, plain)
	wantExactly(t, "all reaches no-work Clients", list(client.ListRequest{Name: "cami", All: true}), camille, noWork)
}

const camilleName = "camille"

func TestListHandler_DoulaAndNameCombineWithOwesMineAndAContractorsReach(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-narrow-combo", []string{ownerRole}, "employee")
	const ownerUID = "staff-owner-narrow-combo"
	const contractorUID = "contractor-narrow-combo"
	contractor := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	doula := testdb.SeedStaffAtPractice(t, db, practiceID, "employee-narrow-combo", []string{doulaRole}, "employee")

	both, bothEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Camille Owes", "both-narrow@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, bothEngagement, doula)
	testdb.SeedGrantedAttachment(t, db, bothEngagement, contractor)
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, bothEngagement, "signed"), "open", 150000)

	paid, paidEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Camille Paid", "paid-narrow@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, paidEngagement, doula)
	seedClientInvoice(t, db, practiceID, seedClientContract(t, db, paidEngagement, "signed"), "paid", 150000)

	other, otherEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Camille Other", "other-narrow@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, otherEngagement, contractor)

	srv, ownerSession := newServer(t, db, ownerUID)
	defer srv.Close()
	wantExactly(t, "doula+name", listByName(t, srv.URL, ownerSession, practiceID, client.ListRequest{Doula: doula, Name: camilleName}), both, paid)
	wantExactly(t, "doula+name+owes", listByName(t, srv.URL, ownerSession, practiceID, client.ListRequest{Doula: doula, Name: camilleName, Owes: true}), both)
	wantExactly(t, "doula+noContract", listWith(t, srv.URL, ownerSession, practiceID, "?doula="+doula+"&noContract=true"))

	contractorSession := authntest.SeedSession(t, db.App, contractorUID)
	wantExactly(t, "contractor sees only her own", listByName(t, srv.URL, contractorSession, practiceID, client.ListRequest{Name: camilleName}), both, other)
	wantExactly(t, "contractor narrowed by a Doula", listWith(t, srv.URL, contractorSession, practiceID, "?doula="+doula), both)
	wantExactly(t, "contractor narrowed by a Doula and name", listByName(t, srv.URL, contractorSession, practiceID, client.ListRequest{Doula: doula, Name: "paid"}))
}

func TestListHandler_NarrowedListPagesByCursorWithoutRepeatsOrGaps(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-narrow-pages", []string{ownerRole}, "employee")
	doula := testdb.SeedStaffAtPractice(t, db, practiceID, "employee-narrow-pages", []string{doulaRole}, "employee")
	const total = 35
	for i := range total {
		_, engagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Paged "+string(rune('A'+i%26))+string(rune('a'+i/26)), "", "active")
		testdb.SeedGrantedAttachment(t, db, engagement, doula)
	}
	for i := range 5 {
		testdb.SeedEngagementInStatus(t, db, practiceID, "Unrelated "+string(rune('A'+i)), "", "active")
	}

	srv, session := newServer(t, db, "staff-owner-narrow-pages")
	defer srv.Close()

	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/list",
			client.ListRequest{Doula: doula, Name: "paged", Cursor: cursor})
		var page client.ListResponse
		if err := json.UnmarshalRead(resp.Body, &page); err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("page %d: status = %d, err = %v", pages+1, resp.StatusCode, err)
		}
		_ = resp.Body.Close()
		pages++
		for _, item := range page.Items {
			if seen[item.ClientID] {
				t.Fatalf("Client %s appears on two pages", item.ClientID)
			}
			seen[item.ClientID] = true
		}
		if !page.HasMore {
			break
		}
		cursor = *page.NextCursor
	}
	if pages != 2 || len(seen) != total {
		t.Fatalf("pages = %d, Clients = %d, want 2 pages and %d Clients", pages, len(seen), total)
	}
}
