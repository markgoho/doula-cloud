package contracts_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

func voidRequestURL(srv *httptest.Server, practiceID, engagementID string) string {
	return contractURL(srv, practiceID, engagementID) + "/void-request"
}

func voidRequestDeclineURL(srv *httptest.Server, practiceID, engagementID, requestID string) string {
	return voidRequestURL(srv, practiceID, engagementID) + "/" + requestID + "/decline"
}

func postJSONWithSession(t *testing.T, session, url string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func postVoidRequest(t *testing.T, srv *httptest.Server, session, practiceID, engagementID, reason string) *http.Response {
	t.Helper()
	return postJSONWithSession(t, session, voidRequestURL(srv, practiceID, engagementID),
		map[string]string{"reason": reason})
}

func postVoidRequestDecline(t *testing.T, srv *httptest.Server, session, practiceID, engagementID, requestID, reason string) *http.Response {
	t.Helper()
	return postJSONWithSession(t, session, voidRequestDeclineURL(srv, practiceID, engagementID, requestID),
		map[string]string{"reason": reason})
}

// decodeContractResponse fails the test unless status matches, and
// returns the decoded body -- mirrors the many local decode-and-assert
// blocks this package's other test files repeat inline.
func decodeContractResponse(t *testing.T, resp *http.Response, wantStatus int) contracts.ContractResponse {
	t.Helper()
	if resp.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", resp.StatusCode, wantStatus)
	}
	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// TestPostVoidRequestHandler_RequiresSignedContract proves a void may
// only be requested for a Signed Contract -- the same precondition
// PostVoidContractHandler's own transition carries, checked here without
// registering a Transition since this route never writes contracts.
func TestPostVoidRequestHandler_RequiresSignedContract(t *testing.T) {
	for _, status := range []string{statusDraft, statusSent, statusVoided} {
		t.Run(status, func(t *testing.T) {
			db := testdb.New(t)
			uid := "void-request-non-signed-" + status
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			seedContract(t, db, engagementID, status, mergeFieldProse)

			srv, session := newContractServer(t, db, uid)
			defer srv.Close()

			resp := postVoidRequest(t, srv, session, practiceID, engagementID, "the client's plan changed")
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
			}
		})
	}
}

