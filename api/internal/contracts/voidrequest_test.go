package contracts_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const noSuchRequestID = "00000000-0000-0000-0000-000000000000"

// reasoned is the body of a void request or a decline.
func reasoned(reason string) reqtest.Option {
	return reqtest.JSON(map[string]string{"reason": reason})
}

func (e *env) voidRequestURL() string {
	return e.contractURL("/void-request")
}

func (e *env) declineURL(requestID string) string {
	return e.contractURL("/void-request/" + requestID + "/decline")
}

// seedVoidRequestBy seeds an open void request on the Engagement's
// Contract, asked for by who.
func (e *env) seedVoidRequestBy(who fixture.StaffRecord, reason string) (requestID string) {
	e.t.Helper()
	return seedVoidRequest(e.t, e.db, e.Engagement.ID, who.ID, reason)
}

// requestVoid sends a void request for a reason and returns the Contract
// it answers with.
func (e *env) requestVoid(reason string, want int) contracts.ContractResponse {
	e.t.Helper()
	return reqtest.Expect[contracts.ContractResponse](e.t, http.MethodPost, e.voidRequestURL(), want, e.signedIn, reasoned(reason))
}

// decline declines the request with a reason and returns the Contract it
// answers with.
func (e *env) decline(requestID, reason string, want int) contracts.ContractResponse {
	e.t.Helper()
	return reqtest.Expect[contracts.ContractResponse](e.t, http.MethodPost, e.declineURL(requestID), want, e.signedIn, reasoned(reason))
}

// TestPostVoidRequestHandler_RequiresSignedContract proves a void may
// only be requested for a Signed Contract -- the same precondition
// PostVoidContractHandler's own transition carries, checked here without
// registering a Transition since this route never writes contracts.
func TestPostVoidRequestHandler_RequiresSignedContract(t *testing.T) {
	for _, status := range []string{statusDraft, statusSent, statusVoided} {
		t.Run(status, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()))
			e.seedContract(status)

			reqtest.ExpectStatus(t, http.MethodPost, e.voidRequestURL(), http.StatusConflict, e.signedIn, reasoned("the client's plan changed"))
		})
	}
}

// TestPostVoidRequestHandler_Refusals proves what a void request is
// refused for: an Engagement it cannot reach or that has no Contract at
// all (404, the same refusal PostVoidContractHandler's own NoContract
// test carries), an undecodable body (400, through apierr.DecodeJSON's
// own refusal, same as every other Contract write in this package), and
// an empty or whitespace-only reason (a field-targeted 400, mirroring
// engagement/fielderrors.go's own "Enter ..." wording convention).
func TestPostVoidRequestHandler_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		seeded bool
		body   reqtest.Option
		want   int
	}{
		{"no contract", false, reasoned("asking"), http.StatusNotFound},
		{"body is not JSON", true, reqtest.RawJSON([]byte("not json")), http.StatusBadRequest},
		{"empty reason", true, reasoned(""), http.StatusBadRequest},
		{"whitespace reason", true, reasoned("   "), http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()))
			if c.seeded {
				e.seedContract(statusSigned)
			}

			reqtest.ExpectStatus(t, http.MethodPost, e.voidRequestURL(), c.want, e.signedIn, c.body)
		})
	}
}

func TestPostVoidRequestHandler_RefusesAnEngagementItCannotReach(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.refusesEngagementsItCannotReach(http.MethodPost, "/void-request", reasoned("asking"))
}

// TestPostVoidRequestHandler_Success proves the happy path: a request
// lands as an open VoidRequestSummary embedded in the ContractResponse,
// naming who asked and why, and the Contract itself stays 'signed' --
// requesting is not voiding.
func TestPostVoidRequestHandler_Success(t *testing.T) {
	e := newEnv(t)
	e.seedContract(statusSigned)

	out := e.requestVoid("the client's plan changed", http.StatusCreated)

	if out.Status != statusSigned {
		t.Fatalf("contract status = %q, want unchanged at %q", out.Status, statusSigned)
	}
	if len(out.VoidRequests) != 1 {
		t.Fatalf("voidRequests = %+v, want exactly one", out.VoidRequests)
	}
	got := out.VoidRequests[0]
	if got.Status != "open" || got.Reason != "the client's plan changed" || got.RequestedBy != e.Staff.ID {
		t.Fatalf("voidRequest = %+v, want open, reasoned, and attributed to %q", got, e.Staff.ID)
	}

	// #1012's own AC: the request lands in the activity ledger too, not
	// just the response body -- a direct SELECT, so this stays
	// independent of #972's read-side money filter, which dropped this
	// action from the money set without changing what row is written.
	// The actor must be the requester.
	assertActivityActor(t, e.db, e.Engagement.ID, activity.ActionContractVoidRequested, e.Staff.ID)
}

