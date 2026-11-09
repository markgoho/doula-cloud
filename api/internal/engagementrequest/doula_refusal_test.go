package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// TestRequestHandler_RefusesAPersonWhoCannotBeNamed is each refusal the
// write gives an Owner, who may name a colleague: a contractor, a person
// with no Doula role, a person who is not Staff at this Practice, and a
// value that is no staff id at all. Each names the doulaStaffId field
// (#488) with the sentence for its own reason, and writes no Request.
func TestRequestHandler_RefusesAPersonWhoCannotBeNamed(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula()), funded)
	contractor := e.Colleague(fixture.Contractor())
	admin := e.Colleague(fixture.Admin())
	stranger := fixture.Staff(t, e.DB, fixture.Practice(t, e.DB, fixture.PracticeName("Other Practice")))

	for _, tc := range []struct {
		name   string
		doula  string
		detail string
	}{
		{"a contractor Doula", contractor.ID, engagementrequest.MsgDoulaIsContractor},
		{"a person with no Doula role", admin.ID, engagementrequest.MsgDoulaNotADoula},
		{"a Doula at another Practice", stranger.ID, engagementrequest.MsgDoulaNotAtPractice},
		{"a person who does not exist", unknownStaffID, engagementrequest.MsgDoulaNotAtPractice},
		{"a value that is not a staff id", notAUUID, engagementrequest.MsgDoulaMalformed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := reqtest.Expect[apierr.APIError](t, http.MethodPost, e.askURL(e.Client.ID), http.StatusBadRequest, e.SignedIn,
				reqtest.JSON(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &tc.doula}))
			if got.Code != "INVALID_ARGUMENT" || got.Details["doulaStaffId"] != tc.detail {
				t.Fatalf("refusal = %+v, want INVALID_ARGUMENT naming doulaStaffId with %q", got, tc.detail)
			}
		})
	}
	if got := countRows(t, e.DB, "engagement_requests", e.Practice.ID); got != 0 {
		t.Fatalf("engagement_requests rows = %d, want 0: a refused ask writes nothing", got)
	}
	if got := countRows(t, e.DB, "engagements", e.Practice.ID); got != 0 {
		t.Fatalf("engagements rows = %d, want 0: a refused ask starts no work", got)
	}
}

// TestRequestHandler_AnOwnerWithNoDoulaRoleCannotNameHerself proves the
// rule is about the person named and not about who asks: Attachment is
// for Doulas only (GLOSSARY.md), so an Owner who does not hold the Doula
// role is refused her own name.
func TestRequestHandler_AnOwnerWithNoDoulaRoleCannotNameHerself(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), funded)

	got := reqtest.Expect[apierr.APIError](t, http.MethodPost, e.askURL(e.Client.ID), http.StatusBadRequest, e.SignedIn,
		reqtest.JSON(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &e.Staff.ID}))
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
	e := newEnv(t)
	colleague := e.Colleague()
	contractor := e.Colleague(fixture.Contractor())

	var first apierr.APIError
	for i, named := range []string{colleague.ID, contractor.ID, unknownStaffID} {
		got := reqtest.Expect[apierr.APIError](t, http.MethodPost, e.askURL(e.Client.ID), http.StatusForbidden, e.SignedIn,
			reqtest.JSON(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate, DoulaStaffID: &named}))
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
	if got := countRows(t, e.DB, "engagement_requests", e.Practice.ID); got != 0 {
		t.Fatalf("engagement_requests rows = %d, want 0", got)
	}
}
