package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// TestRefuseHandler_RequiresReasonAndStampsRequest proves a reason is
// required at the endpoint and, when given, the Request is stamped
// refused with the actor, timestamp, and reason the CHECK constraints
// demand.
func TestRefuseHandler_RequiresReasonAndStampsRequest(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	noReason := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/"+requestID+"/refuse", session,
		engagementrequest.RefuseRequest{})
	expectStatus(t, noReason, http.StatusBadRequest)

	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/"+requestID+"/refuse", session,
		engagementrequest.RefuseRequest{Reason: "already has a doula elsewhere"})
	var out engagementrequest.DecisionResponse
	decode(t, resp, http.StatusOK, &out)
	if out.State != "refused" {
		t.Fatalf("state = %q, want refused", out.State)
	}

	row := readRequest(t, db, requestID)
	if row.state != "refused" || row.decidedBy == nil || row.reason == nil || *row.reason == "" {
		t.Fatalf("request row = %+v, want refused/decided/reasoned", row)
	}
}

// TestRefuseHandler_InvalidRequestIDRejected proves a malformed path
// segment is a 400, not a query against a bogus id.
func TestRefuseHandler_InvalidRequestIDRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/not-a-uuid/refuse", session,
		engagementrequest.RefuseRequest{Reason: "no"})
	expectStatus(t, resp, http.StatusBadRequest)
}

// TestRefuseHandler_InvalidBodyRejected proves malformed JSON is a 400.
func TestRefuseHandler_InvalidBodyRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/"+requestID+"/refuse", session, "not json")
	expectStatus(t, resp, http.StatusBadRequest)
}

// TestRefuseHandler_NotFound proves a bogus request id is a 404.
func TestRefuseHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/00000000-0000-0000-0000-000000000000/refuse", session,
		engagementrequest.RefuseRequest{Reason: "no"})
	expectStatus(t, resp, http.StatusNotFound)
}

// TestDatabase_RefusalWithoutReasonRejected proves
// engagement_requests_refusal_reason (00042) rejects a reasonless refusal
// at the database itself, independent of the endpoint's own 400.
func TestDatabase_RefusalWithoutReasonRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	_, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_requests SET state = 'refused', decided_by = $1, decided_at = now() WHERE id = $2`,
		doulaID, requestID,
	)
	if err == nil {
		t.Fatal("expected the refusal_reason CHECK to reject a reasonless refused row, got no error")
	}
}

// TestRefuseHandler_DoulaForbidden proves refuse is Owner/Admin only.
func TestRefuseHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/"+requestID+"/refuse", session,
		engagementrequest.RefuseRequest{Reason: "no"})
	expectStatus(t, resp, http.StatusForbidden)
}

// TestRefuseHandler_AlreadyDecidedConflict proves refusing an already
// decided Request is a 409.
func TestRefuseHandler_AlreadyDecidedConflict(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	first := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/"+requestID+"/refuse", session,
		engagementrequest.RefuseRequest{Reason: "no"})
	expectStatus(t, first, http.StatusOK)

	second := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/"+requestID+"/refuse", session,
		engagementrequest.RefuseRequest{Reason: "no"})
	expectStatus(t, second, http.StatusConflict)
}
