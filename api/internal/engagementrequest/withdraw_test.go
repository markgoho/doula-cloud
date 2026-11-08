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

// TestWithdrawHandler_RequesterWithdrawsOwnPendingRequest proves the
// requester may withdraw her own pending Request, recording decided_by
// as herself.
func TestWithdrawHandler_RequesterWithdrawsOwnPendingRequest(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	client := fixture.Client(t, db, p)
	requestID := pendingRequest(t, db, p, client, testKindBirth, doula)

	srv, session := newServer(t, db, doula, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/"+requestID+"/withdraw", reqtest.Session(session))
	out := reqtest.DecodeStatus[engagementrequest.DecisionResponse](t, resp, http.StatusOK)
	if out.State != "withdrawn" {
		t.Fatalf("state = %q, want withdrawn", out.State)
	}

	row := readRequest(t, db, requestID)
	if row.state != "withdrawn" || row.decidedBy == nil || *row.decidedBy != doula.ID {
		t.Fatalf("request row = %+v, want withdrawn/decided by %s (herself)", row, doula.ID)
	}
}

// TestWithdrawHandler_OnlyRequesterMayWithdraw proves a different Staff
// member -- even an Owner -- cannot withdraw someone else's Request.
func TestWithdrawHandler_OnlyRequesterMayWithdraw(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	owner := fixture.Staff(t, db, p, fixture.Owner())
	client := fixture.Client(t, db, p)
	requestID := pendingRequest(t, db, p, client, testKindBirth, doula)

	srv, session := newServer(t, db, owner, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/"+requestID+"/withdraw", reqtest.Session(session))
	reqtest.RequireStatus(t, resp, http.StatusForbidden)

	row := readRequest(t, db, requestID)
	if row.state != testStatePending {
		t.Fatalf("request state = %q, want unchanged pending", row.state)
	}
}

// TestWithdrawHandler_OnlyPendingMayBeWithdrawn proves a decided Request
// cannot be withdrawn.
func TestWithdrawHandler_OnlyPendingMayBeWithdrawn(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	client := fixture.Client(t, db, p)
	requestID := pendingRequest(t, db, p, client, testKindBirth, doula)

	srv, session := newServer(t, db, doula, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	first := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/"+requestID+"/withdraw", reqtest.Session(session))
	reqtest.RequireStatus(t, first, http.StatusOK)

	second := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/"+requestID+"/withdraw", reqtest.Session(session))
	reqtest.RequireStatus(t, second, http.StatusConflict)
}

// TestWithdrawHandler_InvalidRequestIDRejected proves a malformed path
// segment is a 400, not a query against a bogus id.
func TestWithdrawHandler_InvalidRequestIDRejected(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)

	srv, session := newServer(t, db, doula, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/not-a-uuid/withdraw", reqtest.Session(session))
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
}

// TestWithdrawHandler_NotFound proves a bogus request id is a 404.
func TestWithdrawHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)

	srv, session := newServer(t, db, doula, &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	resp := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/engagement-requests/00000000-0000-0000-0000-000000000000/withdraw", reqtest.Session(session))
	reqtest.RequireStatus(t, resp, http.StatusNotFound)
}
