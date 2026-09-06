package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// TestWithdrawHandler_RequesterWithdrawsOwnPendingRequest proves the
// requester may withdraw her own pending Request, recording decided_by
// as herself.
func TestWithdrawHandler_RequesterWithdrawsOwnPendingRequest(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/"+requestID+"/withdraw", session, nil)
	var out engagementrequest.DecisionResponse
	decode(t, resp, http.StatusOK, &out)
	if out.State != "withdrawn" {
		t.Fatalf("state = %q, want withdrawn", out.State)
	}

	row := readRequest(t, db, requestID)
	if row.state != "withdrawn" || row.decidedBy == nil || *row.decidedBy != doulaID {
		t.Fatalf("request row = %+v, want withdrawn/decided by %s (herself)", row, doulaID)
	}
}

// TestWithdrawHandler_OnlyRequesterMayWithdraw proves a different Staff
// member -- even an Owner -- cannot withdraw someone else's Request.
func TestWithdrawHandler_OnlyRequesterMayWithdraw(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "owner-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/"+requestID+"/withdraw", session, nil)
	expectStatus(t, resp, http.StatusForbidden)

	row := readRequest(t, db, requestID)
	if row.state != testStatePending {
		t.Fatalf("request state = %q, want unchanged pending", row.state)
	}
}

// TestWithdrawHandler_OnlyPendingMayBeWithdrawn proves a decided Request
// cannot be withdrawn.
func TestWithdrawHandler_OnlyPendingMayBeWithdrawn(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	first := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/"+requestID+"/withdraw", session, nil)
	expectStatus(t, first, http.StatusOK)

	second := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/"+requestID+"/withdraw", session, nil)
	expectStatus(t, second, http.StatusConflict)
}

// TestWithdrawHandler_InvalidRequestIDRejected proves a malformed path
// segment is a 400, not a query against a bogus id.
func TestWithdrawHandler_InvalidRequestIDRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/not-a-uuid/withdraw", session, nil)
	expectStatus(t, resp, http.StatusBadRequest)
}

// TestWithdrawHandler_NotFound proves a bogus request id is a 404.
func TestWithdrawHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/engagement-requests/00000000-0000-0000-0000-000000000000/withdraw", session, nil)
	expectStatus(t, resp, http.StatusNotFound)
}
