package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// TestApproveHandler_CreatesEngagementConsumesCreditAndStampsRequest
// proves ADR-0017's core act: approval is the only path that creates an
// engagements row, its kind and due date equal the Request's, a Credit is
// consumed, and the Request is stamped with the state/actor/timestamp the
// CHECK constraints demand.
func TestApproveHandler_CreatesEngagementConsumesCreditAndStampsRequest(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()), funded)
	requestID := e.pending(e.Colleague())

	out := e.approve(requestID)
	if out.EngagementID == "" || out.State != testStateApproved {
		t.Fatalf("response = %+v, want approved with an engagementId", out)
	}

	row := readRequest(t, e.DB, requestID)
	if row.state != testStateApproved || row.decidedBy == nil || row.engagementID == nil {
		t.Fatalf("request row = %+v, want approved/decided/linked", row)
	}
	kind, dueDate := engagementRow(t, e.DB, out.EngagementID)
	if kind != testKindBirth || dueDate == nil || *dueDate != testDueDate {
		t.Fatalf("engagement = kind %q due %v, want birth/%s", kind, dueDate, testDueDate)
	}
	if got := creditLedgerCount(t, e.DB, e.Practice.ID); got != 2 {
		t.Fatalf("credit_ledger rows = %d, want 2 (signup_bonus + consumption)", got)
	}
}

// TestApproveHandler_EmptyBalanceLeavesRequestPendingAndQueuesNotification
// proves approval into an empty balance fails without consuming the
// Request: no Engagement is created, no Credit is touched, the Request
// stays pending, and the out-of-Credits Notification is queued.
func TestApproveHandler_EmptyBalanceLeavesRequestPendingAndQueuesNotification(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	requestID := e.pending(e.Colleague())

	reqtest.ExpectStatus(t, http.MethodPost, e.requestURL(requestID, "/approve"), http.StatusPaymentRequired, e.SignedIn)

	row := readRequest(t, e.DB, requestID)
	if row.state != testStatePending || row.decidedBy != nil || row.engagementID != nil {
		t.Fatalf("request row = %+v, want still pending/undecided", row)
	}
	if got := creditLedgerCount(t, e.DB, e.Practice.ID); got != 0 {
		t.Fatalf("credit_ledger rows = %d, want 0", got)
	}
	if got := lowCreditOutboxCount(t, e.DB, e.Practice.ID); got != 1 {
		t.Fatalf("low_credit_outbox rows = %d, want 1", got)
	}
}

// TestApproveHandler_WarnsOnLiveEngagement proves the second-live-Engagement
// warning also appears at approval time.
func TestApproveHandler_WarnsOnLiveEngagement(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()), funded)
	e.AnotherEngagement(fixture.ForClient(e.Client), fixture.Status(fixture.StatusActive))
	requestID := e.pendingFor(e.Client, testKindPostpartum, e.Colleague())

	if out := e.approve(requestID); out.Warning == "" {
		t.Fatal("expected a live-engagement warning, got none")
	}
}
