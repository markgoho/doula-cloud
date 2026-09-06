package engagement_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// seedActivity writes one activity row for engagementID via the real
// activity.Record path (not a bare INSERT), so these tests exercise the
// same write shape every #476 call site uses.
func seedActivity(t *testing.T, db *testdb.DB, practiceID, engagementID string, action activity.EngagementAction, actor activity.Actor) {
	t.Helper()
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := activity.Record(t.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(action),
		Actor:       actor,
	}); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// TestListActivityHandler_OwnerSeesEveryEntry proves the Owner column of
// ADR-0008's read table: nothing is filtered, including money actions.
func TestListActivityHandler_OwnerSeesEveryEntry(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-activity-full"
	practiceID := testdb.SeedPractice(t, db, "Owner Activity Full")
	ownerID := seedOwnerAtPractice(t, db, practiceID, identityUID)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "owner-activity-full-client@example.com", "active")

	seedActivity(t, db, practiceID, engagementID, activity.ActionEngagementCreated, activity.StaffActor(ownerID))
	seedActivity(t, db, practiceID, engagementID, activity.ActionInvoiceRaised, activity.StaffActor(ownerID))
	seedActivity(t, db, practiceID, engagementID, activity.ActionInvoicePaid, activity.ClientActor(clientID))

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

// TestListActivityHandler_EmployeeDoulaExcludesMoneyEntries proves the
// employee-Doula column: Invoice/payment (and Contract-money) entries
// never reach her, per ADR-0008.
func TestListActivityHandler_EmployeeDoulaExcludesMoneyEntries(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-activity-money"
	practiceID := testdb.SeedPractice(t, db, "Doula Activity Money")
	doulaID := seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "doula-activity-money-client@example.com", "active")

	seedActivity(t, db, practiceID, engagementID, activity.ActionEngagementCreated, activity.StaffActor(doulaID))
	seedActivity(t, db, practiceID, engagementID, activity.ActionVisitLogged, activity.StaffActor(doulaID))
	seedActivity(t, db, practiceID, engagementID, activity.ActionInvoiceRaised, activity.StaffActor(doulaID))
	seedActivity(t, db, practiceID, engagementID, activity.ActionInvoicePaid, activity.SystemActor())
	seedActivity(t, db, practiceID, engagementID, activity.ActionContractSent, activity.StaffActor(doulaID))

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
	if len(got.Items) != 2 {
		t.Fatalf("employee Doula got %d items, want 2 (engagement_created, visit_logged only)", len(got.Items))
	}
	for _, item := range got.Items {
		if item.Action == string(activity.ActionInvoiceRaised) || item.Action == string(activity.ActionInvoicePaid) || item.Action == string(activity.ActionContractSent) {
			t.Fatalf("employee Doula's result set contained money action %q", item.Action)
		}
	}
}

// TestListActivityHandler_ContractorExcludesMoneyAndPracticePrice proves
// the contractor column: same exclusion set as an employee -- neither
// Invoice/payment nor the Practice's Contract price ever reach her,
// per ADR-0008 ("her own agreed fee only ... never the Practice's
// price"). Her own Offer acceptance is not in the money set and stays
// visible.
func TestListActivityHandler_ContractorExcludesMoneyAndPracticePrice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "contractor-activity-money"
	practiceID := testdb.SeedPractice(t, db, "Contractor Activity Money")
	contractorID := seedContractorAtPractice(t, db, practiceID, identityUID)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "contractor-activity-money-client@example.com", "active")
	seedGrantedAttachment(t, db, engagementID, contractorID)

	seedActivity(t, db, practiceID, engagementID, activity.ActionOfferAccepted, activity.StaffActor(contractorID))
	seedActivity(t, db, practiceID, engagementID, activity.ActionContractSigned, activity.ClientActor(clientID))
	seedActivity(t, db, practiceID, engagementID, activity.ActionInvoicePaid, activity.SystemActor())

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
	if len(got.Items) != 1 || got.Items[0].Action != string(activity.ActionOfferAccepted) {
		t.Fatalf("contractor got %+v, want exactly [offer_accepted]", got.Items)
	}
}

// TestListActivityHandler_SystemActorRendersAsDoulaCloud asserts
// ADR-0022's exact display string -- never "System".
func TestListActivityHandler_SystemActorRendersAsDoulaCloud(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-activity-system-name"
	practiceID := testdb.SeedPractice(t, db, "Owner Activity System Name")
	seedOwnerAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "owner-activity-system-name-client@example.com", "active")

	seedActivity(t, db, practiceID, engagementID, activity.ActionPortalInviteSent, activity.SystemActor())

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
	seedOwnerAtPractice(t, db, practiceID, identityUID)

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
	seedOwnerAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "owner-activity-bad-cursor-client@example.com", "active")

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
	ownerID := seedOwnerAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "owner-activity-paging-client@example.com", "active")

	const total = 31 // activityPageSize (30) + 1, to force a second page
	for range total {
		seedActivity(t, db, practiceID, engagementID, activity.ActionVisitLogged, activity.StaffActor(ownerID))
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
	seedContractorAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "contractor-activity-unattached-client@example.com", "active")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}
