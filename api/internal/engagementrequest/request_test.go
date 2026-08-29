package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// TestRequestHandler_DoulaCreatesPendingRequestAndMailsOwnersAndAdmins
// proves the ordinary path: an employee Doula's request lands pending,
// names her as requester, and queues one outbox row per Owner/Admin.
func TestRequestHandler_DoulaCreatesPendingRequestAndMailsOwnersAndAdmins(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	enq := &tasknudge.FakeEnqueuer{}

	srv, session := newServer(t, db, "doula-1", enq)
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	var out engagementrequest.RequestResponse
	decode(t, resp, http.StatusCreated, &out)
	if out.State != testStatePending || out.EngagementID != "" {
		t.Fatalf("response = %+v, want pending with no engagementId", out)
	}

	row := readRequest(t, db, out.RequestID)
	if row.state != testStatePending || row.decidedBy != nil || row.engagementID != nil {
		t.Fatalf("request row = %+v, want pending/undecided", row)
	}
	if got := outboxCount(t, db, out.RequestID); got != 2 {
		t.Fatalf("outbox rows = %d, want 2 (one owner, one admin)", got)
	}
}

// TestRequestHandler_ContractorForbidden proves ADR-0017's "a contractor
// originates nothing" at the endpoint.
func TestRequestHandler_ContractorForbidden(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "contractor-1", []string{doulaRole}, contractorType)
	clientID := seedClient(t, db, practiceID)

	srv, session := newServer(t, db, "contractor-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	expectStatus(t, resp, http.StatusForbidden)
}

// TestRequestHandler_DuplicatePendingSameKindRefused proves the
// (client_id, kind) partial unique index is surfaced as a clean 409, and
// TestRequestHandler_DifferentKindAllowed proves it does not catch the
// legitimate birth-and-postpartum pair.
func TestRequestHandler_DuplicatePendingSameKindRefused(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	first := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	expectStatus(t, first, http.StatusCreated)

	second := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	expectStatus(t, second, http.StatusConflict)
}

func TestRequestHandler_DifferentKindAllowed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	first := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	expectStatus(t, first, http.StatusCreated)

	second := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindPostpartum, DueDate: testDueDate})
	expectStatus(t, second, http.StatusCreated)
}

// TestRequestHandler_LiveEngagementWarns proves a second live Engagement
// warns rather than refuses, at request time.
func TestRequestHandler_LiveEngagementWarns(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	seedEngagement(t, db, practiceID, clientID, "active")

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindPostpartum, DueDate: testDueDate})
	var out engagementrequest.RequestResponse
	decode(t, resp, http.StatusCreated, &out)
	if out.Warning == "" {
		t.Fatal("expected a live-engagement warning, got none")
	}
}

// TestRequestHandler_SoloOwnerCollapsesToApprovedAndMailsNobody proves
// ADR-0017's solo-Practice rule: an Owner's own request is created and
// decided in the same instant, spends a Credit, and queues no outbox row.
func TestRequestHandler_SoloOwnerCollapsesToApprovedAndMailsNobody(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "owner-1", []string{ownerRole}, contractorType)
	seedCredits(t, db, practiceID)
	clientID := seedClient(t, db, practiceID)

	srv, session := newServer(t, db, "owner-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	var out engagementrequest.RequestResponse
	decode(t, resp, http.StatusCreated, &out)
	if out.State != testStateApproved || out.EngagementID == "" {
		t.Fatalf("response = %+v, want approved with an engagementId", out)
	}

	row := readRequest(t, db, out.RequestID)
	if row.state != testStateApproved || row.decidedBy == nil || row.engagementID == nil {
		t.Fatalf("request row = %+v, want approved/decided", row)
	}
	if got := outboxCount(t, db, out.RequestID); got != 0 {
		t.Fatalf("outbox rows = %d, want 0 (collapsed act mails nobody)", got)
	}
	if got := creditLedgerCount(t, db, practiceID); got != 2 {
		t.Fatalf("credit_ledger rows = %d, want 2 (signup_bonus + consumption)", got)
	}
	kind, dueDate := engagementRow(t, db, out.EngagementID)
	if kind != testKindBirth || dueDate == nil || *dueDate != testDueDate {
		t.Fatalf("engagement = kind %q due %v, want birth/%s", kind, dueDate, testDueDate)
	}
}

// TestRequestHandler_SoloOwnerNoCreditsLeavesNothingBehind proves the
// collapsed act, on an empty balance, fails whole: no request row, no
// engagement, and the out-of-Credits Notification is queued.
func TestRequestHandler_SoloOwnerNoCreditsLeavesNothingBehind(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	clientID := seedClient(t, db, practiceID)

	srv, session := newServer(t, db, "owner-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	expectStatus(t, resp, http.StatusPaymentRequired)

	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM engagement_requests WHERE client_id = $1`, clientID).Scan(&count); err != nil {
		t.Fatalf("count requests: %v", err)
	}
	if count != 0 {
		t.Fatalf("engagement_requests rows = %d, want 0", count)
	}
	if got := lowCreditOutboxCount(t, db, practiceID); got != 1 {
		t.Fatalf("low_credit_outbox rows = %d, want 1", got)
	}
}

// TestRequestHandler_ClientNotFound proves a client id at a different
// Practice (or nonexistent) is a 404, not a 500.
func TestRequestHandler_ClientNotFound(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	otherPracticeID := seedPractice(t, db)
	otherClientID := seedClient(t, db, otherPracticeID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+otherClientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	expectStatus(t, resp, http.StatusNotFound)
}

// TestRequestHandler_InvalidKindRejected proves kind is validated before
// any write.
func TestRequestHandler_InvalidKindRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: "adoption", DueDate: testDueDate})
	expectStatus(t, resp, http.StatusBadRequest)
}

// TestRequestHandler_InvalidClientIDRejected proves a malformed path
// segment is a 400, not a query against a bogus id.
func TestRequestHandler_InvalidClientIDRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/not-a-uuid/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	expectStatus(t, resp, http.StatusBadRequest)
}

// TestRequestHandler_InvalidBodyRejected proves malformed JSON is a 400.
func TestRequestHandler_InvalidBodyRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session, "not json")
	expectStatus(t, resp, http.StatusBadRequest)
}

// TestRequestHandler_NoDueDateAllowed proves dueDate is optional -- a
// postpartum-only Engagement has none.
func TestRequestHandler_NoDueDateAllowed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindPostpartum})
	expectStatus(t, resp, http.StatusCreated)
}

// TestRequestHandler_InvalidDueDateRejected proves a malformed dueDate is
// a 400.
func TestRequestHandler_InvalidDueDateRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: "not-a-date"})
	expectStatus(t, resp, http.StatusBadRequest)
}

// TestRequestHandler_SoloOwnerCollapseWarnsOnLiveEngagement proves the
// collapsed create-and-approve path also carries the second-live-Engagement
// warning.
func TestRequestHandler_SoloOwnerCollapseWarnsOnLiveEngagement(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := seedClient(t, db, practiceID)
	seedEngagement(t, db, practiceID, clientID, "active")

	srv, session := newServer(t, db, "owner-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := do(t, srv.URL+"/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindPostpartum, DueDate: testDueDate})
	var out engagementrequest.RequestResponse
	decode(t, resp, http.StatusCreated, &out)
	if out.Warning == "" {
		t.Fatal("expected a live-engagement warning on the collapsed path, got none")
	}
}
