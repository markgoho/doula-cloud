package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// detailURL is the approval screen's read, addressed the way the screen
// addresses it: a Request id and nothing else.
func detailURL(srvURL, practiceID, requestID string) string {
	return srvURL + "/practices/" + practiceID + "/engagement-requests/" + requestID
}

// TestDetailHandler_ReturnsEveryFactTheApprovalScreenShows proves the six
// facts #401's map names arrive in one read for a Client nobody at this
// Practice has worked with before: her identity and new-to-the-Practice
// status, who asked and when, the kind and due date exactly as requested,
// her note, and the Credit cost with the balance it leaves behind.
func TestDetailHandler_ReturnsEveryFactTheApprovalScreenShows(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)
	setRequestNote(t, db, requestID, "She asked for a home birth")

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var out engagementrequest.DetailResponse
	decode(t, get(t, detailURL(srv.URL, practiceID, requestID), session), http.StatusOK, &out)

	if out.Note == nil || *out.Note != "She asked for a home birth" {
		t.Fatalf("note = %v, want the requester's own words", out.Note)
	}
	if out.RequestID != requestID || out.State != testStatePending || out.Kind != testKindBirth {
		t.Fatalf("response = %+v, want the pending birth Request", out)
	}
	if out.DueDate == nil || *out.DueDate != testDueDate {
		t.Fatalf("dueDate = %v, want %s", out.DueDate, testDueDate)
	}
	if out.RequestedBy != doulaID || out.RequestedByName != "Staff doula-1" || out.RequestedAt.IsZero() {
		t.Fatalf("requester = %s/%s at %v, want the seeded Doula", out.RequestedBy, out.RequestedByName, out.RequestedAt)
	}
	if out.Client.ClientID != clientID || !out.Client.IsNewToPractice {
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
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := seedClient(t, db, practiceID)
	seedEngagement(t, db, practiceID, clientID, "completed")
	seedEngagement(t, db, practiceID, clientID, "active")
	requestID := pendingRequest(t, db, practiceID, clientID, testKindPostpartum, doulaID)

	srv, session := newServer(t, db, "owner-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var out engagementrequest.DetailResponse
	decode(t, get(t, detailURL(srv.URL, practiceID, requestID), session), http.StatusOK, &out)

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
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	pendingRequest(t, db, practiceID, clientID, testKindPostpartum, doulaID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var out engagementrequest.DetailResponse
	decode(t, get(t, detailURL(srv.URL, practiceID, requestID), session), http.StatusOK, &out)

	if out.Client.IsNewToPractice {
		t.Fatal("isNewToPractice = true, want false for a Client who was already asked about")
	}
	if out.BalanceAfter != -1 {
		t.Fatalf("balanceAfter = %d, want -1 on an empty balance", out.BalanceAfter)
	}
}

// TestDetailHandler_RefusesADoula proves the approval screen's read is
// Owner/Admin only, like the decision it exists to support.
func TestDetailHandler_RefusesADoula(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	expectStatus(t, get(t, detailURL(srv.URL, practiceID, requestID), session), http.StatusForbidden)
}

// TestDetailHandler_RejectsAMalformedOrUnknownRequestID proves a bad id
// is a 400 and a well-formed id nobody here owns is a 404 -- RLS scopes
// the read, so another Practice's Request is indistinguishable from none.
func TestDetailHandler_RejectsAMalformedOrUnknownRequestID(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	expectStatus(t, get(t, detailURL(srv.URL, practiceID, "not-a-uuid"), session), http.StatusBadRequest)
	expectStatus(t,
		get(t, detailURL(srv.URL, practiceID, "00000000-0000-0000-0000-000000000000"), session),
		http.StatusNotFound)
}

// TestDetailHandler_RefusesADecidedRequest proves a stale link to a
// Request somebody already decided says so, rather than rendering an
// approval screen for a decision that has been made.
func TestDetailHandler_RefusesADecidedRequest(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	srv, session := newServer(t, db, "admin-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	expectStatus(t, do(t, srv.URL+"/practices/"+practiceID+"/engagement-requests/"+requestID+"/approve", session, nil), http.StatusOK)
	expectStatus(t, get(t, detailURL(srv.URL, practiceID, requestID), session), http.StatusConflict)
}