// TestPostVoidRequestHandler_RequiresReason proves an empty (or
// whitespace-only) reason is refused with a field-targeted 400, mirroring
// engagement/fielderrors.go's own "Enter ..." wording convention.
func TestPostVoidRequestHandler_RequiresReason(t *testing.T) {
	// blankReason is a local identifier rather than a third bare "   "
	// literal -- sign_test.go and send_test.go already carry one each,
	// and goconst flags a third occurrence of the same string.
	const blankReason = "   "
	for name, reason := range map[string]string{"empty": "", "whitespace": blankReason} {
		t.Run(name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "void-request-no-reason-" + name
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			seedContract(t, db, engagementID, statusSigned, mergeFieldProse)

			srv, session := newContractServer(t, db, uid)
			defer srv.Close()

			resp := postVoidRequest(t, srv, session, practiceID, engagementID, reason)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// TestPostVoidRequestHandler_Success proves the happy path: a request
// lands as an open VoidRequestSummary embedded in the ContractResponse,
// naming who asked and why, and the Contract itself stays 'signed' --
// requesting is not voiding.
func TestPostVoidRequestHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-request-success"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSigned, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidRequest(t, srv, session, practiceID, engagementID, "the client's plan changed")
	defer resp.Body.Close()

	out := decodeContractResponse(t, resp, http.StatusCreated)
	if out.Status != statusSigned {
		t.Fatalf("contract status = %q, want unchanged at %q", out.Status, statusSigned)
	}
	if len(out.VoidRequests) != 1 {
		t.Fatalf("voidRequests = %+v, want exactly one", out.VoidRequests)
	}
	got := out.VoidRequests[0]
	if got.Status != "open" || got.Reason != "the client's plan changed" || got.RequestedBy != staffID {
		t.Fatalf("voidRequest = %+v, want open, reasoned, and attributed to %q", got, staffID)
	}

	// #1012's own AC: the request lands in the activity ledger too, not
	// just the response body -- a direct SELECT, so this stays
	// independent of #972's read-side money filter, which dropped this
	// action from the money set without changing what row is written.
	// The actor must be the requester.
	assertActivityActor(t, db, engagementID, activity.ActionContractVoidRequested, staffID)
}

// TestPostVoidRequestHandler_DuplicateOpenRefused proves the same person
// cannot accumulate a second open request on the same Contract (#971's
// own AC) -- the unique index refuses it with a 409 rather than a second
// silent row.
func TestPostVoidRequestHandler_DuplicateOpenRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-request-duplicate"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSigned, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	first := postVoidRequest(t, srv, session, practiceID, engagementID, "first ask")
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first request status = %d, want %d", first.StatusCode, http.StatusCreated)
	}

	second := postVoidRequest(t, srv, session, practiceID, engagementID, "second ask")
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second request status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
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
		wantStatus int
	}{
		{"unattached", http.StatusNotFound},
		{"accrued only", http.StatusNotFound},
		{"granted", http.StatusCreated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "void-request-contractor-" + tc.name
			practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			seedContract(t, db, engagementID, statusSigned, mergeFieldProse)
			switch tc.name {
			case "accrued only":
				testdb.SeedAttachment(t, db, engagementID, staffID, "accrued", false)
			case "granted":
				testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
			}

			srv, session := newContractServer(t, db, uid)
			defer srv.Close()

			resp := postVoidRequest(t, srv, session, practiceID, engagementID, "asking as a contractor")
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusCreated {
				out := decodeContractResponse(t, resp, http.StatusCreated)
				if _, present := out.Values[priceKey]; present {
					t.Fatalf("Values = %+v, want no price key reachable for a contractor", out.Values)
				}
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
		name           string
		roles          []string
		employmentType string
		wantStatus     int
	}{
		{"practice owner", []string{ownerRole}, employeeType, http.StatusOK},
		{"admin", []string{adminRole}, employeeType, http.StatusOK},
		{"employed doula", []string{doulaRole}, employeeType, http.StatusForbidden},
		{"doula, contractor", []string{doulaRole}, contractorType, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "void-decline-role-" + tc.name
			practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			seedContract(t, db, engagementID, statusSigned, mergeFieldProse)
			requestID := seedVoidRequest(t, db, engagementID, ownerID, "asking")

			actorUID := uid
			if tc.name != "practice owner" {
				actorUID = uid + "-actor"
				testdb.SeedStaffAtPractice(t, db, practiceID, actorUID, tc.roles, tc.employmentType)
			}

			srv, session := newContractServer(t, db, actorUID)
			defer srv.Close()

			resp := postVoidRequestDecline(t, srv, session, practiceID, engagementID, requestID, "declining")
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}

// TestPostVoidRequestDeclineHandler_Success proves decline is a distinct
// outcome from voiding: the request's own status reads 'declined', it
// carries the decliner's reason, and the Contract itself is left
// 'signed' -- #971's own AC that a decline is never confused with a void.
func TestPostVoidRequestDeclineHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-decline-success"
	practiceID, adminID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	// A distinct requester from the decliner: #1012's own activity
	// assertion below needs actor_staff_id to name the decliner and
	// nobody else, which a single-person fixture can't discriminate.
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, uid+"-doula", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSigned, mergeFieldProse)
	requestID := seedVoidRequest(t, db, engagementID, doulaID, "asking")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidRequestDecline(t, srv, session, practiceID, engagementID, requestID, "not yet")
	defer resp.Body.Close()

	out := decodeContractResponse(t, resp, http.StatusOK)
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
	// the decliner (adminID) rather than the original requester.
	assertActivityActor(t, db, engagementID, activity.ActionContractVoidDeclined, adminID)
}

// TestPostVoidRequestDeclineHandler_AlreadyResolved proves a second
// decision on the same request 404s rather than silently overwriting the
// first.
func TestPostVoidRequestDeclineHandler_AlreadyResolved(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-decline-already-resolved"
	practiceID, adminID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSigned, mergeFieldProse)
	requestID := seedVoidRequest(t, db, engagementID, adminID, "asking")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	first := postVoidRequestDecline(t, srv, session, practiceID, engagementID, requestID, "not yet")
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first decline status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := postVoidRequestDecline(t, srv, session, practiceID, engagementID, requestID, "still not yet")
	defer second.Body.Close()
	if second.StatusCode != http.StatusNotFound {
		t.Fatalf("second decline status = %d, want %d", second.StatusCode, http.StatusNotFound)
	}
}