// TestPostVoidRequestHandler_DuplicateOpenRefused proves the same person
// cannot accumulate a second open request on the same Contract (#971's
// own AC) -- the unique index refuses it with a 409 rather than a second
// silent row.
func TestPostVoidRequestHandler_DuplicateOpenRefused(t *testing.T) {
	e := newEnv(t)
	e.seedContract(statusSigned)

	e.requestVoid("first ask", http.StatusCreated)
	reqtest.ExpectStatus(t, http.MethodPost, e.voidRequestURL(), http.StatusConflict, e.signedIn, reasoned("second ask"))
}

// TestPostVoidRequestHandler_ContractorReach proves #971's own AC5: a
// contractor Doula may request a void only on an Engagement she holds a
// granted attachment on -- unattached and accrued-only both read as "no
// Engagement here" (404, the same refusal CanAccessEngagement's own doc
// comment gives an unattached contractor everywhere else), and a granted
// attachment succeeds.
func TestPostVoidRequestHandler_ContractorReach(t *testing.T) {
	cases := []struct {
		name       string
		attach     func(*env)
		wantStatus int
	}{
		{"unattached", func(*env) {}, http.StatusNotFound},
		{"accrued only", func(e *env) { testdb.SeedAttachment(e.t, e.db, e.Engagement.ID, e.Staff.ID, "accrued", false) }, http.StatusNotFound},
		{"granted", func(e *env) { testdb.SeedGrantedAttachment(e.t, e.db, e.Engagement.ID, e.Staff.ID) }, http.StatusCreated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Contractor()))
			e.seedContract(statusSigned)
			tc.attach(e)

			if tc.wantStatus != http.StatusCreated {
				reqtest.ExpectStatus(t, http.MethodPost, e.voidRequestURL(), tc.wantStatus, e.signedIn, reasoned("asking as a contractor"))
				return
			}
			out := e.requestVoid("asking as a contractor", tc.wantStatus)
			if _, present := out.Values[priceKey]; present {
				t.Fatalf("Values = %+v, want no price key reachable for a contractor", out.Values)
			}
		})
	}
}

// TestPostVoidRequestDeclineHandler_RefusedByRole proves decline is
// Owner and Admin only, the same table PostVoidContractHandler's own
// role guardrail carries (#970/#282): every Doula is refused regardless
// of employment type.
func TestPostVoidRequestDeclineHandler_RefusedByRole(t *testing.T) {
	cases := []struct {
		name       string
		staff      []fixture.StaffOption
		wantStatus int
	}{
		{"practice owner", []fixture.StaffOption{fixture.Owner()}, http.StatusOK},
		{"admin", []fixture.StaffOption{fixture.Admin()}, http.StatusOK},
		{"employed doula", []fixture.StaffOption{fixture.Doula()}, http.StatusForbidden},
		{"doula, contractor", []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff...))
			e.seedContract(statusSigned)
			requestID := e.seedVoidRequestBy(e.staff(fixture.Owner()), "asking")

			reqtest.ExpectStatus(t, http.MethodPost, e.declineURL(requestID), tc.wantStatus, e.signedIn, reasoned("declining"))
		})
	}
}

// TestPostVoidRequestDeclineHandler_Success proves decline is a distinct
// outcome from voiding: the request's own status reads 'declined', it
// carries the decliner's reason, and the Contract itself is left
// 'signed' -- #971's own AC that a decline is never confused with a void.
func TestPostVoidRequestDeclineHandler_Success(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	e.seedContract(statusSigned)
	// A distinct requester from the decliner: #1012's own activity
	// assertion below needs actor_staff_id to name the decliner and
	// nobody else, which a single-person fixture can't discriminate.
	requestID := e.seedVoidRequestBy(e.staff(fixture.Doula()), "asking")

	out := e.decline(requestID, "not yet", http.StatusOK)

	if out.Status != statusSigned {
		t.Fatalf("contract status = %q, want unchanged at %q", out.Status, statusSigned)
	}
	if len(out.VoidRequests) != 1 {
		t.Fatalf("voidRequests = %+v, want exactly one", out.VoidRequests)
	}
	got := out.VoidRequests[0]
	if got.Status != "declined" || got.DeclineReason != "not yet" {
		t.Fatalf("voidRequest = %+v, want declined with the decliner's reason", got)
	}

	// #1012's own AC: the decline lands in the activity ledger too, actor
	// the decliner rather than the original requester.
	assertActivityActor(t, e.db, e.Engagement.ID, activity.ActionContractVoidDeclined, e.Staff.ID)
}

