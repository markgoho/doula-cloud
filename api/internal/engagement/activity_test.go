package engagement_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// TestListActivityHandler_OwnerSeesEveryEntry proves the Owner column of
// ADR-0008's read table: nothing is filtered, including money actions.
func TestListActivityHandler_OwnerSeesEveryEntry(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-activity-full"
	practiceID := testdb.SeedPractice(t, db, "Owner Activity Full")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, "employee")
	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "owner-activity-full-client@example.com", "active")

	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionEngagementCreated), activity.StaffActor(ownerID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionInvoiceRaised), activity.StaffActor(ownerID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionInvoicePaid), activity.ClientActor(clientID))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got engagement.ActivityListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 3 {
		t.Fatalf("Owner got %d items, want 3 (no filtering)", len(got.Items))
	}
}

// TestListActivityHandler_EmployeeDoulaSeesMoneyEntries proves the
// employee-Doula column as amended by #282: Invoice/payment and
// Contract-money entries reach her the same as an Owner or Admin, since
// employment type -- not role -- is now the boundary.
func TestListActivityHandler_EmployeeDoulaSeesMoneyEntries(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-activity-money"
	practiceID := testdb.SeedPractice(t, db, "Doula Activity Money")
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "doula-activity-money-client@example.com", "active")

	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionEngagementCreated), activity.StaffActor(doulaID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionVisitLogged), activity.StaffActor(doulaID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionInvoiceRaised), activity.StaffActor(doulaID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionInvoicePaid), activity.SystemActor())
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionContractSent), activity.StaffActor(doulaID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionContractPriced), activity.StaffActor(doulaID))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got engagement.ActivityListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 6 {
		t.Fatalf("employee Doula got %d items, want 6 (no filtering)", len(got.Items))
	}
}

// TestListActivityHandler_ContractorExcludesMoneyAndPracticePrice proves
// the contractor column: neither Invoice/payment nor the Practice's
// Contract price ever reach her, per ADR-0008 as amended by #282 ("her
// own agreed fee only ... never the Practice's price"). Her own Offer
// acceptance and the Contract entity's own lifecycle (#972: no longer in
// the money set) both stay visible; only the price itself
// (contract_priced) and Invoice/payment history are held back.
func TestListActivityHandler_ContractorExcludesMoneyAndPracticePrice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "contractor-activity-money"
	practiceID := testdb.SeedPractice(t, db, "Contractor Activity Money")
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, identityUID)
	clientID, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "contractor-activity-money-client@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, engagementID, contractorID)

	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionOfferAccepted), activity.StaffActor(contractorID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionContractSigned), activity.ClientActor(clientID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionContractPriced), activity.StaffActor(contractorID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionInvoicePaid), activity.SystemActor())

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got engagement.ActivityListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gotActions := map[string]bool{}
	for _, item := range got.Items {
		gotActions[item.Action] = true
	}
	if len(got.Items) != 2 || !gotActions[string(activity.ActionOfferAccepted)] || !gotActions[string(activity.ActionContractSigned)] {
		t.Fatalf("contractor got %+v, want exactly [offer_accepted, contract_signed]", got.Items)
	}
}

// TestListActivityHandler_SystemActorRendersAsDoulaCloud asserts
// ADR-0022's exact display string -- never "System".
func TestListActivityHandler_SystemActorRendersAsDoulaCloud(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-activity-system-name"
	practiceID := testdb.SeedPractice(t, db, "Owner Activity System Name")
	testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "owner-activity-system-name-client@example.com", "active")

	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionPortalInviteSent), activity.SystemActor())

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	var got engagement.ActivityListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(got.Items))
	}
	if got.Items[0].ActorName != "Doula Cloud" {
		t.Fatalf("system actor ActorName = %q, want %q", got.Items[0].ActorName, "Doula Cloud")
	}
	if got.Items[0].ActorName == "System" {
		t.Fatal("system actor must never render as \"System\"")
	}
}

// TestListActivityHandler_InvalidEngagementIDRejected mirrors
// DetailHandler's own bad-UUID handling.
func TestListActivityHandler_InvalidEngagementIDRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-activity-bad-engagement-id"
	practiceID := testdb.SeedPractice(t, db, "Owner Activity Bad Engagement Id")
	testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, "employee")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/not-a-uuid/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestListActivityHandler_InvalidCursorRejected mirrors
// visit.TestListHandler_InvalidCursorRejected.
func TestListActivityHandler_InvalidCursorRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-activity-bad-cursor"
	practiceID := testdb.SeedPractice(t, db, "Owner Activity Bad Cursor")
	testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "owner-activity-bad-cursor-client@example.com", "active")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity?cursor=not!valid!base64!")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestListActivityHandler_PaginatesNewestFirst mirrors
// visit.TestListHandler_PaginatesNewestFirst.
func TestListActivityHandler_PaginatesNewestFirst(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-activity-paging"
	practiceID := testdb.SeedPractice(t, db, "Owner Activity Paging")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "owner-activity-paging-client@example.com", "active")

	const total = 31 // activityPageSize (30) + 1, to force a second page
	for range total {
		testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionVisitLogged), activity.StaffActor(ownerID))
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity")
	defer firstResp.Body.Close()
	var first engagement.ActivityListResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity?cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second engagement.ActivityListResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
}

