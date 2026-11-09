package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// TestRefuseHandler_RequiresReasonAndStampsRequest proves a reason is
// required at the endpoint and, when given, the Request is stamped
// refused with the actor, timestamp, and reason the CHECK constraints
// demand.
func TestRefuseHandler_RequiresReasonAndStampsRequest(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	requestID := e.pending(e.Colleague())
	refuseURL := e.requestURL(requestID, "/refuse")

	reqtest.ExpectStatus(t, http.MethodPost, refuseURL, http.StatusBadRequest, e.SignedIn, reqtest.JSON(engagementrequest.RefuseRequest{}))

	out := reqtest.Expect[engagementrequest.DecisionResponse](t, http.MethodPost, refuseURL, http.StatusOK, e.SignedIn,
		reqtest.JSON(engagementrequest.RefuseRequest{Reason: "already has a doula elsewhere"}))
	if out.State != testStateRefused {
		t.Fatalf("state = %q, want refused", out.State)
	}

	row := readRequest(t, e.DB, requestID)
	if row.state != testStateRefused || row.decidedBy == nil || row.reason == nil || *row.reason == "" {
		t.Fatalf("request row = %+v, want refused/decided/reasoned", row)
	}
}

// TestRefuseHandler_InvalidBodyRejected proves malformed JSON is a 400.
func TestRefuseHandler_InvalidBodyRejected(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	requestID := e.pending(e.Colleague())

	reqtest.ExpectStatus(t, http.MethodPost, e.requestURL(requestID, "/refuse"), http.StatusBadRequest, e.SignedIn, reqtest.JSON("not json"))
}

// TestDatabase_RefusalWithoutReasonRejected proves
// engagement_requests_refusal_reason (00042) rejects a reasonless refusal
// at the database itself, independent of the endpoint's own 400.
func TestDatabase_RefusalWithoutReasonRejected(t *testing.T) {
	e := newEnv(t)
	requestID := e.pending(e.Staff)

	_, err := e.DB.Admin.ExecContext(t.Context(),
		`UPDATE engagement_requests SET state = 'refused', decided_by = $1, decided_at = now() WHERE id = $2`,
		e.Staff.ID, requestID,
	)
	if err == nil {
		t.Fatal("expected the refusal_reason CHECK to reject a reasonless refused row, got no error")
	}
}
