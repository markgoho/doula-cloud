package contracts_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func (e *env) voidRequestsAwaitingURL() string {
	return e.Srv.URL + "/api/practices/" + e.Practice.ID + "/contracts/void-requests"
}

// voidRequestsAwaiting fetches one page of the roll-up, with query
// appended to its route.
func (e *env) voidRequestsAwaiting(query string) contracts.VoidRequestsAwaitingResponse {
	e.T.Helper()
	return reqtest.Expect[contracts.VoidRequestsAwaitingResponse](e.T, http.MethodGet, e.voidRequestsAwaitingURL()+query, http.StatusOK, e.SignedIn)
}

// TestVoidRequestsAwaitingHandler_RefusedByRole proves the roll-up is
// Owner and Admin only, mirroring AwaitingSignatureHandler's own reach:
// void itself is Owner/Admin-only, so nobody else has anything to act on
// here.
func TestVoidRequestsAwaitingHandler_RefusedByRole(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodGet, e.voidRequestsAwaitingURL(), http.StatusForbidden, e.SignedIn)
}

// TestVoidRequestsAwaitingHandler_ListsOnlyOpenRequests proves the
// roll-up names the Client and the requester for a still-open request,
// oldest first, and leaves out a request already declined or voided --
// #971's own "sees the void requests waiting on them", not the ones
// already decided.
func TestVoidRequestsAwaitingHandler_ListsOnlyOpenRequests(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	doula := e.Colleague(fixture.Doula())

	openClientID, openEngagementID := testdb.SeedNamedEngagement(t, e.DB, e.Practice.ID, "Waiting Client", "waiting@example.com")
	// #2114: the list names each Client in full.
	testdb.NameClient(t, e.DB, openClientID, "Wren", "Okafor")
	seedContract(t, e.DB, openEngagementID, statusSigned, mergeFieldProse)
	seedVoidRequest(t, e.DB, openEngagementID, doula.ID, "the client's plan changed")

	_, declinedEngagementID := testdb.SeedNamedEngagement(t, e.DB, e.Practice.ID, "Decided Client", "decided@example.com")
	seedContract(t, e.DB, declinedEngagementID, statusSigned, mergeFieldProse)
	requestID := seedVoidRequest(t, e.DB, declinedEngagementID, doula.ID, "already handled")
	reqtest.ExpectStatus(t, http.MethodPost, e.contractURLOf(declinedEngagementID, "/void-request/"+requestID+"/decline"), http.StatusOK, e.SignedIn, reasoned("not needed"))

	out := e.voidRequestsAwaiting("")

	if len(out.Items) != 1 {
		t.Fatalf("items = %+v, want exactly the one still-open request", out.Items)
	}
	item := out.Items[0]
	if item.EngagementID != openEngagementID || item.ClientName != "Wren Okafor" || item.Reason != "the client's plan changed" {
		t.Fatalf("item = %+v, want the open request against %q", item, openEngagementID)
	}
}

// TestVoidRequestsAwaitingHandler_NamesTheContractAndCountsTheDays proves a
// request carries the Contract it asks to void, and how many whole days of
// the Practice's zone ago it was made: 0 for today, 1 a minute past the
// Practice's midnight.
func TestVoidRequestsAwaitingHandler_NamesTheContractAndCountsTheDays(t *testing.T) {
	asked := time.Date(2026, 11, 10, 4, 30, 0, 0, time.UTC)
	e := newEnv(t, signedInAs(fixture.Owner()), atInstant(asked.Add(10*time.Minute)))
	e.Exec(`UPDATE practices SET timezone = 'America/New_York' WHERE id = $1`, e.Practice.ID)
	contractID := seedListedContract(t, e.DB, e.Engagement.ID, statusSigned, asked)
	requestID := seedVoidRequest(t, e.DB, e.Engagement.ID, e.Colleague(fixture.Doula()).ID, "wrong Client")
	e.Exec(`UPDATE contract_void_requests SET created_at = $2 WHERE id = $1`, requestID, asked)

	today := e.voidRequestsAwaiting("")
	if len(today.Items) != 1 || today.Items[0].ContractID != contractID || today.Items[0].DaysAgo != 0 || today.Total != 1 {
		t.Fatalf("response = %+v, want the Contract named, 0 days ago and a total of 1", today)
	}

	e.MoveClockTo(asked.Add(30 * time.Minute))
	next := reqtest.Expect[contracts.VoidRequestsAwaitingResponse](t, http.MethodGet, e.voidRequestsAwaitingURL(), http.StatusOK, e.As(e.Staff))
	if next.Items[0].DaysAgo != 1 {
		t.Fatalf("daysAgo = %d, want 1 half an hour after the Practice's midnight", next.Items[0].DaysAgo)
	}
}

// TestVoidRequestsAwaitingHandler_ABadStoredZoneIsAnError proves a Practice
// whose stored zone will not load gets a 500 rather than the wrong day.
func TestVoidRequestsAwaitingHandler_ABadStoredZoneIsAnError(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.Exec(`UPDATE practices SET timezone = 'Nowhere/Bad' WHERE id = $1`, e.Practice.ID)

	reqtest.ExpectStatus(t, http.MethodGet, e.voidRequestsAwaitingURL(), http.StatusInternalServerError, e.SignedIn)
}

// TestVoidRequestsAwaitingHandler_RejectsAMalformedCursor mirrors
// TestAwaitingSignatureHandler_RejectsAMalformedCursor (awaiting_test.go):
// a cursor that doesn't decode is refused with the shared MsgInvalidCursor
// message, not a 500.
func TestVoidRequestsAwaitingHandler_RejectsAMalformedCursor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	reqtest.ExpectStatus(t, http.MethodGet, e.voidRequestsAwaitingURL()+"?cursor=not!valid!base64!", http.StatusBadRequest, e.SignedIn)
}

// TestVoidRequestsAwaitingHandler_WalksTheCursor mirrors
// TestAwaitingSignatureHandler_WalksTheCursor: a full first page carries
// a cursor, and that cursor resumes at the row after the last one.
func TestVoidRequestsAwaitingHandler_WalksTheCursor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	const total = 31 // voidRequestsAwaitingPageSize (30) + 1, to force a second page
	for range total {
		engagement := e.AnotherEngagement()
		seedContract(t, e.DB, engagement.ID, statusSigned, mergeFieldProse)
		seedVoidRequest(t, e.DB, engagement.ID, e.Staff.ID, "asking")
	}

	firstPage := e.voidRequestsAwaiting("")
	if len(firstPage.Items) != 30 || !firstPage.HasMore || firstPage.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(firstPage.Items), firstPage.HasMore, firstPage.NextCursor)
	}

	if firstPage.Total != total {
		t.Fatalf("first page total = %d, want every open request, %d", firstPage.Total, total)
	}

	secondPage := e.voidRequestsAwaiting("?cursor=" + *firstPage.NextCursor)
	if secondPage.Total != total {
		t.Fatalf("second page total = %d, want %d on every page", secondPage.Total, total)
	}
	if len(secondPage.Items) != 1 || secondPage.HasMore || secondPage.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(secondPage.Items), secondPage.HasMore, secondPage.NextCursor)
	}
}
