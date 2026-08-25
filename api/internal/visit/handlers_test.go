package visit_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

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
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	engagementID := seedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
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

func TestCreateHandler_ForbiddenForNonDoula(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "admin-creating"
	practiceID := seedPractice(t, db)
	seedStaffAtPracticeWithRoles(t, db, practiceID, identityUID, []string{adminRole})
	engagementID := seedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestCreateHandler_EngagementNotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-wrong-practice"
	practiceID, _ := seedDoulaWithMembership(t, db, identityUID)
	otherPracticeID, _ := seedDoulaWithMembership(t, db, "doula-elsewhere")
	engagementID := seedEngagement(t, db, otherPracticeID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestCreateHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-bad-engagement"
	practiceID, _ := seedDoulaWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestListHandler_ReturnsVisitsForEngagement(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	engagementID := seedEngagement(t, db, practiceID)
	seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var list []visit.Visit
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 || list[0].StaffID != staffID {
		t.Fatalf("list = %+v, want one Visit assigned to %q", list, staffID)
	}
}

func TestListHandler_VisibleToNonDoulaStaff(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := seedDoulaWithMembership(t, db, "doula-creator")
	seedStaffAtPracticeWithRoles(t, db, practiceID, "admin-bystander", []string{adminRole})
	engagementID := seedEngagement(t, db, practiceID)
	seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, "admin-bystander")
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
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
	practiceID, staffID := seedDoulaWithMembership(t, db, "doula-owner-of-visits")
	contractorUID := "contractor-unattached-visits"
	seedContractorAtPractice(t, db, practiceID, contractorUID)
	engagementID := seedEngagement(t, db, practiceID)
	seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestListHandler_ContractorWithGrantedAttachmentSucceeds proves the
// other half: an open, granted attachment reaches.
func TestListHandler_ContractorWithGrantedAttachmentSucceeds(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := seedDoulaWithMembership(t, db, "doula-owner-of-visits-2")
	contractorUID := "contractor-attached-visits"
	contractorStaffID := seedContractorAtPractice(t, db, practiceID, contractorUID)
	engagementID := seedEngagement(t, db, practiceID)
	seedVisit(t, db, engagementID, staffID)
	seedGrantedAttachment(t, db, engagementID, contractorStaffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestListHandler_EngagementNotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-list-wrong-practice"
	practiceID, _ := seedDoulaWithMembership(t, db, identityUID)
	otherPracticeID, _ := seedDoulaWithMembership(t, db, "doula-list-elsewhere")
	engagementID := seedEngagement(t, db, otherPracticeID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestListHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-list-bad-engagement"
	practiceID, _ := seedDoulaWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassigning"
	practiceID, creatorStaffID := seedDoulaWithMembership(t, db, identityUID)
	targetStaffID := seedStaffAtPracticeWithRoles(t, db, practiceID, "doula-target", []string{doulaRole})
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, creatorStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
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
}

func TestReassignHandler_EngagementNotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-wrong-practice"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	otherPracticeID, otherStaffID := seedDoulaWithMembership(t, db, "doula-reassign-elsewhere")
	otherEngagementID := seedEngagement(t, db, otherPracticeID)
	visitID := seedVisit(t, db, otherEngagementID, otherStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+otherEngagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestReassignHandler_ForbiddenForNonDoulaCaller(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedStaffAtPracticeWithRoles(t, db, practiceID, "admin-reassigning", []string{adminRole})
	doulaStaffID := seedStaffAtPracticeWithRoles(t, db, practiceID, "doula-bystander", []string{doulaRole})
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, doulaStaffID)

	srv, session := newServer(t, db, "admin-reassigning")
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: doulaStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestReassignHandler_TargetNotStaffAtPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-unknown-target"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: "00000000-0000-0000-0000-000000000000"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_TargetNotDoula(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-non-doula-target"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	nonDoulaStaffID := seedStaffAtPracticeWithRoles(t, db, practiceID, "admin-target", []string{adminRole})
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: nonDoulaStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_VisitNotFound(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-missing-visit"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	engagementID := seedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/00000000-0000-0000-0000-000000000000", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestReassignHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-bad-body"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_InvalidStaffID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-bad-staff-id"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: "not-a-uuid"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-bad-engagement-id"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid/visits/00000000-0000-0000-0000-000000000000", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_InvalidVisitID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-bad-visit-id"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	engagementID := seedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/not-a-uuid", body)
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
	practiceID, creatorStaffID := seedDoulaWithMembership(t, db, identityUID)
	targetStaffID := seedStaffAtPracticeWithRoles(t, db, practiceID, "doula-employee-target", []string{doulaRole})
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, creatorStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
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
	if origin != "granted" || attachedBy != creatorStaffID {
		t.Fatalf("attachment = %s by %s, want granted by the person who handed it over", origin, attachedBy)
	}
}

// "A contractor can only be attached by her own acceptance of an Offer:
// nobody can put an outsider on a Client's birth without her agreement."
// So handing her a Visit is refused until she has accepted one.
func TestReassignHandler_RefusesAContractorWhoHasNotAccepted(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-to-contractor"
	practiceID, creatorStaffID := seedDoulaWithMembership(t, db, identityUID)
	targetStaffID := seedContractorAtPractice(t, db, practiceID, "contractor-target")
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, creatorStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
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
	practiceID, creatorStaffID := seedDoulaWithMembership(t, db, identityUID)
	targetStaffID := seedContractorAtPractice(t, db, practiceID, "contractor-attached")
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, creatorStaffID)
	seedGrantedAttachment(t, db, engagementID, targetStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// Logging her own Visit puts an employee Doula on the birth, granted.
func TestCreateHandler_GrantsTheEmployeeWhoLoggedTheVisit(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-logging"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	engagementID := seedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
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
	if origin != "granted" {
		t.Fatalf("origin = %q, want granted", origin)
	}
}

// A contractor logging a Visit gets no grant: that would hand her the
// reach an Offer exists to ask for. She is left with whatever the
// write-side seam accrues, which is a record and never a key.
func TestCreateHandler_GrantsNothingToAContractorWhoLoggedAVisit(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "contractor-logging"
	practiceID := seedPractice(t, db)
	staffID := seedContractorAtPractice(t, db, practiceID, identityUID)
	engagementID := seedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND origin = 'granted'`,
		engagementID, staffID,
	).Scan(&count); err != nil {
		t.Fatalf("count granted attachments: %v", err)
	}
	if count != 0 {
		t.Fatalf("granted attachments = %d, want none", count)
	}
}
