package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
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
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	admin := fixture.Staff(t, db, p, fixture.Admin())
	firstClient := fixture.Client(t, db, p)
	secondClient := fixture.Client(t, db, p)
	// #2114: the inbox names each Client in full, the preferred name
	// followed by the family name.
	testdb.NameClient(t, db, firstClient.ID, "Tess", "Okafor")
	oldest := pendingRequest(t, db, p, firstClient, testKindBirth, doula)
	newest := pendingRequest(t, db, p, secondClient, testKindPostpartum, doula)

	srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	outResp := reqtest.Get(t, listURL(srv.URL, p.ID), reqtest.Session(session))
	defer outResp.Body.Close()
	out := reqtest.DecodeStatus[engagementrequest.ListResponse](t, outResp, http.StatusOK)

	if len(out.Items) != 2 || out.HasMore || out.NextCursor != nil {
		t.Fatalf("page = %d items, hasMore=%v, cursor=%v; want both Requests on one page",
			len(out.Items), out.HasMore, out.NextCursor)
	}
	if out.Items[0].RequestID != oldest || out.Items[1].RequestID != newest {
		t.Fatalf("order = %s, %s; want the oldest wait first", out.Items[0].RequestID, out.Items[1].RequestID)
	}
	row := out.Items[0]
	if row.ClientID != firstClient.ID || row.ClientName != "Tess Okafor" {
		t.Fatalf("client = %s/%q, want the seeded Client named in full", row.ClientID, row.ClientName)
	}
	if row.Kind != testKindBirth || row.DueDate == nil || *row.DueDate != testDueDate {
		t.Fatalf("ask = %s due %v, want the birth Request as it was made", row.Kind, row.DueDate)
	}
	if row.RequestedByName != doula.Name || row.RequestedAt.IsZero() {
		t.Fatalf("requester = %q at %v, want the seeded Doula and her timestamp", row.RequestedByName, row.RequestedAt)
	}
}

// TestListHandler_OmitsDecidedRequests proves the inbox is a queue of
// decisions still owed, not a history: an approved Request leaves it, and
// so does a Request whose due date was never given.
func TestListHandler_OmitsDecidedRequests(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.Credits(3))
	doula := fixture.Staff(t, db, p)
	owner := fixture.Staff(t, db, p, fixture.Owner())
	decidedClient := fixture.Client(t, db, p)
	waitingClient := fixture.Client(t, db, p)
	decided := pendingRequest(t, db, p, decidedClient, testKindBirth, doula)
	waiting := pendingRequest(t, db, p, waitingClient, testKindBirth, doula)
	clearDueDate(t, db, waiting)

	srv, session := newServer(t, db, owner, &tasknudge.FakeEnqueuer{})
	defer srv.Close()
	resp := reqtest.Post(t, detailURL(srv.URL, p.ID, decided)+"/approve", reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)

	outResp := reqtest.Get(t, listURL(srv.URL, p.ID), reqtest.Session(session))
	defer outResp.Body.Close()
	out := reqtest.DecodeStatus[engagementrequest.ListResponse](t, outResp, http.StatusOK)

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
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)

	srv, session := newServer(t, db, doula, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := reqtest.Get(t, listURL(srv.URL, p.ID), reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusForbidden)
}

// TestListHandler_RejectsAMalformedCursor proves a cursor nobody this
// endpoint issued is refused rather than silently treated as page one.
func TestListHandler_RejectsAMalformedCursor(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	admin := fixture.Staff(t, db, p, fixture.Admin())

	srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		resp := reqtest.Get(t, listURL(srv.URL, p.ID)+"?cursor="+cursor, reqtest.Session(session))
		defer resp.Body.Close()
		reqtest.RequireStatus(t, resp, http.StatusBadRequest)
	}
}

// TestListHandler_WalksTheCursor proves the envelope docs/api-design.md
// section 4 asks for: a full first page carries a cursor, and that cursor
// resumes at the row after the last one rather than repeating it.
func TestListHandler_WalksTheCursor(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	admin := fixture.Staff(t, db, p, fixture.Admin())
	const total = 31 // pageSize (30) + 1, to force a second page
	for range total {
		pendingRequest(t, db, p, fixture.Client(t, db, p), testKindBirth, doula)
	}

	srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	firstResp := reqtest.Get(t, listURL(srv.URL, p.ID), reqtest.Session(session))
	defer firstResp.Body.Close()
	first := reqtest.DecodeStatus[engagementrequest.ListResponse](t, firstResp, http.StatusOK)
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := reqtest.Get(t, listURL(srv.URL, p.ID)+"?cursor="+*first.NextCursor, reqtest.Session(session))
	defer secondResp.Body.Close()
	second := reqtest.DecodeStatus[engagementrequest.ListResponse](t, secondResp, http.StatusOK)
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
	if second.Items[0].RequestID == first.Items[29].RequestID {
		t.Fatal("the second page repeated the first page's last row")
	}
}