// TestPostVoidRequestDeclineHandler_AlreadyResolved proves a second
// decision on the same request 404s rather than silently overwriting the
// first.
func TestPostVoidRequestDeclineHandler_AlreadyResolved(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	e.seedContract(statusSigned)
	requestID := e.seedVoidRequestBy(e.Staff, "asking")

	e.decline(requestID, "not yet", http.StatusOK)
	reqtest.ExpectStatus(t, http.MethodPost, e.declineURL(requestID), http.StatusNotFound, e.signedIn, reasoned("still not yet"))
}

// TestPostVoidContractHandler_ClosesOpenRequestsAsVoided proves #971's
// own "void-after-request" AC: an Owner or an Admin acting on a Doula's
// ask is the existing Void endpoint, and it closes the open request as
// voided (not declined) so the requester can tell the two outcomes
// apart.
func TestPostVoidContractHandler_ClosesOpenRequestsAsVoided(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedContract(statusSigned)
	requestID := e.seedVoidRequestBy(e.staff(fixture.Doula()), "please void this")

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.voidURL(), http.StatusOK, e.signedIn)

	if out.Status != statusVoided {
		t.Fatalf("contract status = %q, want %q", out.Status, statusVoided)
	}
	var found bool
	for _, vr := range out.VoidRequests {
		if vr.ID != requestID {
			continue
		}
		found = true
		if vr.Status != statusVoided {
			t.Fatalf("voidRequest status = %q, want %q", vr.Status, statusVoided)
		}
	}
	if !found {
		t.Fatalf("voidRequests = %+v, want the seeded request present", out.VoidRequests)
	}
}

func TestPostVoidRequestDeclineHandler_RefusesAnEngagementItCannotReach(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.refusesEngagementsItCannotReach(http.MethodPost, "/void-request/"+noSuchRequestID+"/decline", reasoned("declining"))
}

// TestPostVoidRequestDeclineHandler_Refusals proves what a decline is
// refused for: a malformed :requestId path segment (400 before any
// request lookup runs), an Engagement with no Contract row at all (404
// before the request id is even looked up), an undecodable body, and an
// empty reason (a field-targeted 400, mirroring the request handler's
// own RequiresReason).
func TestPostVoidRequestDeclineHandler_Refusals(t *testing.T) {
	cases := []struct {
		name      string
		seeded    bool
		requestID func(*env) string
		body      reqtest.Option
		want      int
	}{
		{"request id is not a uuid", true, func(*env) string { return "not-a-uuid" }, reasoned("declining"), http.StatusBadRequest},
		{"no contract", false, func(*env) string { return noSuchRequestID }, reasoned("declining"), http.StatusNotFound},
		{"body is not JSON", true, (*env).openRequest, reqtest.RawJSON([]byte("not json")), http.StatusBadRequest},
		{"empty reason", true, (*env).openRequest, reasoned(""), http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Admin()))
			if c.seeded {
				e.seedContract(statusSigned)
			}

			reqtest.ExpectStatus(t, http.MethodPost, e.declineURL(c.requestID(e)), c.want, e.signedIn, c.body)
		})
	}
}

// openRequest seeds an open void request asked for by the Staff member
// at the screen and returns its id.
func (e *env) openRequest() string {
	e.t.Helper()
	return e.seedVoidRequestBy(e.Staff, "asking")
}

// seedVoidRequest seeds an open contract_void_requests row directly
// (bypassing PostVoidRequestHandler), against engagementID's current
// Contract row -- for a decline/void test that needs a request already
// open rather than exercising the write that would produce one. Mirrors
// fetchContract's own "most recent row" lookup.
func seedVoidRequest(t *testing.T, db *testdb.DB, engagementID, requestedBy, reason string) (requestID string) {
	t.Helper()
	var contractID, practiceID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT c.id, e.practice_id FROM contracts c
		 JOIN engagements e ON e.id = c.engagement_id
		 WHERE c.engagement_id = $1 ORDER BY c.created_at DESC LIMIT 1`,
		engagementID,
	).Scan(&contractID, &practiceID); err != nil {
		t.Fatalf("seed void request: find contract: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contract_void_requests (contract_id, engagement_id, practice_id, requested_by, reason)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		contractID, engagementID, practiceID, requestedBy, reason,
	).Scan(&requestID); err != nil {
		t.Fatalf("seed void request: %v", err)
	}
	return requestID
}
