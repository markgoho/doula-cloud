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
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Detail Client", "detail@example.com", "active")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID)
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

// TestDetailHandler_DueDate proves the Staff side reads the same
// engagements.due_date column the Client portal's page already reads
// (#505), so a Doula can see when the Engagement she's working is due
// without leaving the page (#538).
func TestDetailHandler_DueDate(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-viewing-due-date"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Due Date Client", "due-date@example.com", "active")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET due_date = '2027-06-15' WHERE id = $1`, engagementID,
	); err != nil {
		t.Fatalf("seed due_date: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var d engagement.Detail
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if d.DueDate == nil || *d.DueDate != "2027-06-15" {
		t.Fatalf("dueDate = %v, want %q", d.DueDate, "2027-06-15")
	}
}

// TestDetailHandler_NullDueDate covers ADR-0017's "genuinely none" case: a
// postpartum-only Engagement has no due date, and the field must be
// omitted from the JSON entirely -- the page relies on `omitempty` to
// tell "nothing to show" apart from a fetch that broke.
func TestDetailHandler_NullDueDate(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-viewing-null-due-date"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "No Due Date Client", "no-due-date@example.com", "active")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, present := raw["dueDate"]; present {
		t.Fatalf("dueDate key present in response, want omitted: %v", raw["dueDate"])
	}
}

func TestDetailHandler_NotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-wrong-practice"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	otherPracticeID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owns-engagement", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, otherPracticeID, "Elsewhere Client", "elsewhere@example.com", "intake")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID)
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
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-of-practice-2", []string{doulaRole}, "employee")
	testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Unattached Detail Client", "unattached-detail@example.com", "intake")

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID)
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
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "staff-owner-of-practice-3", []string{doulaRole}, "employee")
	staffID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Attached Detail Client", "attached-detail@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var d engagement.Detail
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(d.StatusMoves) != 0 {
		t.Fatalf("statusMoves = %v, want none -- ADR-0015 refuses a contractor Doula every move outright", d.StatusMoves)
	}
}

// TestDetailHandler_StatusMoves proves Detail.StatusMoves (#253) mirrors
// legalMoves for every combination TestTransitionHandler_LegalMovesByRole
// doesn't already reach through a successful write -- a bare Staff
// member with no role, and an employee Doula reading a completed
// Engagement, whose reopen move is Owner/Admin only.
func TestDetailHandler_StatusMoves(t *testing.T) {
	cases := []struct {
		name   string
		roles  []string
		status string
		want   []string
	}{
		{"bare staff, no role, at intake", []string{}, "intake", []string{}},
		{"owner at intake sees active and completed", []string{ownerRole}, "intake", []string{"active", "completed"}},
		{"owner at completed sees reopen", []string{ownerRole}, "completed", []string{"active"}},
		{"employee doula at completed sees no reopen", []string{doulaRole}, "completed", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "status-moves-" + tc.name
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, tc.roles, "employee")
			_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", uid+"@example.com", tc.status)

			srv, session := newServer(t, db, uid)
			defer srv.Close()

			resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			var d engagement.Detail
			if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(d.StatusMoves) != len(tc.want) || (len(tc.want) > 0 && d.StatusMoves[0] != tc.want[0]) {
				t.Fatalf("statusMoves = %v, want %v", d.StatusMoves, tc.want)
			}
		})
	}
}

func TestDetailHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-bad-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/not-a-uuid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
