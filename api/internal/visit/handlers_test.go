package visit_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

func authedGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func authedPost(t *testing.T, url string) *http.Response {
	t.Helper()
	return authedBody(t, http.MethodPost, url, nil)
}

func authedPatch(t *testing.T, url string, body []byte) *http.Response {
	t.Helper()
	return authedBody(t, http.MethodPatch, url, body)
}

func authedBody(t *testing.T, method, url string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
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
	const identityUID = "office-manager-creating"
	practiceID := seedPractice(t, db)
	seedStaffAtPracticeWithRoles(t, db, practiceID, identityUID, []string{officeManagerRole})
	engagementID := seedEngagement(t, db, practiceID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestCreateHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-bad-engagement"
	practiceID, _ := seedDoulaWithMembership(t, db, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid/visits")
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
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
	seedStaffAtPracticeWithRoles(t, db, practiceID, "office-manager-bystander", []string{officeManagerRole})
	engagementID := seedEngagement(t, db, practiceID)
	seedVisit(t, db, engagementID, staffID)

	srv := newServer(fakeVerifier{uid: "office-manager-bystander"}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestListHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-list-bad-engagement"
	practiceID, _ := seedDoulaWithMembership(t, db, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid/visits")
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, srv.URL+"/practices/"+practiceID+"/engagements/"+otherEngagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestReassignHandler_ForbiddenForNonDoulaCaller(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedStaffAtPracticeWithRoles(t, db, practiceID, "office-manager-reassigning", []string{officeManagerRole})
	doulaStaffID := seedStaffAtPracticeWithRoles(t, db, practiceID, "doula-bystander", []string{doulaRole})
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, doulaStaffID)

	srv := newServer(fakeVerifier{uid: "office-manager-reassigning"}, db)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: doulaStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: "00000000-0000-0000-0000-000000000000"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_TargetNotDoula(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-non-doula-target"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)
	nonDoulaStaffID := seedStaffAtPracticeWithRoles(t, db, practiceID, "office-manager-target", []string{officeManagerRole})
	engagementID := seedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, staffID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: nonDoulaStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/00000000-0000-0000-0000-000000000000", body)
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedPatch(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, []byte("not json"))
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: "not-a-uuid"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestReassignHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassign-bad-engagement-id"
	practiceID, staffID := seedDoulaWithMembership(t, db, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid/visits/00000000-0000-0000-0000-000000000000", body)
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: staffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/visits/not-a-uuid", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
