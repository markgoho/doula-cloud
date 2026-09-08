package visit_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// TestListHandler_InvalidCursorRejected mirrors
// message.TestListHandler_InvalidCursorRejected.
func TestListHandler_InvalidCursorRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-visits-bad-cursor"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits?cursor="+cursor)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("cursor %q: status = %d, want %d", cursor, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

// TestListHandler_PaginatesNewestFirst seeds more than one page of
// Visits and walks the cursor, mirroring
// message.TestListHandler_PaginatesNewestFirst.
func TestListHandler_PaginatesNewestFirst(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-visits-paging"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	const total = 31 // pageSize (30) + 1, to force a second page
	for range total {
		seedVisit(t, db, engagementID, staffID)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer firstResp.Body.Close()
	var first visit.ListResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits?cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second visit.ListResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
}

func authedGet(t *testing.T, session, url string) *http.Response {
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

func authedPost(t *testing.T, session, url string) *http.Response {
	t.Helper()
	return authedBody(t, session, http.MethodPost, url, nil)
}

func authedPatch(t *testing.T, session, url string, body []byte) *http.Response {
	t.Helper()
	return authedBody(t, session, http.MethodPatch, url, body)
}

func authedBody(t *testing.T, session, method, url string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestCreateHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-creating"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out visit.CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.VisitID == "" || out.StaffID != staffID {
		t.Fatalf("unexpected response: %+v, want staffId = %q", out, staffID)
	}
}

// An Admin who is not a Doula has no self to put on a birth, so a create
// with no assignee is a refusal for her rather than a silent
// self-assignment (#268). Naming a colleague is what she does instead --
// TestCreateHandler_AdminWhoIsNotADoulaNamesAColleague, in assign_test.go.
func TestCreateHandler_ForbiddenForNonDoula(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "admin-creating"
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{adminRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestCreateHandler_EngagementNotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-wrong-practice"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	otherPracticeID, _ := testdb.SeedStaffAtNewPractice(t, db, "doula-elsewhere", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestCreateHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-bad-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/not-a-uuid/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestListHandler_ReturnsVisitsForEngagement(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var listResp visit.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	list := listResp.Items
	if len(list) != 1 || list[0].StaffID != staffID {
		t.Fatalf("list = %+v, want one Visit assigned to %q", list, staffID)
	}
}

func TestListHandler_VisibleToNonDoulaStaff(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "doula-creator", []string{doulaRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, "admin-bystander", []string{adminRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, "admin-bystander")
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestListHandler_ContractorWithoutAttachmentForbidden proves ADR-0008's
// attachment rule: a contractor Doula with no engagement_attachments row
// gets the same "not found" response an out-of-practice Engagement gets.
func TestListHandler_ContractorWithoutAttachmentForbidden(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "doula-owner-of-visits", []string{doulaRole}, "employee")
	contractorUID := "contractor-unattached-visits"
	testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestListHandler_ContractorWithGrantedAttachmentSucceeds proves the
// other half: an open, granted attachment reaches.
func TestListHandler_ContractorWithGrantedAttachmentSucceeds(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "doula-owner-of-visits-2", []string{doulaRole}, "employee")
	contractorUID := "contractor-attached-visits"
	contractorStaffID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedVisit(t, db, engagementID, staffID)
	testdb.SeedGrantedAttachment(t, db, engagementID, contractorStaffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestListHandler_EngagementNotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-list-wrong-practice"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	otherPracticeID, _ := testdb.SeedStaffAtNewPractice(t, db, "doula-list-elsewhere", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestListHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-list-bad-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/not-a-uuid/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassigning"
	practiceID, creatorStaffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole, doulaRole}, "employee")
	targetStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-target", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, creatorStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out visit.ReassignResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.VisitID != visitID || out.StaffID != targetStaffID {
		t.Fatalf("unexpected response: %+v", out)
	}

	// A reassignment is a move, so the entry names both ends of it: the
	// Staff member the Visit came off as well as the one it went to
	// (#887). The "before" is read under a row lock, so it is the value
	// the write actually overwrote.
	var rawDiff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff FROM activity WHERE subject_id = $1 AND action = $2`,
		engagementID, string(activity.ActionVisitReassigned),
	).Scan(&rawDiff); err != nil {
		t.Fatalf("read the reassignment's activity row: %v", err)
	}
	var diff map[string]string
	if err := json.Unmarshal(rawDiff, &diff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if diff[activity.DiffKeyAssignedStaffIDBefore] != creatorStaffID {
		t.Fatalf("%s = %q, want the Staff member the Visit came off (%q)", activity.DiffKeyAssignedStaffIDBefore, diff[activity.DiffKeyAssignedStaffIDBefore], creatorStaffID)
	}
	if diff[activity.DiffKeyAssignedStaffIDAfter] != targetStaffID {
		t.Fatalf("%s = %q, want the Staff member the Visit went to (%q)", activity.DiffKeyAssignedStaffIDAfter, diff[activity.DiffKeyAssignedStaffIDAfter], targetStaffID)
	}
}

// TestCreateHandler_LogsOnlyTheAssignee holds visit_logged's own diff to
// its single-sided shape: a creation has no "before", so it never grows
// the reassignment's two keys (#887).
func TestCreateHandler_LogsOnlyTheAssignee(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-create-diff"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole, doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	created := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", visit.CreateRequest{StaffID: &staffID})
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", created.StatusCode, http.StatusCreated)
	}

	var rawDiff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff FROM activity WHERE subject_id = $1 AND action = $2`,
		engagementID, string(activity.ActionVisitLogged),
	).Scan(&rawDiff); err != nil {
		t.Fatalf("read the creation's activity row: %v", err)
	}
	var diff map[string]string
	if err := json.Unmarshal(rawDiff, &diff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if len(diff) != 1 || diff["assignedStaffId"] != staffID {
		t.Fatalf("visit_logged diff = %v, want only assignedStaffId = %q", diff, staffID)
	}
}

func TestReassignHandler_EngagementNotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-wrong-practice"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	otherPracticeID, otherStaffID := testdb.SeedStaffAtNewPractice(t, db, "doula-reassign-elsewhere", []string{doulaRole}, "employee")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)
	visitID := seedVisit(t, db, otherEngagementID, otherStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+otherEngagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// An Admin who is not herself a Doula is the person whose job scheduling
// is (ADR-0006, ADR-0008), so handing a Visit to a colleague is hers to
// do -- #268 replaced the Doula-only guard that used to refuse her.
func TestReassignHandler_AllowedForAnAdminWhoIsNotADoula(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "admin-reassigning", []string{adminRole}, "employee")
	doulaStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-bystander", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, doulaStaffID)

	srv, session := newServer(t, db, "admin-reassigning")
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: doulaStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestReassignHandler_TargetNotStaffAtPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-unknown-target"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole, doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: "00000000-0000-0000-0000-000000000000"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_TargetNotDoula(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-non-doula-target"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole, doulaRole}, "employee")
	nonDoulaStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "admin-target", []string{adminRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: nonDoulaStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_VisitNotFound(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-missing-visit"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/00000000-0000-0000-0000-000000000000", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestReassignHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-bad-body"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_InvalidStaffID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-bad-staff-id"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: notAUUID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-bad-engagement-id"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/not-a-uuid/visits/00000000-0000-0000-0000-000000000000", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_InvalidVisitID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-bad-visit-id"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/not-a-uuid", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// CONTEXT.md's Attachment entry: "An Admin may attach an employee
// directly -- naming her on a Visit is granted, not accrued, because she
// has done nothing." Handing an employee a Visit puts her on the birth.
func TestReassignHandler_GrantsTheEmployeeItHandsTheVisitTo(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-granting"
	practiceID, creatorStaffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole, doulaRole}, "employee")
	targetStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-employee-target", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, creatorStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var origin, attachedBy string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT origin::text, attached_by::text FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2`, engagementID, targetStaffID,
	).Scan(&origin, &attachedBy); err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if origin != grantedOrigin || attachedBy != creatorStaffID {
		t.Fatalf("attachment = %s by %s, want granted by the person who handed it over", origin, attachedBy)
	}
}

