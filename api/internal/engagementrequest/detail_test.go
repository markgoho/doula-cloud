package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

func (e *env) detail(requestID string) engagementrequest.DetailResponse {
	e.T.Helper()
	return reqtest.Expect[engagementrequest.DetailResponse](e.T, http.MethodGet, e.requestURL(requestID, ""), http.StatusOK, e.SignedIn)
}

// TestDetailHandler_ReturnsEveryFactTheApprovalScreenShows proves the six
// facts #401's map names arrive in one read for a Client nobody at this
// Practice has worked with before: her identity and new-to-the-Practice
// status, who asked and when, the kind and due date exactly as requested,
// her note, and the Credit cost with the balance it leaves behind.
func TestDetailHandler_ReturnsEveryFactTheApprovalScreenShows(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()), funded)
	doula := e.Colleague()
	requestID := e.pending(doula)
	e.setRequestNote(requestID, "She asked for a home birth")

	out := e.detail(requestID)

	if out.Note == nil || *out.Note != "She asked for a home birth" {
		t.Fatalf("note = %v, want the requester's own words", out.Note)
	}
	if out.RequestID != requestID || out.State != testStatePending || out.Kind != testKindBirth {
		t.Fatalf("response = %+v, want the pending birth Request", out)
	}
	if out.DueDate == nil || *out.DueDate != testDueDate {
		t.Fatalf("dueDate = %v, want %s", out.DueDate, testDueDate)
	}
	if out.RequestedBy != doula.ID || out.RequestedByName != doula.Name || out.RequestedAt.IsZero() {
		t.Fatalf("requester = %s/%s at %v, want the seeded Doula", out.RequestedBy, out.RequestedByName, out.RequestedAt)
	}
	if out.Client.ClientID != e.Client.ID || !out.Client.IsNewToPractice {
		t.Fatalf("client = %+v, want the seeded Client, new to the Practice", out.Client)
	}
	if out.CreditCost != 1 || out.Balance != 3 || out.BalanceAfter != 2 {
		t.Fatalf("cost/balance = %d/%d/%d, want 1/3/2", out.CreditCost, out.Balance, out.BalanceAfter)
	}
	if len(out.Engagements) != 0 || out.Warning != "" {
		t.Fatalf("engagements/warning = %+v/%q, want none for a Client with no history", out.Engagements, out.Warning)
	}
}

// TestDetailHandler_KnownClientCarriesHerEngagementsAndTheLiveWarning
// proves a Client the Practice already works with is not reported as new,
// that her Engagements past and present travel with the read, and that
// ADR-0017's second-live-Engagement warning reaches the approver's seat.
func TestDetailHandler_KnownClientCarriesHerEngagementsAndTheLiveWarning(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), funded)
	e.AnotherEngagement(fixture.ForClient(e.Client), fixture.Status(fixture.StatusCompleted))
	e.AnotherEngagement(fixture.ForClient(e.Client), fixture.Status(fixture.StatusActive))
	requestID := e.pendingFor(e.Client, testKindPostpartum, e.Colleague())

	out := e.detail(requestID)

	if out.Client.IsNewToPractice {
		t.Fatal("isNewToPractice = true, want false for a Client with Engagements here")
	}
	if len(out.Engagements) != 2 {
		t.Fatalf("engagements = %d, want both the completed and the live one", len(out.Engagements))
	}
	if out.Warning == "" {
		t.Fatal("warning = empty, want the second-live-Engagement warning")
	}
}

// TestDetailHandler_AnEarlierRequestAloneMakesHerKnown proves the
// new-or-known split reads Requests as well as Engagements, and never
// counts the Request being decided against itself.
func TestDetailHandler_AnEarlierRequestAloneMakesHerKnown(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	doula := e.Colleague()
	e.pendingFor(e.Client, testKindPostpartum, doula)
	requestID := e.pending(doula)

	out := e.detail(requestID)

	if out.Client.IsNewToPractice {
		t.Fatal("isNewToPractice = true, want false for a Client who was already asked about")
	}
	if out.BalanceAfter != -1 {
		t.Fatalf("balanceAfter = %d, want -1 on an empty balance", out.BalanceAfter)
	}
}