// TestListActivityHandler_ContractorWithoutAttachmentNotFound mirrors
// visit.ListHandler's gate: a contractor with no open, granted
// attachment gets a 404, same as the Engagement detail read.
func TestListActivityHandler_ContractorWithoutAttachmentNotFound(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "contractor-activity-unattached"
	practiceID := testdb.SeedPractice(t, db, "Contractor Activity Unattached")
	testdb.SeedContractorAtPractice(t, db, practiceID, identityUID)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "contractor-activity-unattached-client@example.com", "active")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// reassignDiff is the diff a reassignment writes, built through the same
// constants the write side uses so this fixture cannot drift from it.
func reassignDiff(t *testing.T, before, after string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]string{
		activity.DiffKeyAssignedStaffIDBefore: before,
		activity.DiffKeyAssignedStaffIDAfter:  after,
	})
	if err != nil {
		// coverage:ignore reason: a map of strings always marshals cleanly
		t.Fatalf("marshal diff: %v", err)
	}
	return raw
}

func readActivity(t *testing.T, db *testdb.DB, practiceID, engagementID, identityUID string) engagement.ActivityListResponse {
	t.Helper()
	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got engagement.ActivityListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		// coverage:ignore reason: decode failure of a response this handler just wrote
		t.Fatalf("decode: %v", err)
	}
	return got
}

// TestListActivityHandler_ReassignmentNamesBothPeople is #887's read
// side: the ledger reads the two ids out of the diff and resolves them
// to names server-side, the way actorName is already resolved, so a
// reader is told which Doula the Visit left and which one it reached.
func TestListActivityHandler_ReassignmentNamesBothPeople(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-activity-reassign"
	practiceID := testdb.SeedPractice(t, db, "Reassignment Names")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, "employee")
	fromID := testdb.SeedStaffAtPractice(t, db, practiceID, "reassign-from", []string{doulaRole}, "employee")
	toID := testdb.SeedStaffAtPractice(t, db, practiceID, "reassign-to", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "reassign-names-client@example.com", "active")

	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectEngagement, engagementID,
		string(activity.ActionVisitReassigned), activity.StaffActor(ownerID), reassignDiff(t, fromID, toID))

	got := readActivity(t, db, practiceID, engagementID, identityUID)
	if len(got.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(got.Items))
	}
	// SeedStaffAtPractice names a Staff row "Test Staff "+identityUID, so
	// the two names asserted here are the ones the query resolved out of
	// the ids in the diff, not values this test wrote into it.
	want := "Visit reassigned from Test Staff reassign-from to Test Staff reassign-to"
	if got.Items[0].Detail != want {
		t.Fatalf("detail = %q, want %q", got.Items[0].Detail, want)
	}
}

// TestListActivityHandler_ReassignmentFromSomebodyGone holds the entry
// readable when one end of the move no longer resolves to a Staff row:
// a reader is told a colleague has gone, never shown a bare uuid (#887).
func TestListActivityHandler_ReassignmentFromSomebodyGone(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-activity-reassign-gone"
	const departedID = "11111111-2222-3333-4444-555555555555"
	practiceID := testdb.SeedPractice(t, db, "Reassignment Gone")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, "employee")
	toID := testdb.SeedStaffAtPractice(t, db, practiceID, "reassign-gone-to", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "reassign-gone-client@example.com", "active")

	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectEngagement, engagementID,
		string(activity.ActionVisitReassigned), activity.StaffActor(ownerID), reassignDiff(t, departedID, toID))

	got := readActivity(t, db, practiceID, engagementID, identityUID)
	if len(got.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(got.Items))
	}
	detail := got.Items[0].Detail
	if strings.Contains(detail, departedID) {
		t.Fatalf("detail = %q, which shows a bare uuid", detail)
	}
	want := "Visit reassigned from a former colleague to Test Staff reassign-gone-to"
	if detail != want {
		t.Fatalf("detail = %q, want %q", detail, want)
	}
}

// TestListActivityHandler_OtherActionsCarryNoDetail holds every other
// action to the rendering it has today: no detail field at all, so the
// app falls back to its own generic description (#887).
func TestListActivityHandler_OtherActionsCarryNoDetail(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-activity-no-detail"
	practiceID := testdb.SeedPractice(t, db, "No Detail")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "no-detail-client@example.com", "active")

	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID,
		string(activity.ActionVisitLogged), activity.StaffActor(ownerID))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()

	// Decoded loosely rather than into ActivityEntry, because what is
	// asserted is the key's absence, which the struct cannot show.
	var raw struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		// coverage:ignore reason: decode failure of a response this handler just wrote
		t.Fatalf("decode: %v", err)
	}
	if len(raw.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(raw.Items))
	}
	if _, has := raw.Items[0]["detail"]; has {
		t.Fatalf("a visit_logged entry carried a detail field: %v", raw.Items[0])
	}
}
