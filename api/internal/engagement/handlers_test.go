package engagement_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
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

func TestDetailHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-viewing"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Detail Client", "detail@example.com", "active")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var d engagement.Detail
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if d.EngagementID != engagementID || d.ClientName != "Detail Client" || d.Status != "active" {
		t.Fatalf("unexpected detail: %+v", d)
	}
}

func TestDetailHandler_NotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-wrong-practice"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	otherPracticeID := seedStaffWithMembership(t, db, "staff-owns-engagement")
	_, engagementID := seedClientEngagement(t, db, otherPracticeID, "Elsewhere Client", "elsewhere@example.com", "intake")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestDetailHandler_ContractorWithoutAttachmentForbidden proves the
// per-Engagement half of ADR-0008's attachment rule: same "not found"
// response an out-of-practice Engagement gets, so a contractor can't
// distinguish "doesn't exist" from "not attached" (#230).
func TestDetailHandler_ContractorWithoutAttachmentForbidden(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "contractor-unattached-detail"
	practiceID := seedStaffWithMembership(t, db, "staff-owner-of-practice-2")
	seedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Unattached Detail Client", "unattached-detail@example.com", "intake")

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestDetailHandler_ContractorWithGrantedAttachmentSucceeds proves the
// other half: an open, granted attachment reaches.
func TestDetailHandler_ContractorWithGrantedAttachmentSucceeds(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "contractor-attached-detail"
	practiceID := seedStaffWithMembership(t, db, "staff-owner-of-practice-3")
	staffID := seedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Attached Detail Client", "attached-detail@example.com", "active")
	seedGrantedAttachment(t, db, engagementID, staffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestDetailHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-bad-id"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
