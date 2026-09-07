package contracts_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/testdb"
)

// sendPushNotificationPayload mirrors send.go's sendPushPayload's JSON
// shape (that type is unexported, so tests in this external package
// redeclare the wire shape to decode against) -- the AC this proves is
// "a body-free payload: no Contract content", so only this one field
// should ever appear.
type sendPushNotificationPayload struct {
	EngagementID string `json:"engagementId"`
}

func sendContractURL(srv *httptest.Server, practiceID, engagementID string) string {
	return srv.URL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/contract/send"
}

func postSendContract(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, sendContractURL(srv, practiceID, engagementID), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestPostSendContractHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-invalid-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postSendContract(t, srv, session, practiceID, "not-a-uuid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPostSendContractHandler_EngagementNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-no-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postSendContract(t, srv, session, practiceID, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestPostSendContractHandler_NoContract(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-no-contract"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postSendContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostSendContractHandler_NonDraftRejected proves Send 409s once a
// Contract has already moved past 'draft' -- it's a one-way transition.
func TestPostSendContractHandler_NonDraftRejected(t *testing.T) {
	for _, status := range []string{statusSent, statusSigned, statusVoided} {
		t.Run(status, func(t *testing.T) {
			db := testdb.New(t)
			uid := "send-non-draft-" + status
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			seedContract(t, db, engagementID, status, mergeFieldProse)

			srv, session := newContractServer(t, db, uid)
			defer srv.Close()

			resp := postSendContract(t, srv, session, practiceID, engagementID)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
			}
		})
	}
}

// TestPostSendContractHandler_NoPortalInvite proves the #255 precondition:
// Send refuses with 409 FAILED_PRECONDITION when the Engagement's Client
// has never been sent a portal invite, before any write -- the Contract
// stays a draft, no contract-sent activity entry is written, and no push
// fires.
func TestPostSendContractHandler_NoPortalInvite(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-no-portal-invite"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Never Invited Client", "never-invited@example.com")
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)
	testdb.SeedPushSubscription(t, db, "client", clientID, "https://push.example.com/no-invite-recipient")

	pusher := push.NewFakePusher()
	srv, session := newContractServerWithPusher(t, db, uid, pusher)
	defer srv.Close()

	resp := postSendContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Code != string(apierr.CodeFailedPrecondition) {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeFailedPrecondition)
	}
	if !strings.Contains(out.Message, "invited") {
		t.Fatalf("message = %q, want it to name the missing invite", out.Message)
	}

	getResp := getContract(t, srv, session, practiceID, engagementID)
	defer getResp.Body.Close()
	var getOut contracts.ContractResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getOut.Status != statusDraft {
		t.Fatalf("status after refused Send = %q, want draft (unchanged)", getOut.Status)
	}
	if calls := pusher.Calls(); len(calls) != 0 {
		t.Fatalf("Pusher.Send call count = %d, want 0 (refused before any write)", len(calls))
	}
	var activityCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE subject_id = $1 AND action = 'contract_sent'`,
		engagementID,
	).Scan(&activityCount); err != nil {
		t.Fatalf("count activity rows: %v", err)
	}
	if activityCount != 0 {
		t.Fatalf("contract_sent activity rows = %d, want 0 (refused before any write)", activityCount)
	}
}

// TestPostSendContractHandler_Success proves the draft -> sent transition,
// that the response reflects the new status while keeping the Contract's
// prose/mergeFields/values intact, and that it triggers exactly one
// Pusher.Send call to the Client's registered subscription with a
// body-free payload (just the Engagement id -- no Contract content). The
// Client holds a pending (not yet accepted) portal invite -- #255's own
// AC that Send succeeds before acceptance, not only after it.
func TestPostSendContractHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-success"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedContractWithValues(t, db, engagementID,
		contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: testPriceValue})
	testdb.SeedPushSubscription(t, db, "client", clientID, "https://push.example.com/client-recipient")
	testdb.SeedPendingPortalInvite(t, db, clientID)

	pusher := push.NewFakePusher()
	srv, session := newContractServerWithPusher(t, db, uid, pusher)
	defer srv.Close()

	resp := postSendContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != statusSent {
		t.Fatalf("status = %q, want sent", out.Status)
	}
	if out.Prose != mergeFieldProse {
		t.Fatalf("prose = %q, want the original snapshot unchanged", out.Prose)
	}

	getResp := getContract(t, srv, session, practiceID, engagementID)
	defer getResp.Body.Close()
	var getOut contracts.ContractResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getOut.Status != statusSent {
		t.Fatalf("GET status after Send = %q, want sent (the transition persisted)", getOut.Status)
	}

	calls := pusher.Calls()
	if len(calls) != 1 {
		t.Fatalf("Pusher.Send call count = %d, want 1: %+v", len(calls), calls)
	}
	if calls[0].Subscription.Endpoint != "https://push.example.com/client-recipient" {
		t.Fatalf("notified endpoint = %q, want the Client's own subscription", calls[0].Subscription.Endpoint)
	}
	var payload sendPushNotificationPayload
	if err := json.Unmarshal(calls[0].Payload, &payload); err != nil {
		t.Fatalf("decode push payload: %v", err)
	}
	if payload.EngagementID != engagementID {
		t.Fatalf("push payload engagementId = %q, want %q", payload.EngagementID, engagementID)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(calls[0].Payload, &raw); err != nil {
		t.Fatalf("decode push payload as raw map: %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("push payload keys = %v, want only engagementId (body-free, no Contract content)", raw)
	}
}

// TestPostSendContractHandler_NoSubscriptionNoPush proves Send still
// succeeds when the Client has no registered push subscription -- push
// delivery is best-effort, not part of Send's own success criteria. Its
// Client has an accepted portal invite -- #255's own AC that Send
// succeeds for a Client who has already accepted, not only a pending one
// (TestPostSendContractHandler_Success covers the pending case).
func TestPostSendContractHandler_NoSubscriptionNoPush(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-no-subscription"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractWithValues(t, db, engagementID,
		contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: testPriceValue})
	testdb.SeedPortalUser(t, db, "send-no-subscription-portal", clientID)

	pusher := push.NewFakePusher()
	srv, session := newContractServerWithPusher(t, db, uid, pusher)
	defer srv.Close()

	resp := postSendContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if calls := pusher.Calls(); len(calls) != 0 {
		t.Fatalf("Pusher.Send call count = %d, want 0 (no subscription registered)", len(calls))
	}
}

// TestPostSendContractHandler_PushFailureDoesNotBlockSend proves push
// delivery failure (e.g. the recipient's subscription expired at the push
// service) is non-fatal: the Contract's status has already been written
// to tx by the time Pusher.Send runs, so a push failure is logged, not
// surfaced as a 500 -- mirrors message's
// TestCreateHandler_PushFailureDoesNotBlockMessageCreation.
func TestPostSendContractHandler_PushFailureDoesNotBlockSend(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-push-fails"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedContractWithValues(t, db, engagementID,
		contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: testPriceValue})
	testdb.SeedPushSubscription(t, db, "client", clientID, "https://push.example.com/gone")
	testdb.SeedPendingPortalInvite(t, db, clientID)

	pusher := push.NewFakePusher()
	pusher.Err = errors.New("simulated push service failure")
	srv, session := newContractServerWithPusher(t, db, uid, pusher)
	defer srv.Close()

	resp := postSendContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if calls := pusher.Calls(); len(calls) != 1 {
		t.Fatalf("Pusher.Send call count = %d, want 1 (still attempted): %+v", len(calls), calls)
	}
}

// TestPostSendContractHandler_BlankMergeFieldRejected proves #258's
// completeness precondition: Send refuses 409 FAILED_PRECONDITION when a
// merge field parsed out of the prose has no value at all, before any
// write -- the Contract stays a draft, no contract-sent activity entry
// is written, and no push fires. mergeFieldProse parses two keys and
// neither is filled, so the refusal names both, not just the first.
func TestPostSendContractHandler_BlankMergeFieldRejected(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-blank-merge-field"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Never Filled Client", "never-filled@example.com")
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)
	testdb.SeedPendingPortalInvite(t, db, clientID)

	pusher := push.NewFakePusher()
	srv, session := newContractServerWithPusher(t, db, uid, pusher)
	defer srv.Close()

	resp := postSendContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Code != string(apierr.CodeFailedPrecondition) {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeFailedPrecondition)
	}
	if _, ok := out.Details[clientNameKey]; !ok {
		t.Fatalf("details = %+v, want an entry for %q", out.Details, clientNameKey)
	}
	if _, ok := out.Details[priceKey]; !ok {
		t.Fatalf("details = %+v, want an entry for %q", out.Details, priceKey)
	}

	getResp := getContract(t, srv, session, practiceID, engagementID)
	defer getResp.Body.Close()
	var getOut contracts.ContractResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getOut.Status != statusDraft {
		t.Fatalf("status after refused Send = %q, want draft (unchanged)", getOut.Status)
	}
	if calls := pusher.Calls(); len(calls) != 0 {
		t.Fatalf("Pusher.Send call count = %d, want 0 (refused before any write)", len(calls))
	}
	var activityCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE subject_id = $1 AND action = 'contract_sent'`,
		engagementID,
	).Scan(&activityCount); err != nil {
		t.Fatalf("count activity rows: %v", err)
	}
	if activityCount != 0 {
		t.Fatalf("contract_sent activity rows = %d, want 0 (refused before any write)", activityCount)
	}
}

// TestPostSendContractHandler_WhitespaceOnlyMergeFieldRejected proves a
// value that is only whitespace counts as missing, the same as an empty
// string or an absent key, and that a Contract with one filled key and
// one blank key is refused naming only the blank one.
func TestPostSendContractHandler_WhitespaceOnlyMergeFieldRejected(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-whitespace-merge-field"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Whitespace Client", "whitespace@example.com")
	seedContractWithValues(t, db, engagementID,
		contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: "   "})
	testdb.SeedPendingPortalInvite(t, db, clientID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postSendContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := out.Details[priceKey]; !ok {
		t.Fatalf("details = %+v, want an entry for %q", out.Details, priceKey)
	}
	if _, ok := out.Details[clientNameKey]; ok {
		t.Fatalf("details = %+v, want no entry for the filled %q", out.Details, clientNameKey)
	}
}
