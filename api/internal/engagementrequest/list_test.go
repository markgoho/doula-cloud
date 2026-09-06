package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// listURL is the inbox, addressed the way the screen addresses it: the
// Practice and nothing else.
func listURL(srvURL, practiceID string) string {
	return srvURL + "/api/practices/" + practiceID + "/engagement-requests"
}

// TestListHandler_GathersEveryPendingRequestOldestFirst proves the inbox
// answers #503's whole point: every pending Request at the Practice in
// one read, each row carrying the five facts the screen prints plus the
// id it links to, oldest first.
func TestListHandler_GathersEveryPendingRequestOldestFirst(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	firstClient := seedClient(t, db, practiceID)
	secondClient := seedClient(t, db, practiceID)
	oldest := pendingRequest(t, db, practiceID, firstClient, testKindBirth, doulaID)
	newest := pendingRequest(t, db, practiceID, secondClient, testKindPostpartum, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var out engagementrequest.ListResponse
	decode(t, get(t, listURL(srv.URL, practiceID), session), http.StatusOK, &out)

	if len(out.Items) != 2 || out.HasMore || out.NextCursor != nil {
		t.Fatalf("page = %d items, hasMore=%v, cursor=%v; want both Requests on one page",
			len(out.Items), out.HasMore, out.NextCursor)
	}
	if out.Items[0].RequestID != oldest || out.Items[1].RequestID != newest {
		t.Fatalf("order = %s, %s; want the oldest wait first", out.Items[0].RequestID, out.Items[1].RequestID)
	}
	row := out.Items[0]
	if row.ClientID != firstClient || row.ClientName != "Test Client" {
		t.Fatalf("client = %s/%q, want the seeded Client named", row.ClientID, row.ClientName)
	}
	if row.Kind != testKindBirth || row.DueDate == nil || *row.DueDate != testDueDate {
		t.Fatalf("ask = %s due %v, want the birth Request as it was made", row.Kind, row.DueDate)
	}
	if row.RequestedByName != "Staff doula-1" || row.RequestedAt.IsZero() {
		t.Fatalf("requester = %q at %v, want the seeded Doula and her timestamp", row.RequestedByName, row.RequestedAt)
	}
}

// TestListHandler_OmitsDecidedRequests proves the inbox is a queue of
// decisions still owed, not a history: an approved Request leaves it, and
// so does a Request whose due date was never given.
func TestListHandler_OmitsDecidedRequests(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	seedCredits(t, db, practiceID)
	decidedClient := seedClient(t, db, practiceID)
	waitingClient := seedClient(t, db, practiceID)
	decided := pendingRequest(t, db, practiceID, decidedClient, testKindBirth, doulaID)
	waiting := pendingRequest(t, db, practiceID, waitingClient, testKindBirth, doulaID)
	clearDueDate(t, db, waiting)

	srv, session := newServer(t, db, "owner-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()
	expectStatus(t, do(t, detailURL(srv.URL, practiceID, decided)+"/approve", session, nil), http.StatusOK)

	var out engagementrequest.ListResponse
	decode(t, get(t, listURL(srv.URL, practiceID), session), http.StatusOK, &out)

	if len(out.Items) != 1 || out.Items[0].RequestID != waiting {
		t.Fatalf("items = %+v, want only the Request still waiting", out.Items)
	}
	if out.Items[0].DueDate != nil {
		t.Fatalf("dueDate = %v, want nothing for a Request that named none", out.Items[0].DueDate)
	}
}

// TestListHandler_RefusesADoula proves the inbox holds the same seat the
// decisions do: a Doula cannot read a queue of decisions she cannot make.
func TestListHandler_RefusesADoula(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	expectStatus(t, get(t, listURL(srv.URL, practiceID), session), http.StatusForbidden)
}

// TestListHandler_RejectsAMalformedCursor proves a cursor nobody this
// endpoint issued is refused rather than silently treated as page one.
func TestListHandler_RejectsAMalformedCursor(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		resp := get(t, listURL(srv.URL, practiceID)+"?cursor="+cursor, session)
		if resp.status != http.StatusBadRequest {
			t.Fatalf("cursor %q: status = %d, want %d", cursor, resp.status, http.StatusBadRequest)
		}
	}
}

// TestListHandler_WalksTheCursor proves the envelope docs/api-design.md
// section 4 asks for: a full first page carries a cursor, and that cursor
// resumes at the row after the last one rather than repeating it.
func TestListHandler_WalksTheCursor(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	const total = 31 // pageSize (30) + 1, to force a second page
	for range total {
		pendingRequest(t, db, practiceID, seedClient(t, db, practiceID), testKindBirth, doulaID)
	}

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var first engagementrequest.ListResponse
	decode(t, get(t, listURL(srv.URL, practiceID), session), http.StatusOK, &first)
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	var second engagementrequest.ListResponse
	decode(t, get(t, listURL(srv.URL, practiceID)+"?cursor="+*first.NextCursor, session), http.StatusOK, &second)
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
	if second.Items[0].RequestID == first.Items[29].RequestID {
		t.Fatal("the second page repeated the first page's last row")
	}
}