// "A contractor can only be attached by her own acceptance of an Offer:
// nobody can put an outsider on a Client's birth without her agreement."
// So handing her a Visit is refused until she has accepted one.
func TestReassignHandler_RefusesAContractorWhoHasNotAccepted(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-to-contractor"
	practiceID, creatorStaffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole, doulaRole}, "employee")
	targetStaffID := testdb.SeedContractorAtPractice(t, db, practiceID, "contractor-target")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, creatorStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		engagementID, targetStaffID,
	).Scan(&count); err != nil {
		t.Fatalf("count attachments: %v", err)
	}
	if count != 0 {
		t.Fatalf("attachments = %d, want none -- nobody may put a contractor on a birth", count)
	}
}

// A contractor who has already accepted is on the birth, so handing her
// a Visit is ordinary.
func TestReassignHandler_AllowsAnAttachedContractor(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-to-attached"
	practiceID, creatorStaffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole, doulaRole}, "employee")
	targetStaffID := testdb.SeedContractorAtPractice(t, db, practiceID, "contractor-attached")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, creatorStaffID)
	testdb.SeedGrantedAttachment(t, db, engagementID, targetStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// Logging her own Visit puts an employee Doula on the birth, granted.
func TestCreateHandler_GrantsTheEmployeeWhoLoggedTheVisit(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-logging"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var origin string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT origin::text FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		engagementID, staffID,
	).Scan(&origin); err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if origin != grantedOrigin {
		t.Fatalf("origin = %q, want granted", origin)
	}
}