// TestPostVoidContractHandler_ClosesOpenRequestsAsVoided proves #971's
// own "void-after-request" AC: an Owner or an Admin acting on a Doula's
// ask is the existing Void endpoint, and it closes the open request as
// voided (not declined) so the requester can tell the two outcomes
// apart.
func TestPostVoidContractHandler_ClosesOpenRequestsAsVoided(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-after-request"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, uid+"-doula", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSigned, mergeFieldProse)
	requestID := seedVoidRequest(t, db, engagementID, doulaID, "please void this")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	out := decodeContractResponse(t, resp, http.StatusOK)
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

// TestPostVoidRequestHandler_InvalidEngagementID mirrors
// TestPostVoidContractHandler_InvalidEngagementID: a malformed
// :engagementId is refused before any Contract lookup runs.
func TestPostVoidRequestHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-request-invalid-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidRequest(t, srv, session, practiceID, "not-a-uuid", "asking")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostVoidRequestHandler_NoContract proves an Engagement with no
// Contract row at all 404s, the same refusal PostVoidContractHandler's
// own TestPostVoidContractHandler_NoContract carries.
func TestPostVoidRequestHandler_NoContract(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-request-no-contract"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidRequest(t, srv, session, practiceID, engagementID, "asking")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostVoidRequestHandler_MalformedBody proves an undecodable request
// body 400s through apierr.DecodeJSON's own refusal, same as every other
// Contract write in this package.
func TestPostVoidRequestHandler_MalformedBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-request-malformed-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSigned, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, voidRequestURL(srv, practiceID, engagementID), bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostVoidRequestDeclineHandler_InvalidEngagementID mirrors the
// request handler's own equivalent above.
func TestPostVoidRequestDeclineHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-decline-invalid-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidRequestDecline(t, srv, session, practiceID, "not-a-uuid", "00000000-0000-0000-0000-000000000000", "declining")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostVoidRequestDeclineHandler_InvalidRequestID proves a malformed
// :requestId path segment 400s before any request lookup runs.
func TestPostVoidRequestDeclineHandler_InvalidRequestID(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-decline-invalid-request-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSigned, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidRequestDecline(t, srv, session, practiceID, engagementID, "not-a-uuid", "declining")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostVoidRequestDeclineHandler_NoContract proves an Engagement with
// no Contract row at all 404s before the request id is even looked up.
func TestPostVoidRequestDeclineHandler_NoContract(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-decline-no-contract"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidRequestDecline(t, srv, session, practiceID, engagementID, "00000000-0000-0000-0000-000000000000", "declining")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostVoidRequestDeclineHandler_MalformedBody mirrors the request
// handler's own equivalent above.
func TestPostVoidRequestDeclineHandler_MalformedBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-decline-malformed-body"
	practiceID, adminID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSigned, mergeFieldProse)
	requestID := seedVoidRequest(t, db, engagementID, adminID, "asking")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, voidRequestDeclineURL(srv, practiceID, engagementID, requestID), bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostVoidRequestDeclineHandler_RequiresReason proves an empty
// decline reason is refused with a field-targeted 400, mirroring the
// request handler's own TestPostVoidRequestHandler_RequiresReason.
func TestPostVoidRequestDeclineHandler_RequiresReason(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-decline-no-reason"
	practiceID, adminID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusSigned, mergeFieldProse)
	requestID := seedVoidRequest(t, db, engagementID, adminID, "asking")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidRequestDecline(t, srv, session, practiceID, engagementID, requestID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
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
