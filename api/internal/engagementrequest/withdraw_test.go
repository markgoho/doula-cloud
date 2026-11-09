package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// TestWithdrawHandler_RequesterWithdrawsOwnPendingRequest proves the
// requester may withdraw her own pending Request, recording decided_by
// as herself.
func TestWithdrawHandler_RequesterWithdrawsOwnPendingRequest(t *testing.T) {
	e := newEnv(t)
	requestID := e.pending(e.Staff)

	out := reqtest.Expect[engagementrequest.DecisionResponse](t, http.MethodPost, e.requestURL(requestID, "/withdraw"), http.StatusOK, e.SignedIn)
	if out.State != testStateWithdrawn {
		t.Fatalf("state = %q, want withdrawn", out.State)
	}

	row := readRequest(t, e.DB, requestID)
	if row.state != testStateWithdrawn || row.decidedBy == nil || *row.decidedBy != e.Staff.ID {
		t.Fatalf("request row = %+v, want withdrawn/decided by %s (herself)", row, e.Staff.ID)
	}
}

// TestWithdrawHandler_OnlyRequesterMayWithdraw proves a different Staff
// member -- even an Owner -- cannot withdraw someone else's Request.
func TestWithdrawHandler_OnlyRequesterMayWithdraw(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	requestID := e.pending(e.Colleague())

	reqtest.ExpectStatus(t, http.MethodPost, e.requestURL(requestID, "/withdraw"), http.StatusForbidden, e.SignedIn)

	if row := readRequest(t, e.DB, requestID); row.state != testStatePending {
		t.Fatalf("request state = %q, want unchanged pending", row.state)
	}
}
