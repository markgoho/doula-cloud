package engagement_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
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

func authedPost(t *testing.T, url string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
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

func TestListHandler_ReturnsOnlyClientsAtCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	otherPracticeID := seedStaffWithMembership(t, db, "staff-elsewhere")
	seedClientEngagement(t, db, practiceID, "Jamie Client", "jamie@example.com", "intake")
	seedClientEngagement(t, db, otherPracticeID, "Other Client", "other@example.com", "intake")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/clients")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var list []engagement.ClientEngagement
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Jamie Client" {
		t.Fatalf("list = %+v, want only Jamie Client", list)
	}
}

// TestListHandler_VisibleToAnyStaffAtSamePractice proves the ticket's "any
// Staff member at a Practice sees all Clients and Engagements there, not
// just ones assigned to them" requirement: a Staff member who neither
// created nor was assigned the Client still sees it, because there is no
// staff_id column anywhere in the schema to restrict on.
func TestListHandler_VisibleToAnyStaffAtSamePractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-creator")
	seedStaffAtPractice(t, db, practiceID, "staff-bystander")
	seedClientEngagement(t, db, practiceID, "Shared Client", "shared@example.com", "intake")

	srv := newServer(fakeVerifier{uid: "staff-bystander"}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/clients")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var list []engagement.ClientEngagement
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Shared Client" {
		t.Fatalf("list = %+v, want the bystander Staff member to see Shared Client too", list)
	}
}

func TestCreateHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-creating"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(engagement.CreateClientRequest{Name: "New Client", Email: "new@example.com"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/clients", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out engagement.CreateClientResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ClientID == "" || out.EngagementID == "" || out.Status != "intake" {
		t.Fatalf("unexpected response: %+v", out)
	}

	listResp := authedGet(t, srv.URL+"/practices/"+practiceID+"/clients")
	defer listResp.Body.Close()
	var list []engagement.ClientEngagement
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0].ClientID != out.ClientID {
		t.Fatalf("expected the created client to appear in the list, got %+v", list)
	}
}

func TestCreateHandler_MissingFields(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-missing-fields"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	body, err := json.Marshal(engagement.CreateClientRequest{Name: "", Email: "new@example.com"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/clients", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestCreateHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-invalid-body"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/clients", []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestDetailHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-viewing"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Detail Client", "detail@example.com", "active")

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID)
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

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestDetailHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-bad-id"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv := newServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := authedGet(t, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
