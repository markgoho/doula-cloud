package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// TestApproveHandler_CreatesEngagementConsumesCreditAndStampsRequest
// proves ADR-0017's core act: approval is the only path that creates an
// engagements row, its kind and due date equal the Request's, a Credit is
// consumed, and the Request is stamped with the state/actor/timestamp the
// CHECK constraints demand.
func TestApproveHandler_CreatesEngagementConsumesCreditAndStampsRequest(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/engagement-requests/"+requestID+"/approve", session, nil)
	var out engagementrequest.ApproveResponse
	decode(t, resp, http.StatusOK, &out)
	if out.EngagementID == "" || out.State != testStateApproved {
		t.Fatalf("response = %+v, want approved with an engagementId", out)
	}

	row := readRequest(t, db, requestID)
	if row.state != testStateApproved || row.decidedBy == nil || row.engagementID == nil {
		t.Fatalf("request row = %+v, want approved/decided/linked", row)
	}
	kind, dueDate := engagementRow(t, db, out.EngagementID)
	if kind != testKindBirth || dueDate == nil || *dueDate != testDueDate {
		t.Fatalf("engagement = kind %q due %v, want birth/%s", kind, dueDate, testDueDate)
	}
	if got := creditLedgerCount(t, db, practiceID); got != 2 {
		t.Fatalf("credit_ledger rows = %d, want 2 (signup_bonus + consumption)", got)
	}
}

// TestApproveHandler_EmptyBalanceLeavesRequestPendingAndQueuesNotification
// proves approval into an empty balance fails without consuming the
// Request: no Engagement is created, no Credit is touched, the Request
// stays pending, and the out-of-Credits Notification is queued.
func TestApproveHandler_EmptyBalanceLeavesRequestPendingAndQueuesNotification(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/engagement-requests/"+requestID+"/approve", session, nil)
	expectStatus(t, resp, http.StatusPaymentRequired)

	row := readRequest(t, db, requestID)
	if row.state != testStatePending || row.decidedBy != nil || row.engagementID != nil {
		t.Fatalf("request row = %+v, want still pending/undecided", row)
	}
	if got := creditLedgerCount(t, db, practiceID); got != 0 {
		t.Fatalf("credit_ledger rows = %d, want 0", got)
	}
	if got := lowCreditOutboxCount(t, db, practiceID); got != 1 {
		t.Fatalf("low_credit_outbox rows = %d, want 1", got)
	}
}

// TestApproveHandler_DoulaForbidden proves approve is Owner/Admin only.
func TestApproveHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/engagement-requests/"+requestID+"/approve", session, nil)
	expectStatus(t, resp, http.StatusForbidden)
}

// TestApproveHandler_AlreadyDecidedConflict proves a second approval of
// the same Request is a 409, not a second Engagement.
func TestApproveHandler_AlreadyDecidedConflict(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	first := do(t, srv.URL+"/practices/"+practiceID+"/engagement-requests/"+requestID+"/approve", session, nil)
	expectStatus(t, first, http.StatusOK)

	second := do(t, srv.URL+"/practices/"+practiceID+"/engagement-requests/"+requestID+"/approve", session, nil)
	expectStatus(t, second, http.StatusConflict)
}

// TestApproveHandler_NotFound proves a bogus request id is a 404.
func TestApproveHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/engagement-requests/00000000-0000-0000-0000-000000000000/approve", session, nil)
	expectStatus(t, resp, http.StatusNotFound)
}

// TestApproveHandler_InvalidRequestIDRejected proves a malformed path
// segment is a 400, not a query against a bogus id.
func TestApproveHandler_InvalidRequestIDRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/engagement-requests/not-a-uuid/approve", session, nil)
	expectStatus(t, resp, http.StatusBadRequest)
}

// TestApproveHandler_WarnsOnLiveEngagement proves the second-live-Engagement
// warning also appears at approval time.
func TestApproveHandler_WarnsOnLiveEngagement(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := seedClient(t, db, practiceID)
	seedEngagement(t, db, practiceID, clientID, "active")
	requestID := pendingRequest(t, db, practiceID, clientID, testKindPostpartum, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/engagement-requests/"+requestID+"/approve", session, nil)
	var out engagementrequest.ApproveResponse
	decode(t, resp, http.StatusOK, &out)
	if out.Warning == "" {
		t.Fatal("expected a live-engagement warning, got none")
	}
}
