package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestRequestHandler_DoulaCreatesPendingRequestAndMailsOwnersAndAdmins
// proves the ordinary path: an employee Doula's request lands pending,
// names her as requester, and queues one outbox row per Owner/Admin.
func TestRequestHandler_DoulaCreatesPendingRequestAndMailsOwnersAndAdmins(t *testing.T) {
	e := newEnv(t)
	e.Colleague(fixture.Owner())
	e.Colleague(fixture.Admin())

	out := e.ask(birthAsk)
	if out.State != testStatePending || out.EngagementID != "" {
		t.Fatalf("response = %+v, want pending with no engagementId", out)
	}

	row := readRequest(t, e.DB, out.RequestID)
	if row.state != testStatePending || row.decidedBy != nil || row.engagementID != nil {
		t.Fatalf("request row = %+v, want pending/undecided", row)
	}
	if got := outboxCount(t, e.DB, out.RequestID); got != 2 {
		t.Fatalf("outbox rows = %d, want 2 (one owner, one admin)", got)
	}
}

// TestRequestHandler_ContractorForbidden proves ADR-0017's "a contractor
// originates nothing" at the endpoint.
func TestRequestHandler_ContractorForbidden(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	e.askRefused(http.StatusForbidden, birthAsk)
}

// TestRequestHandler_DuplicatePendingSameKindRefused proves the
// (client_id, kind) partial unique index is surfaced as a clean 409, and
// that it does not catch the legitimate birth-and-postpartum pair.
func TestRequestHandler_DuplicatePendingSameKindRefused(t *testing.T) {
	postpartumAsk := engagementrequest.RequestBody{Kind: testKindPostpartum, DueDate: testDueDate}
	for _, tc := range []struct {
		name   string
		second engagementrequest.RequestBody
		want   int
	}{
		{"the same kind", birthAsk, http.StatusConflict},
		{"a different kind", postpartumAsk, http.StatusCreated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.ask(birthAsk)
			e.askRefused(tc.want, tc.second)
		})
	}
}

// TestRequestHandler_LiveEngagementWarns proves a second live Engagement
// warns rather than refuses, at request time, on the ordinary path and on
// the solo Owner's collapsed create-and-approve one.
func TestRequestHandler_LiveEngagementWarns(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []envOption
	}{
		{"a Doula's ask", nil},
		{"a solo Owner's collapsed act", []envOption{signedInAs(fixture.Owner()), funded}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, tc.opts...)
			e.AnotherEngagement(fixture.ForClient(e.Client), fixture.Status(fixture.StatusActive))

			out := e.ask(engagementrequest.RequestBody{Kind: testKindPostpartum, DueDate: testDueDate})
			if out.Warning == "" {
				t.Fatal("expected a live-engagement warning, got none")
			}
		})
	}
}

// TestRequestHandler_SoloOwnerCollapsesToApprovedAndMailsNobody proves
// ADR-0017's solo-Practice rule: an Owner's own request is created and
// decided in the same instant, spends a Credit, and queues no outbox row.
func TestRequestHandler_SoloOwnerCollapsesToApprovedAndMailsNobody(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Contractor()), funded)

	out := e.ask(birthAsk)
	if out.State != testStateApproved || out.EngagementID == "" {
		t.Fatalf("response = %+v, want approved with an engagementId", out)
	}

	row := readRequest(t, e.DB, out.RequestID)
	if row.state != testStateApproved || row.decidedBy == nil || row.engagementID == nil {
		t.Fatalf("request row = %+v, want approved/decided", row)
	}
	if got := outboxCount(t, e.DB, out.RequestID); got != 0 {
		t.Fatalf("outbox rows = %d, want 0 (collapsed act mails nobody)", got)
	}
	if got := creditLedgerCount(t, e.DB, e.Practice.ID); got != 2 {
		t.Fatalf("credit_ledger rows = %d, want 2 (signup_bonus + consumption)", got)
	}
	kind, dueDate := engagementRow(t, e.DB, out.EngagementID)
	if kind != testKindBirth || dueDate == nil || *dueDate != testDueDate {
		t.Fatalf("engagement = kind %q due %v, want birth/%s", kind, dueDate, testDueDate)
	}
}

// TestRequestHandler_SoloOwnerNoCreditsLeavesNothingBehind proves the
// collapsed act, on an empty balance, fails whole: no request row, no
// engagement, and the out-of-Credits Notification is queued.
func TestRequestHandler_SoloOwnerNoCreditsLeavesNothingBehind(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	e.askRefused(http.StatusPaymentRequired, birthAsk)

	if got := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM engagement_requests WHERE client_id = $1`, e.Client.ID); got != 0 {
		t.Fatalf("engagement_requests rows = %d, want 0", got)
	}
	if got := lowCreditOutboxCount(t, e.DB, e.Practice.ID); got != 1 {
		t.Fatalf("low_credit_outbox rows = %d, want 1", got)
	}
}

// TestRequestHandler_ClientNotFound proves a client id at a different
// Practice (or nonexistent) is a 404, not a 500.
func TestRequestHandler_ClientNotFound(t *testing.T) {
	e := newEnv(t)
	otherClient := fixture.Client(t, e.DB, fixture.Practice(t, e.DB))

	for _, clientID := range []string{otherClient.ID, zeroUUID} {
		e.askClientRefused(clientID, http.StatusNotFound, birthAsk)
	}
}

// TestRequestHandler_RejectsABadAsk proves the ask is validated before any
// write: a kind that is not one, a path segment that is no id, a body that is
// no JSON, and a due date that is no date are each a 400.
func TestRequestHandler_RejectsABadAsk(t *testing.T) {
	for _, tc := range []struct {
		name     string
		clientID string
		body     any
	}{
		{"an invalid kind", "", engagementrequest.RequestBody{Kind: "adoption", DueDate: testDueDate}},
		{"a malformed client id", notAUUID, birthAsk},
		{"a malformed body", "", "not json"},
		{"a malformed due date", "", engagementrequest.RequestBody{Kind: testKindBirth, DueDate: "not-a-date"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			clientID := tc.clientID
			if clientID == "" {
				clientID = e.Client.ID
			}
			e.askClientRefused(clientID, http.StatusBadRequest, tc.body)
		})
	}
}

// TestRequestHandler_NoDueDateAllowed proves dueDate is optional -- a
// postpartum-only Engagement has none.
func TestRequestHandler_NoDueDateAllowed(t *testing.T) {
	e := newEnv(t)
	e.ask(engagementrequest.RequestBody{Kind: testKindPostpartum})
}
