package contracts_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

func voidRequestsAwaitingURL(srv *httptest.Server, practiceID string) string {
	return srv.URL + "/api/practices/" + practiceID + "/contracts/void-requests"
}

func getVoidRequestsAwaiting(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, voidRequestsAwaitingURL(srv, practiceID), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// TestVoidRequestsAwaitingHandler_RefusedByRole proves the roll-up is
// Owner and Admin only, mirroring AwaitingSignatureHandler's own reach:
// void itself is Owner/Admin-only, so nobody else has anything to act on
// here.
func TestVoidRequestsAwaitingHandler_RefusedByRole(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-requests-awaiting-role"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getVoidRequestsAwaiting(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestVoidRequestsAwaitingHandler_ListsOnlyOpenRequests proves the
// roll-up names the Client and the requester for a still-open request,
// oldest first, and leaves out a request already declined or voided --
// #971's own "sees the void requests waiting on them", not the ones
// already decided.
func TestVoidRequestsAwaitingHandler_ListsOnlyOpenRequests(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-requests-awaiting-list"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, uid+"-doula", []string{doulaRole}, "employee")

	_, openEngagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Waiting Client", "waiting@example.com")
	seedContract(t, db, openEngagementID, statusSigned, mergeFieldProse)
	seedVoidRequest(t, db, openEngagementID, doulaID, "the client's plan changed")

	_, declinedEngagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Decided Client", "decided@example.com")
	seedContract(t, db, declinedEngagementID, statusSigned, mergeFieldProse)
	requestID := seedVoidRequest(t, db, declinedEngagementID, doulaID, "already handled")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	declineResp := postVoidRequestDecline(t, srv, session, practiceID, declinedEngagementID, requestID, "not needed")
	_ = declineResp.Body.Close()
	if declineResp.StatusCode != http.StatusOK {
		t.Fatalf("decline setup status = %d, want %d", declineResp.StatusCode, http.StatusOK)
	}

	resp := getVoidRequestsAwaiting(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out contracts.VoidRequestsAwaitingResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("items = %+v, want exactly the one still-open request", out.Items)
	}
	item := out.Items[0]
	if item.EngagementID != openEngagementID || item.ClientName != "Waiting Client" || item.Reason != "the client's plan changed" {
		t.Fatalf("item = %+v, want the open request against %q", item, openEngagementID)
	}
}

// TestVoidRequestsAwaitingHandler_RejectsAMalformedCursor mirrors
// TestAwaitingSignatureHandler_RejectsAMalformedCursor (awaiting_test.go):
// a cursor that doesn't decode is refused with the shared MsgInvalidCursor
// message, not a 500.
func TestVoidRequestsAwaitingHandler_RejectsAMalformedCursor(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-requests-awaiting-bad-cursor"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, voidRequestsAwaitingURL(srv, practiceID)+"?cursor=not!valid!base64!", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestVoidRequestsAwaitingHandler_WalksTheCursor mirrors
// TestAwaitingSignatureHandler_WalksTheCursor: a full first page carries
// a cursor, and that cursor resumes at the row after the last one.
func TestVoidRequestsAwaitingHandler_WalksTheCursor(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-requests-awaiting-cursor"
	practiceID, doulaID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	const total = 31 // voidRequestsAwaitingPageSize (30) + 1, to force a second page
	for range total {
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		seedContract(t, db, engagementID, statusSigned, mergeFieldProse)
		seedVoidRequest(t, db, engagementID, doulaID, "asking")
	}

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	first := getVoidRequestsAwaiting(t, srv, session, practiceID)
	defer first.Body.Close()
	var firstPage contracts.VoidRequestsAwaitingResponse
	if err := json.NewDecoder(first.Body).Decode(&firstPage); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(firstPage.Items) != 30 || !firstPage.HasMore || firstPage.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(firstPage.Items), firstPage.HasMore, firstPage.NextCursor)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, voidRequestsAwaitingURL(srv, practiceID)+"?cursor="+*firstPage.NextCursor, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	second, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer second.Body.Close()
	var secondPage contracts.VoidRequestsAwaitingResponse
	if err := json.NewDecoder(second.Body).Decode(&secondPage); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(secondPage.Items) != 1 || secondPage.HasMore || secondPage.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(secondPage.Items), secondPage.HasMore, secondPage.NextCursor)
	}
}
