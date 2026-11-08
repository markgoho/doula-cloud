package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// TestRefuseHandler_RequiresReasonAndStampsRequest proves a reason is
// required at the endpoint and, when given, the Request is stamped
// refused with the actor, timestamp, and reason the CHECK constraints
// demand.
func TestRefuseHandler_RequiresReasonAndStampsRequest(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	admin := fixture.Staff(t, db, p, fixture.Admin())
	client := fixture.Client(t, db, p)
	requestID := pendingRequest(t, db, p, client, testKindBirth, doula)

	srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	noReason := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/"+requestID+"/refuse", reqtest.Session(session), reqtest.JSON(engagementrequest.RefuseRequest{}))
	reqtest.RequireStatus(t, noReason, http.StatusBadRequest)

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/"+requestID+"/refuse", reqtest.Session(session), reqtest.JSON(engagementrequest.RefuseRequest{Reason: "already has a doula elsewhere"}))
	out := reqtest.DecodeStatus[engagementrequest.DecisionResponse](t, resp, http.StatusOK)
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
	p := fixture.Practice(t, db)
	admin := fixture.Staff(t, db, p, fixture.Admin())

	srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/not-a-uuid/refuse", reqtest.Session(session), reqtest.JSON(engagementrequest.RefuseRequest{Reason: "no"}))
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
}

// TestRefuseHandler_InvalidBodyRejected proves malformed JSON is a 400.
func TestRefuseHandler_InvalidBodyRejected(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	admin := fixture.Staff(t, db, p, fixture.Admin())
	client := fixture.Client(t, db, p)
	requestID := pendingRequest(t, db, p, client, testKindBirth, doula)

	srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/"+requestID+"/refuse", reqtest.Session(session), reqtest.JSON("not json"))
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
}

// TestRefuseHandler_NotFound proves a bogus request id is a 404.
func TestRefuseHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	admin := fixture.Staff(t, db, p, fixture.Admin())

	srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/00000000-0000-0000-0000-000000000000/refuse", reqtest.Session(session), reqtest.JSON(engagementrequest.RefuseRequest{Reason: "no"}))
	reqtest.RequireStatus(t, resp, http.StatusNotFound)
}

// TestDatabase_RefusalWithoutReasonRejected proves
// engagement_requests_refusal_reason (00042) rejects a reasonless refusal
// at the database itself, independent of the endpoint's own 400.
func TestDatabase_RefusalWithoutReasonRejected(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	client := fixture.Client(t, db, p)
	requestID := pendingRequest(t, db, p, client, testKindBirth, doula)

	_, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_requests SET state = 'refused', decided_by = $1, decided_at = now() WHERE id = $2`,
		doula.ID, requestID,
	)
	if err == nil {
		t.Fatal("expected the refusal_reason CHECK to reject a reasonless refused row, got no error")
	}
}

// TestRefuseHandler_DoulaForbidden proves refuse is Owner/Admin only.
func TestRefuseHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	client := fixture.Client(t, db, p)
	requestID := pendingRequest(t, db, p, client, testKindBirth, doula)

	srv, session := newServer(t, db, doula, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/"+requestID+"/refuse", reqtest.Session(session), reqtest.JSON(engagementrequest.RefuseRequest{Reason: "no"}))
	reqtest.RequireStatus(t, resp, http.StatusForbidden)
}

// TestRefuseHandler_AlreadyDecidedConflict proves refusing an already
// decided Request is a 409.
func TestRefuseHandler_AlreadyDecidedConflict(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	admin := fixture.Staff(t, db, p, fixture.Admin())
	client := fixture.Client(t, db, p)
	requestID := pendingRequest(t, db, p, client, testKindBirth, doula)

	srv, session := newServer(t, db, admin, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	first := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/"+requestID+"/refuse", reqtest.Session(session), reqtest.JSON(engagementrequest.RefuseRequest{Reason: "no"}))
	reqtest.RequireStatus(t, first, http.StatusOK)

	second := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/"+requestID+"/refuse", reqtest.Session(session), reqtest.JSON(engagementrequest.RefuseRequest{Reason: "no"}))
	reqtest.RequireStatus(t, second, http.StatusConflict)
}
