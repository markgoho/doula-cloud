package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func (e *env) list(query string) engagementrequest.ListResponse {
	e.T.Helper()
	return reqtest.Expect[engagementrequest.ListResponse](e.T, http.MethodGet, e.listURL()+query, http.StatusOK, e.SignedIn)
}

// TestListHandler_GathersEveryPendingRequestOldestFirst proves the inbox
// answers #503's whole point: every pending Request at the Practice in
// one read, each row carrying the five facts the screen prints plus the
// id it links to, oldest first.
func TestListHandler_GathersEveryPendingRequestOldestFirst(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	doula := e.Colleague()
	secondClient := fixture.Client(t, e.DB, e.Practice)
	// #2114: the inbox names each Client in full, the preferred name
	// followed by the family name.
	testdb.NameClient(t, e.DB, e.Client.ID, "Tess", "Okafor")
	oldest := e.pending(doula)
	newest := e.pendingFor(secondClient, testKindPostpartum, doula)

	out := e.list("")

	if len(out.Items) != 2 || out.HasMore || out.NextCursor != nil {
		t.Fatalf("page = %d items, hasMore=%v, cursor=%v; want both Requests on one page",
			len(out.Items), out.HasMore, out.NextCursor)
	}
	if out.Items[0].RequestID != oldest || out.Items[1].RequestID != newest {
		t.Fatalf("order = %s, %s; want the oldest wait first", out.Items[0].RequestID, out.Items[1].RequestID)
	}
	row := out.Items[0]
	if row.ClientID != e.Client.ID || row.ClientName != "Tess Okafor" {
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
	e := newEnv(t, signedInAs(fixture.Owner()), funded)
	doula := e.Colleague()
	decided := e.pending(doula)
	waiting := e.pendingFor(fixture.Client(t, e.DB, e.Practice), testKindBirth, doula)
	e.clearDueDate(waiting)
	e.approve(decided)

	out := e.list("")

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
	e := newEnv(t)
	reqtest.ExpectStatus(t, http.MethodGet, e.listURL(), http.StatusForbidden, e.SignedIn)
}

// TestListHandler_RejectsAMalformedCursor proves a cursor nobody this
// endpoint issued is refused rather than silently treated as page one.
func TestListHandler_RejectsAMalformedCursor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		reqtest.ExpectStatus(t, http.MethodGet, e.listURL()+"?cursor="+cursor, http.StatusBadRequest, e.SignedIn)
	}
}

// TestListHandler_WalksTheCursor proves the envelope docs/api-design.md
// section 4 asks for: a full first page carries a cursor, and that cursor
// resumes at the row after the last one rather than repeating it.
func TestListHandler_WalksTheCursor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	doula := e.Colleague()
	const total = 31 // pageSize (30) + 1, to force a second page
	for range total {
		e.pendingFor(fixture.Client(t, e.DB, e.Practice), testKindBirth, doula)
	}

	first := e.list("")
	reqtest.RequirePage(t, "first", len(first.Items), first.HasMore, first.NextCursor, 30, true)

	second := e.list("?cursor=" + *first.NextCursor)
	reqtest.RequirePage(t, "second", len(second.Items), second.HasMore, second.NextCursor, 1, false)
	if second.Items[0].RequestID == first.Items[29].RequestID {
		t.Fatal("the second page repeated the first page's last row")
	}
}