// A contractor logging a Visit gets no *additional* grant: that would
// hand her the reach an Offer exists to ask for. She must already be
// attached to reach the route at all -- staffauth.AttachingWrite's own
// CanAccessEngagement precheck, mounted in front of CreateHandler in
// production, refuses an unattached contractor's write before
// CreateHandler's own body ever runs (#836 wired this package's own test
// mount through visit.Mount, the same production interface, and this
// test's earlier form -- posting with no attachment at all and expecting
// 201 -- only ever passed because the test mux had never applied
// AttachingWrite). What create.go's own contractor branch guards against
// is attachActor's accrual promoting her existing granted row, or a
// second row appearing beside it; ON CONFLICT DO NOTHING is what this
// proves.
func TestCreateHandler_GrantsNothingToAContractorWhoLoggedAVisit(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "contractor-logging"
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	staffID := testdb.SeedContractorAtPractice(t, db, practiceID, identityUID)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var count int
	var origin string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(origin::text) FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NULL`,
		engagementID, staffID,
	).Scan(&count, &origin); err != nil {
		t.Fatalf("count attachments: %v", err)
	}
	if count != 1 {
		t.Fatalf("open attachments = %d, want exactly the one seeded", count)
	}
	if origin != grantedOrigin {
		t.Fatalf("origin = %q, want granted -- creating a Visit must not touch it", origin)
	}
}

// TestCreateHandler_RefusesAnUnattachedContractor is what the old form of
// the test above never actually exercised: with no attachment at all, a
// contractor cannot reach the route -- staffauth.AttachingWrite's own
// CanAccessEngagement precheck 404s before CreateHandler's own body runs,
// the same "not found" shape TestListHandler_ContractorWithoutAttachmentForbidden
// proves for the read side.
func TestCreateHandler_RefusesAnUnattachedContractor(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "contractor-unattached-logging"
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedContractorAtPractice(t, db, practiceID, identityUID)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// #250: POST .../visits now accepts an optional scheduledAt, and a Visit
// created with one carries it into both the create response and the list.
func TestCreateHandler_WithScheduledAt(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-creating-scheduled"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	const scheduledAt = "2027-03-15T14:30:00Z"
	body, err := json.Marshal(visit.CreateRequest{ScheduledAt: new(scheduledAt)})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedBody(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out visit.CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ScheduledAt == nil || !out.ScheduledAt.Equal(parseRFC3339(t, scheduledAt)) {
		t.Fatalf("ScheduledAt = %v, want %q", out.ScheduledAt, scheduledAt)
	}

	listResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer listResp.Body.Close()
	var list visit.ListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ScheduledAt == nil || !list.Items[0].ScheduledAt.Equal(parseRFC3339(t, scheduledAt)) {
		t.Fatalf("list = %+v, want one Visit scheduled at %q", list.Items, scheduledAt)
	}
}

// A Visit created with no body at all -- POST's long-standing shape --
// stays valid and unscheduled.
func TestCreateHandler_NoBodyStaysUnscheduled(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-creating-unscheduled"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out visit.CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ScheduledAt != nil {
		t.Fatalf("ScheduledAt = %v, want nil", out.ScheduledAt)
	}
}

func TestCreateHandler_InvalidScheduledAt(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-creating-bad-schedule"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.CreateRequest{ScheduledAt: new("not-a-timestamp")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedBody(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// A malformed body -- not merely an unparsable scheduledAt value, but
// invalid JSON altogether -- still refuses, even though POST's body is
// otherwise optional (apierr.DecodeJSONOptional).
func TestCreateHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-creating-bad-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedBody(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits", []byte("not json"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestScheduleHandler_SetsScheduledAt(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-scheduling"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	const scheduledAt = "2027-04-01T09:00:00Z"
	body, err := json.Marshal(visit.ScheduleRequest{ScheduledAt: new(scheduledAt)})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/schedule", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out visit.ScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.VisitID != visitID || out.ScheduledAt == nil || !out.ScheduledAt.Equal(parseRFC3339(t, scheduledAt)) {
		t.Fatalf("unexpected response: %+v", out)
	}

	var actorStaffID string
	var diffJSON []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id::text, diff FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'visit_scheduled'`,
		engagementID,
	).Scan(&actorStaffID, &diffJSON); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actorStaffID != staffID {
		t.Fatalf("activity actor = %q, want %q", actorStaffID, staffID)
	}
	var diff struct {
		Before *string `json:"scheduledAtBefore"`
		After  *string `json:"scheduledAtAfter"`
	}
	if err := json.Unmarshal(diffJSON, &diff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if diff.Before != nil {
		t.Fatalf("diff.scheduledAtBefore = %v, want nil (this Visit had no prior schedule)", diff.Before)
	}
	if diff.After == nil || !parseRFC3339(t, *diff.After).Equal(parseRFC3339(t, scheduledAt)) {
		t.Fatalf("diff.scheduledAtAfter = %v, want %q", diff.After, scheduledAt)
	}
}

