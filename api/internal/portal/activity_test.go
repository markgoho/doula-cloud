package portal_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
)

// activityServer is newServer under another name: portal.Mount already
// registers ActivityHandler alongside DetailHandler, so both test files
// share one mount.
func activityServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	return newServer(t, db, uid)
}

// ownerRole avoids goconst flagging the literal at every seed call below.
const ownerRole = "owner"

// seedEngagementForActivity mirrors seedClientWithEngagement but also
// returns practiceID, which activity seeding needs and the existing
// helper has no caller that does.
func seedEngagementForActivity(t *testing.T, db *testdb.DB, identityUID, practiceName string) (practiceID, engagementID string) {
	t.Helper()

	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ($1) RETURNING id`, practiceName,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Test Client', 'activity-client@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, 'active', 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	testdb.SeedPortalAccount(t, db, identityUID, identityUID+"@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		identityUID, clientID,
	); err != nil {
		t.Fatalf("seed client_portal_users: %v", err)
	}
	return practiceID, engagementID
}

// seedActivity writes one activity row via the real activity.Record path,
// against subject_kind 'engagement' -- the only kind this package's own
// reader ever queries -- mirroring engagement_test.seedActivity (#706).
func seedActivity(t *testing.T, db *testdb.DB, practiceID, engagementID, action string, actor activity.Actor) {
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
		Action:      action,
		Actor:       actor,
	}); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func authedActivityGet(t *testing.T, session, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
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

// TestActivityHandler_ReturnsOwnEngagementActivity proves #486 AC4/AC5's
// record-scoped read reaches the Client portal at all: her own
// Engagement's activity, not a bare id.
func TestActivityHandler_ReturnsOwnEngagementActivity(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-activity-owner"
	practiceID, engagementID := seedEngagementForActivity(t, db, identityUID, "Activity Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "portal-activity-staff", []string{ownerRole}, "employee")

	seedActivity(t, db, practiceID, engagementID, string(activity.ActionContractSent), activity.StaffActor(staffID))

	srv, session := activityServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Action != string(activity.ActionContractSent) {
		t.Fatalf("Items = %+v, want exactly the one contract_sent row", got.Items)
	}
}

// TestActivityHandler_RedactsStaffActorNames proves CONTEXT.md's Activity
// entry -- "she reads her own Activity ... never who inside the Practice
// did what" -- for the half staffingActionsNotIn's own action exclusion
// cannot cover: a Contract sent by a named Staff member is still hers to
// see (it is about her own Engagement), but who at the Practice sent it is
// not. A Client actor's own name (her signature) and the system actor's
// name are both left exactly as resolved -- only a Staff actor's own name
// is replaced.
func TestActivityHandler_RedactsStaffActorNames(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-activity-redact"
	practiceID, engagementID := seedEngagementForActivity(t, db, identityUID, "Activity Redact Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "portal-activity-redact-staff", []string{ownerRole}, "employee")

	seedActivity(t, db, practiceID, engagementID, string(activity.ActionContractSent), activity.StaffActor(staffID))
	seedActivity(t, db, practiceID, engagementID, string(activity.ActionPortalInviteSent), activity.SystemActor())

	srv, session := activityServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	byAction := map[string]activityfeed.Entry{}
	for _, entry := range got.Items {
		byAction[entry.Action] = entry
	}
	if got := byAction[string(activity.ActionContractSent)]; got.ActorName != "Your practice" {
		t.Fatalf("staff actor name = %q, want the redacted \"Your practice\", not the Staff member's own name", got.ActorName)
	}
	if got := byAction[string(activity.ActionPortalInviteSent)]; got.ActorName != activity.SystemActorName {
		t.Fatalf("system actor name = %q, want %q unredacted", got.ActorName, activity.SystemActorName)
	}
}

// TestActivityHandler_KeepsMoneyEntries proves CONTEXT.md's "her money"
// half: unlike an employed Doula under ADR-0008's money tier, a Client
// keeps every Contract and Invoice entry on her own Engagement -- this
// reader applies no money filter at all.
func TestActivityHandler_KeepsMoneyEntries(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-activity-money"
	practiceID, engagementID := seedEngagementForActivity(t, db, identityUID, "Activity Money Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "portal-activity-money-staff", []string{ownerRole}, "employee")

	seedActivity(t, db, practiceID, engagementID, string(activity.ActionInvoiceRaised), activity.StaffActor(staffID))

	srv, session := activityServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Action != string(activity.ActionInvoiceRaised) {
		t.Fatalf("Items = %+v, want the invoice_raised row kept", got.Items)
	}
}

// TestActivityHandler_HidesStaffingEntries proves CONTEXT.md's Activity
// entry: "never who inside the Practice did what" -- an Offer is which
// Doula was asked, accepted or bumped, a Practice-roster fact rather than
// a fact about her.
func TestActivityHandler_HidesStaffingEntries(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-activity-staffing"
	practiceID, engagementID := seedEngagementForActivity(t, db, identityUID, "Activity Staffing Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "portal-activity-staffing-staff", []string{ownerRole}, "employee")

	seedActivity(t, db, practiceID, engagementID, string(activity.ActionVisitLogged), activity.StaffActor(staffID))
	seedActivity(t, db, practiceID, engagementID, string(activity.ActionOfferSent), activity.StaffActor(staffID))
	seedActivity(t, db, practiceID, engagementID, string(activity.ActionVisitReassigned), activity.StaffActor(staffID))

	srv, session := activityServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Action != string(activity.ActionVisitLogged) {
		t.Fatalf("Items = %+v, want only the visit_logged row (offer/reassignment hidden)", got.Items)
	}
}

// TestActivityHandler_PaginatesNewestFirst mirrors
// engagement.TestListActivityHandler_PaginatesNewestFirst.
func TestActivityHandler_PaginatesNewestFirst(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-activity-paginate"
	practiceID, engagementID := seedEngagementForActivity(t, db, identityUID, "Activity Paginate Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "portal-activity-paginate-staff", []string{ownerRole}, "employee")

	const total = 31 // activityPageSize (30) + 1, to force a second page
	for range total {
		seedActivity(t, db, practiceID, engagementID, string(activity.ActionVisitLogged), activity.StaffActor(staffID))
	}

	srv, session := activityServer(t, db, identityUID)
	defer srv.Close()

	firstResp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/activity")
	defer firstResp.Body.Close()
	var first activityfeed.ListResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/activity?cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second activityfeed.ListResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
}

// TestActivityHandler_InvalidCursorRejected mirrors
// engagement.TestListActivityHandler_InvalidCursorRejected.
func TestActivityHandler_InvalidCursorRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-activity-bad-cursor"
	_, engagementID := seedEngagementForActivity(t, db, identityUID, "Activity Bad Cursor Practice")

	srv, session := activityServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/activity?cursor=not!valid!base64!")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
