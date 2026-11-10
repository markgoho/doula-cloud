package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// TestApproveHandler_RefusesARequestWhoseClientWasErased reproduces the
// audit's traced case: erasure leaves a pending Request pending (it
// touches no engagement_requests row), and approval used to turn it into
// a new Engagement, spending a Credit, for a Client whose details were
// already gone. The refusal comes before any write, so the Request stays
// pending, no Engagement exists and no Credit is spent.
func TestApproveHandler_RefusesARequestWhoseClientWasErased(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()), funded)
	requestID := e.pending(e.Colleague())
	fixture.EraseClient(t, e.DB, e.Client.ID)

	got := apierrtest.Expect(t, http.MethodPost, e.requestURL(requestID, "/approve"), http.StatusConflict, e.SignedIn)

	if _, want := clientstanding.ActApproveRequest.Check(clientstanding.Erased); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
	if row := readRequest(t, e.DB, requestID); row.state != testStatePending || row.engagementID != nil {
		t.Errorf("request row = %+v, want still pending with no Engagement", row)
	}
	if got := countRows(t, e.DB, "engagements", e.Practice.ID); got != 0 {
		t.Errorf("engagements = %d, want 0", got)
	}
	if got := creditLedgerCount(t, e.DB, e.Practice.ID); got != 1 {
		t.Errorf("credit_ledger rows = %d, want 1 (the signup grant, nothing spent)", got)
	}
}

// TestRequestHandler_RefusesAnErasedOrMergedClient: the ask is refused in
// words for a record in either standing, before anything is written --
// for an Owner too, whose ask would otherwise collapse straight into an
// approval.
func TestRequestHandler_RefusesAnErasedOrMergedClient(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), funded)
	erased := e.Client
	fixture.EraseClient(t, e.DB, erased.ID)
	merged := fixture.Client(t, e.DB, e.Practice)
	fixture.MergeClient(t, e.DB, merged.ID, fixture.Client(t, e.DB, e.Practice).ID)

	for standing, clientID := range map[clientstanding.Standing]string{clientstanding.Erased: erased.ID, clientstanding.Merged: merged.ID} {
		got := apierrtest.Expect(t, http.MethodPost, e.askURL(clientID), http.StatusConflict, e.SignedIn, reqtest.JSON(birthAsk))
		if _, want := clientstanding.ActRequestEngagement.Check(standing); got.Message != want {
			t.Errorf("%s: message = %q, want %q", standing, got.Message, want)
		}
	}
	if got := countRows(t, e.DB, "engagement_requests", e.Practice.ID); got != 0 {
		t.Errorf("engagement_requests = %d, want 0", got)
	}
}