// Changing an already-scheduled Visit's instant, then clearing it, proves
// ScheduleHandler is a plain "set to the given value" write in both
// directions -- not only nothing -> a value.
func TestScheduleHandler_ChangesThenClearsScheduledAt(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-rescheduling"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedScheduledVisit(t, db, engagementID, staffID, time.Date(2027, 1, 1, 8, 0, 0, 0, time.UTC))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	const changedTo = "2027-01-02T08:00:00Z"
	changeBody, err := json.Marshal(visit.ScheduleRequest{ScheduledAt: new(changedTo)})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	changeResp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/schedule", changeBody)
	defer changeResp.Body.Close()
	if changeResp.StatusCode != http.StatusOK {
		t.Fatalf("change status = %d, want %d", changeResp.StatusCode, http.StatusOK)
	}

	clearBody, err := json.Marshal(visit.ScheduleRequest{ScheduledAt: nil})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	clearResp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/schedule", clearBody)
	defer clearResp.Body.Close()
	if clearResp.StatusCode != http.StatusOK {
		t.Fatalf("clear status = %d, want %d", clearResp.StatusCode, http.StatusOK)
	}
	var out visit.ScheduleResponse
	if err := json.NewDecoder(clearResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ScheduledAt != nil {
		t.Fatalf("ScheduledAt = %v, want nil after clearing", out.ScheduledAt)
	}

	// The clear's own activity row -- newest of the two this test made --
	// records the direction covered above (nothing -> a value)'s
	// counterpart: a value -> nothing.
	var diffJSON []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'visit_scheduled'
		  ORDER BY created_at DESC LIMIT 1`,
		engagementID,
	).Scan(&diffJSON); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	var diff struct {
		Before *string `json:"scheduledAtBefore"`
		After  *string `json:"scheduledAtAfter"`
	}
	if err := json.Unmarshal(diffJSON, &diff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if diff.Before == nil || !parseRFC3339(t, *diff.Before).Equal(parseRFC3339(t, changedTo)) {
		t.Fatalf("diff.scheduledAtBefore = %v, want %q", diff.Before, changedTo)
	}
	if diff.After != nil {
		t.Fatalf("diff.scheduledAtAfter = %v, want nil", diff.After)
	}
}

func TestScheduleHandler_InvalidFormat(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-scheduling-bad-format"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ScheduleRequest{ScheduledAt: new("not-a-timestamp")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/schedule", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// Setting a Visit's date is the Admin's own job, not something the Doula
// role gates (#268) -- see ScheduleHandler's doc comment.
func TestScheduleHandler_AllowedForAnAdminWhoIsNotADoula(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "admin-scheduling", []string{adminRole}, "employee")
	doulaStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-schedule-bystander", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, doulaStaffID)

	srv, session := newServer(t, db, "admin-scheduling")
	defer srv.Close()

	body, err := json.Marshal(visit.ScheduleRequest{ScheduledAt: new("2027-04-01T09:00:00Z")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/schedule", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestScheduleHandler_EngagementNotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-schedule-wrong-practice"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	otherPracticeID, otherStaffID := testdb.SeedStaffAtNewPractice(t, db, "doula-schedule-elsewhere", []string{doulaRole}, "employee")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)
	visitID := seedVisit(t, db, otherEngagementID, otherStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ScheduleRequest{ScheduledAt: new("2027-04-01T09:00:00Z")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+otherEngagementID+"/visits/"+visitID+"/schedule", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestScheduleHandler_VisitNotFound(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-schedule-missing-visit"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ScheduleRequest{ScheduledAt: new("2027-04-01T09:00:00Z")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/00000000-0000-0000-0000-000000000000/schedule", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestScheduleHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-schedule-bad-body"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/schedule", []byte("not json"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestScheduleHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-schedule-bad-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ScheduleRequest{ScheduledAt: new("2027-04-01T09:00:00Z")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/not-a-uuid/visits/00000000-0000-0000-0000-000000000000/schedule", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestScheduleHandler_InvalidVisitID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-schedule-bad-visit-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ScheduleRequest{ScheduledAt: new("2027-04-01T09:00:00Z")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/not-a-uuid/schedule", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// staffauth.AttachingWrite's own CanAccessEngagement precheck 404s an
// unattached contractor before ScheduleHandler's own body ever runs, the
// same shape TestCreateHandler_RefusesAnUnattachedContractor proves for
// create.
func TestScheduleHandler_RefusesAnUnattachedContractor(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "doula-owner-of-schedule", []string{doulaRole}, "employee")
	contractorUID := "contractor-unattached-schedule"
	testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ScheduleRequest{ScheduledAt: new("2027-04-01T09:00:00Z")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/schedule", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestListHandler_ReturnsNotes proves list.Visit round-trips notes (#251):
// present for a Visit that has had them written, absent for one that has
// not.
func TestListHandler_ReturnsNotes(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing-notes"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	withNotes := seedVisitWithNotes(t, db, engagementID, staffID, "Client seemed anxious about the birth plan.")
	withoutNotes := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	var listResp visit.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	byID := map[string]visit.Visit{}
	for _, v := range listResp.Items {
		byID[v.VisitID] = v
	}
	if got := byID[withNotes].Notes; got == nil || *got != "Client seemed anxious about the birth plan." {
		t.Fatalf("notes = %v, want the seeded text", got)
	}
	if got := byID[withoutNotes].Notes; got != nil {
		t.Fatalf("notes = %v, want nil for a Visit that has never had notes written", got)
	}
}

// TestListHandler_ReturnsVisitType proves #281's AC directly against the
// live HTTP surface: a Visit before the Engagement's pregnancy-end date
// types prenatal, one on it types birth, one after types postpartum --
// and every row's type agrees with visit.DeriveType called independently
// on the same inputs, which is the "one derivation, every surface
// agrees" AC in one assertion.
func TestListHandler_ReturnsVisitType(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing-visit-type"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	pregnancyEndedOn := "2026-03-15"
	setPregnancyEnded(t, db, engagementID, "live_birth", pregnancyEndedOn)

	before := seedScheduledVisit(t, db, engagementID, staffID, time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC))
	onPivot := seedScheduledVisit(t, db, engagementID, staffID, time.Date(2026, 3, 15, 20, 0, 0, 0, time.UTC))
	after := seedScheduledVisit(t, db, engagementID, staffID, time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	var listResp visit.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	byID := map[string]visit.Visit{}
	for _, v := range listResp.Items {
		byID[v.VisitID] = v
	}

	for id, want := range map[string]string{
		before:  visit.TypePrenatal,
		onPivot: visit.TypeBirth,
		after:   visit.TypePostpartum,
	} {
		v, ok := byID[id]
		if !ok {
			t.Fatalf("visit %s missing from response", id)
		}
		if v.Type != want {
			t.Errorf("visit %s: type = %q, want %q", id, v.Type, want)
		}
		if agree := visit.DeriveType(*v.ScheduledAt, &pregnancyEndedOn); v.Type != agree {
			t.Errorf("visit %s: handler's type %q disagrees with DeriveType's own %q", id, v.Type, agree)
		}
	}
}

// TestListHandler_ReturnsVisitType_NoOutcomeIsPrenatal proves the AC's
// "no pivot yet" case: an Engagement with no recorded pregnancy-end date
// types every one of its Visits prenatal, whatever their own instant.
func TestListHandler_ReturnsVisitType_NoOutcomeIsPrenatal(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing-visit-type-no-outcome"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedScheduledVisit(t, db, engagementID, staffID, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	var listResp visit.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].VisitID != visitID {
		t.Fatalf("items = %+v, want the one seeded Visit", listResp.Items)
	}
	if got := listResp.Items[0].Type; got != visit.TypePrenatal {
		t.Errorf("type = %q, want prenatal for an Engagement with no pregnancy-end date", got)
	}
}

// TestListHandler_ReturnsVisitType_UnscheduledFallsBackToCreatedAt
// proves the fallback type.go's own doc comment names: a Visit nobody
// has scheduled yet still types, from when it was logged.
func TestListHandler_ReturnsVisitType_UnscheduledFallsBackToCreatedAt(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing-visit-type-unscheduled"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	setPregnancyEnded(t, db, engagementID, "live_birth", "2026-03-15")
	visitID := seedVisitCreatedAt(t, db, engagementID, staffID, time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	var listResp visit.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].VisitID != visitID {
		t.Fatalf("items = %+v, want the one seeded Visit", listResp.Items)
	}
	if got := listResp.Items[0].Type; got != visit.TypePostpartum {
		t.Errorf("type = %q, want postpartum derived from created_at since the Visit was never scheduled", got)
	}
}

// TestListHandler_ReturnsVisitType_LossTypesPostpartumNoBranch is #281's
// own AC: a Visit after a loss types postpartum, the same as after a
// live birth, with no branch on the outcome -- DeriveType takes no
// outcome parameter at all, and this proves the two Engagements agree.
func TestListHandler_ReturnsVisitType_LossTypesPostpartumNoBranch(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing-visit-type-loss"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")

	_, liveBirthEngagement := testdb.SeedEngagement(t, db, practiceID)
	setPregnancyEnded(t, db, liveBirthEngagement, "live_birth", "2026-03-15")
	liveBirthVisit := seedScheduledVisit(t, db, liveBirthEngagement, staffID, time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC))

	_, lossEngagement := testdb.SeedEngagement(t, db, practiceID)
	setPregnancyEnded(t, db, lossEngagement, "loss", "2026-03-15")
	lossVisit := seedScheduledVisit(t, db, lossEngagement, staffID, time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	for engagementID, visitID := range map[string]string{
		liveBirthEngagement: liveBirthVisit,
		lossEngagement:      lossVisit,
	} {
		resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
		defer resp.Body.Close()
		var listResp visit.ListResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(listResp.Items) != 1 || listResp.Items[0].VisitID != visitID {
			t.Fatalf("items = %+v, want the one seeded Visit", listResp.Items)
		}
		if got := listResp.Items[0].Type; got != visit.TypePostpartum {
			t.Errorf("engagement %s: type = %q, want postpartum after a loss the same as after a live birth", engagementID, got)
		}
	}
}

// TestListHandler_ReturnsVisitType_CorrectionRetypesWithNoVisitWrite is
// #281's AC that correcting the Engagement's pregnancy-end date retypes
// its existing Visits at once, with no write to the Visit row and no
// backfill -- the whole reason the type is derived rather than stored.
func TestListHandler_ReturnsVisitType_CorrectionRetypesWithNoVisitWrite(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing-visit-type-correction"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	setPregnancyEnded(t, db, engagementID, "live_birth", "2026-03-20")
	visitID := seedScheduledVisit(t, db, engagementID, staffID, time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer firstResp.Body.Close()
	var first visit.ListResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if got := first.Items[0].Type; got != visit.TypePrenatal {
		t.Fatalf("type before correction = %q, want prenatal", got)
	}

	// The date entered a week late, corrected to before the Visit's own
	// instant -- the row #293's own handler would touch, updated here
	// directly since that handler's own frozen-write rules are not what
	// this test is proving.
	setPregnancyEnded(t, db, engagementID, "live_birth", "2026-03-05")

	secondResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer secondResp.Body.Close()
	var second visit.ListResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if len(second.Items) != 1 || second.Items[0].VisitID != visitID {
		t.Fatalf("items = %+v, want the same one Visit, untouched", second.Items)
	}
	if got := second.Items[0].Type; got != visit.TypePostpartum {
		t.Errorf("type after correction = %q, want postpartum with no separate write to the Visit", got)
	}
}

func TestNotesHandler_SetsNotes(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-writing-notes"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	const notes = "Talked through the birth plan; she wants a low-intervention birth."
	body, err := json.Marshal(visit.NotesRequest{Notes: new(notes)})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/notes", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out visit.NotesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.VisitID != visitID || out.Notes != notes {
		t.Fatalf("unexpected response: %+v", out)
	}

	var storedNotes sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT notes FROM visits WHERE id = $1`, visitID).Scan(&storedNotes); err != nil {
		t.Fatalf("read stored notes: %v", err)
	}
	if !storedNotes.Valid || storedNotes.String != notes {
		t.Fatalf("stored notes = %+v, want %q", storedNotes, notes)
	}

	var actorStaffID string
	var diffJSON []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id::text, diff FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'visit_notes_edited'`,
		engagementID,
	).Scan(&actorStaffID, &diffJSON); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actorStaffID != staffID {
		t.Fatalf("activity actor = %q, want %q", actorStaffID, staffID)
	}
	var diff struct {
		VisitID  string `json:"visitId"`
		HadNotes bool   `json:"hadNotes"`
		HasNotes bool   `json:"hasNotes"`
	}
	if err := json.Unmarshal(diffJSON, &diff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if diff.VisitID != visitID {
		t.Fatalf("diff.visitId = %q, want %q", diff.VisitID, visitID)
	}
	if diff.HadNotes {
		t.Fatalf("diff.hadNotes = true, want false (this Visit had no prior notes)")
	}
	if !diff.HasNotes {
		t.Fatalf("diff.hasNotes = false, want true")
	}
	if strings.Contains(string(diffJSON), notes) {
		t.Fatalf("diff = %s, must not carry the notes text itself", diffJSON)
	}
}

// TestNotesHandler_ChangesThenClearsNotes proves NotesHandler is a plain
// "set to the given value" write in both directions, and that clearing
// notes stores the empty string rather than NULL -- distinguishable from
// a Visit that has never had notes written (TestListHandler_ReturnsNotes).
func TestNotesHandler_ChangesThenClearsNotes(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-editing-notes"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisitWithNotes(t, db, engagementID, staffID, "First note.")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	const changedTo = "Second, longer note after a follow-up call."
	changeBody, err := json.Marshal(visit.NotesRequest{Notes: new(changedTo)})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	changeResp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/notes", changeBody)
	defer changeResp.Body.Close()
	if changeResp.StatusCode != http.StatusOK {
		t.Fatalf("change status = %d, want %d", changeResp.StatusCode, http.StatusOK)
	}

	clearBody, err := json.Marshal(visit.NotesRequest{Notes: new("")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	clearResp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/notes", clearBody)
	defer clearResp.Body.Close()
	if clearResp.StatusCode != http.StatusOK {
		t.Fatalf("clear status = %d, want %d", clearResp.StatusCode, http.StatusOK)
	}
	var out visit.NotesResponse
	if err := json.NewDecoder(clearResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Notes != "" {
		t.Fatalf("Notes = %q, want empty after clearing", out.Notes)
	}

	var storedNotes sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT notes FROM visits WHERE id = $1`, visitID).Scan(&storedNotes); err != nil {
		t.Fatalf("read stored notes: %v", err)
	}
	if !storedNotes.Valid || storedNotes.String != "" {
		t.Fatalf("stored notes = %+v, want a valid empty string (cleared, not never-written)", storedNotes)
	}

	// The clear's own activity row -- newest of the two this test made.
	var diffJSON []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'visit_notes_edited'
		  ORDER BY created_at DESC LIMIT 1`,
		engagementID,
	).Scan(&diffJSON); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	var diff struct {
		HadNotes bool `json:"hadNotes"`
		HasNotes bool `json:"hasNotes"`
	}
	if err := json.Unmarshal(diffJSON, &diff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if !diff.HadNotes {
		t.Fatalf("diff.hadNotes = false, want true (this Visit had a prior note)")
	}
	if diff.HasNotes {
		t.Fatalf("diff.hasNotes = true, want false after clearing")
	}
}

// TestNotesHandler_AllowedForNonDoulaCaller states the rule notes,
// schedule and reassign now share (#268): any Staff member who may read a
// Visit may also write its notes and its date, and an Owner or Admin may
// additionally say who it is for. An Admin holding no Doula role at all
// succeeds at all three; only logging a Visit for *herself* is still
// closed to her, because she has no self on a birth to log.
func TestNotesHandler_AllowedForNonDoulaCaller(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "admin-writing-notes", []string{adminRole}, "employee")
	doulaStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-notes-bystander", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, doulaStaffID)

	srv, session := newServer(t, db, "admin-writing-notes")
	defer srv.Close()

	body, err := json.Marshal(visit.NotesRequest{Notes: new("Admin recorded this on the Doula's behalf.")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/notes", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestNotesHandler_EngagementNotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-notes-wrong-practice"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	otherPracticeID, otherStaffID := testdb.SeedStaffAtNewPractice(t, db, "doula-notes-elsewhere", []string{doulaRole}, "employee")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)
	visitID := seedVisit(t, db, otherEngagementID, otherStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.NotesRequest{Notes: new("notes")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+otherEngagementID+"/visits/"+visitID+"/notes", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestNotesHandler_VisitNotFound(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-notes-missing-visit"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.NotesRequest{Notes: new("notes")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/00000000-0000-0000-0000-000000000000/notes", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestNotesHandler_MissingNotesRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-notes-missing-field"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.NotesRequest{Notes: nil})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/notes", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestNotesHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-notes-bad-body"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/notes", []byte("not json"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestNotesHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-notes-bad-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.NotesRequest{Notes: new("notes")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/not-a-uuid/visits/00000000-0000-0000-0000-000000000000/notes", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestNotesHandler_InvalidVisitID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-notes-bad-visit-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.NotesRequest{Notes: new("notes")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/not-a-uuid/notes", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// staffauth.AttachingWrite's own CanAccessEngagement precheck 404s an
// unattached contractor before NotesHandler's own body ever runs, the same
// shape TestScheduleHandler_RefusesAnUnattachedContractor proves for
// schedule -- proving the read-parity rule holds even though NotesHandler
// asserts no role of its own.
func TestNotesHandler_RefusesAnUnattachedContractor(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "doula-owner-of-notes", []string{doulaRole}, "employee")
	contractorUID := "contractor-unattached-notes"
	testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	body, err := json.Marshal(visit.NotesRequest{Notes: new("notes")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/notes", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestNotesHandler_AllowsAnAttachedContractor is the other half: a
// contractor with an open, granted attachment reaches the write even
// though she is not the Visit's own assigned staff_id -- "any Staff
// member who may read a Visit may also write its notes" holds for her
// too.
func TestNotesHandler_AllowsAnAttachedContractor(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "doula-owner-of-notes-2", []string{doulaRole}, "employee")
	contractorUID := "contractor-attached-notes"
	contractorStaffID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)
	testdb.SeedGrantedAttachment(t, db, engagementID, contractorStaffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	body, err := json.Marshal(visit.NotesRequest{Notes: new("Contractor's own observation from the Visit.")})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/notes", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func parseRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return parsed
}
