package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// TestRequestHandler_RefusesAPersonWhoCannotBeNamed is each refusal the
// write gives an Owner, who may name a colleague: a contractor, a person
// with no Doula role, a person who is not Staff at this Practice, and a
// value that is no staff id at all. Each names the doulaStaffId field
// (#488) with the sentence for its own reason, and writes no Request.
func TestRequestHandler_RefusesAPersonWhoCannotBeNamed(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "owner-1", []string{ownerRole, doulaRole}, employeeType)
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, "contractor-1")
	adminID := testdb.SeedStaffAtPractice(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	strangerID := testdb.SeedStaffAtPractice(t, db, otherPracticeID, "stranger-1", []string{doulaRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")

	srv, session := newServer(t, db, "owner-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	cases := []struct {
		name   string
		doula  string
		detail string
	}{
		{"a contractor Doula", contractorID, engagementrequest.MsgDoulaIsContractor},
		{"a person with no Doula role", adminID, engagementrequest.MsgDoulaNotADoula},
		{"a Doula at another Practice", strangerID, engagementrequest.MsgDoulaNotAtPractice},
		{"a person who does not exist", unknownStaffID, engagementrequest.MsgDoulaNotAtPractice},
		{"a value that is not a staff id", "not-a-uuid", engagementrequest.MsgDoulaMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeRefusal(t, do(t, requestsURL(srv.URL, practiceID, clientID), session,
				engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &tc.doula}),
				http.StatusBadRequest)
			if got.Code != "INVALID_ARGUMENT" || got.Details["doulaStaffId"] != tc.detail {
				t.Fatalf("refusal = %+v, want INVALID_ARGUMENT naming doulaStaffId with %q", got, tc.detail)
			}
		})
	}
	if got := countRows(t, db, "engagement_requests", practiceID); got != 0 {
		t.Fatalf("engagement_requests rows = %d, want 0: a refused ask writes nothing", got)
	}
	if got := countRows(t, db, "engagements", practiceID); got != 0 {
		t.Fatalf("engagements rows = %d, want 0: a refused ask starts no work", got)
	}
}

// TestRequestHandler_AnOwnerWithNoDoulaRoleCannotNameHerself proves the
// rule is about the person named and not about who asks: Attachment is
// for Doulas only (CONTEXT.md), so an Owner who does not hold the Doula
// role is refused her own name.
func TestRequestHandler_AnOwnerWithNoDoulaRoleCannotNameHerself(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	seedCredits(t, db, practiceID)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")

	srv, session := newServer(t, db, "owner-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	got := decodeRefusal(t, do(t, requestsURL(srv.URL, practiceID, clientID), session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &ownerID}),
		http.StatusBadRequest)
	if got.Details["doulaStaffId"] != engagementrequest.MsgDoulaNotADoula {
		t.Fatalf("refusal = %+v, want the no-Doula-role sentence", got)
	}
}

// TestRequestHandler_APlainDoulaNamesNobodyButHerself proves ADR-0008's
// half of the rule: a person with no approval authority may name herself
// and nobody else. The refusal is the same 403 whoever she named -- a
// colleague who could be named, a contractor, a person who does not
// exist -- because a refusal that changed with the person would be a
// read of the roster she is not given.
func TestRequestHandler_APlainDoulaNamesNobodyButHerself(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	colleagueID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-2", []string{doulaRole}, employeeType)
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, "contractor-1")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()

	var first refusal
	for i, named := range []string{colleagueID, contractorID, unknownStaffID} {
		got := decodeRefusal(t, do(t, requestsURL(srv.URL, practiceID, clientID), session,
			engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &named}),
			http.StatusForbidden)
		if got.Code != "FORBIDDEN" || got.Details["doulaStaffId"] != engagementrequest.MsgDoulaNotHerself {
			t.Fatalf("refusal = %+v, want FORBIDDEN naming doulaStaffId", got)
		}
		if i == 0 {
			first = got
		}
		if got.Message != first.Message {
			t.Fatalf("refusal message = %q, want %q for every person named", got.Message, first.Message)
		}
	}
	if got := countRows(t, db, "engagement_requests", practiceID); got != 0 {
		t.Fatalf("engagement_requests rows = %d, want 0", got)
	}
}
