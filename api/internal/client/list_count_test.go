package client_test

import (
	"fmt"
	"net/http"
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// readClients reads one page of the Clients list at query and requires the 200.
func readClients(t *testing.T, srvURL, session, practiceID, query string) client.ListResponse {
	t.Helper()
	resp := authedGet(t, session, srvURL+"/api/practices/"+practiceID+"/clients"+query)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET clients%s = %d, want 200", query, resp.StatusCode)
	}
	return reqtest.Decode[client.ListResponse](t, resp)
}

func wantClientCount(t *testing.T, page client.ListResponse, want int) {
	t.Helper()
	if page.ClientCount == nil {
		t.Fatalf("clientCount absent, want %d", want)
	}
	if *page.ClientCount != want {
		t.Fatalf("clientCount = %d, want %d", *page.ClientCount, want)
	}
}

// pagedClients walks every page of the list at query and returns how many rows it holds.
func pagedClients(t *testing.T, srvURL, session, practiceID, query string) int {
	t.Helper()
	rows := 0
	page := readClients(t, srvURL, session, practiceID, query)
	for {
		rows += len(page.Items)
		if page.NextCursor == nil {
			return rows
		}
		page = readClients(t, srvURL, session, practiceID, query+"&cursor="+*page.NextCursor)
	}
}

// TestListHandler_FirstPageCarriesTheTotalOfTheNarrowing is #2867: the first
// page says how many Clients the narrowing matches, whole rather than the
// page's, and a page after it says nothing.
func TestListHandler_FirstPageCarriesTheTotalOfTheNarrowing(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-client-count", []string{ownerRole}, "employee")
	for i := range 31 {
		testdb.SeedEngagementInStatus(t, db, practiceID, fmt.Sprintf("Counted %02d", i), "", "active")
	}
	testdb.SeedNamedClient(t, db, practiceID, "Idle Client", "")

	srv, session := newServer(t, db, "staff-owner-client-count")
	defer srv.Close()

	first := readClients(t, srv.URL, session, practiceID, "?all=false")
	if len(first.Items) != 30 || !first.HasMore {
		t.Fatalf("first page = %d items, hasMore=%v, want 30/true", len(first.Items), first.HasMore)
	}
	wantClientCount(t, first, 31)

	second := readClients(t, srv.URL, session, practiceID, "?cursor="+*first.NextCursor)
	if second.ClientCount != nil {
		t.Fatalf("second page clientCount = %d, want it absent", *second.ClientCount)
	}

	wantClientCount(t, readClients(t, srv.URL, session, practiceID, "?all=true"), 32)

	named := listByNameResponse(t, srv.URL, session, practiceID, client.ListRequest{Name: "counted 07"})
	wantClientCount(t, named, 1)
}

func listByNameResponse(t *testing.T, srvURL, session, practiceID string, req client.ListRequest) client.ListResponse {
	t.Helper()
	resp := authedJSON(t, session, http.MethodPost, srvURL+"/api/practices/"+practiceID+"/clients/list", req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST clients/list = %d, want 200", resp.StatusCode)
	}
	return reqtest.Decode[client.ListResponse](t, resp)
}

// TestListHandler_ContractorTotalIsTheRowsSheCanPageThrough: her total is
// narrowed by her attachments as her rows are, so it never tells her the
// Practice's size; with none it is 0.
func TestListHandler_ContractorTotalIsTheRowsSheCanPageThrough(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-contractor-count", []string{doulaRole}, "employee")
	const contractorUID = "contractor-count"
	staffID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	const attached = 31
	for i := range attached {
		_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Mine", fmt.Sprintf("count-mine-%d@example.com", i), "active")
		testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
	}
	for i := range 4 {
		testdb.SeedEngagementInStatus(t, db, practiceID, "Not mine", fmt.Sprintf("count-other-%d@example.com", i), "active")
	}

	const unattachedUID = "contractor-count-none"
	testdb.SeedContractorAtPractice(t, db, practiceID, unattachedUID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	wantClientCount(t, readClients(t, srv.URL, session, practiceID, "?all=true"), attached)
	if rows := pagedClients(t, srv.URL, session, practiceID, "?all=true"); rows != attached {
		t.Fatalf("rows paged through = %d, want the total %d", rows, attached)
	}

	noneSrv, noneSession := newServer(t, db, unattachedUID)
	defer noneSrv.Close()
	wantClientCount(t, readClients(t, noneSrv.URL, noneSession, practiceID, "?all=true"), 0)
}

// TestListHandler_MineTotalIsHerOwnClients: ?mine=true narrows the count the
// way it narrows the rows.
func TestListHandler_MineTotalIsHerOwnClients(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-mine-count", []string{ownerRole}, "employee")
	const doulaUID = "employee-doula-mine-count"
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, doulaUID, []string{doulaRole}, "employee")
	for i := range 2 {
		_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Mine", fmt.Sprintf("mine-count-%d@example.com", i), "active")
		testdb.SeedGrantedAttachment(t, db, engagementID, doulaID)
	}
	testdb.SeedEngagementInStatus(t, db, practiceID, "Theirs", "theirs-count@example.com", "active")

	srv, session := newServer(t, db, doulaUID)
	defer srv.Close()

	wantClientCount(t, readClients(t, srv.URL, session, practiceID, ""), 3)
	wantClientCount(t, readClients(t, srv.URL, session, practiceID, "?mine=true"), 2)
}
