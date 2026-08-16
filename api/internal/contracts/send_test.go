package contracts_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
	return srv.URL + "/practices/" + practiceID + "/engagements/" + engagementID + "/contract/send"
}

func postSendContract(t *testing.T, srv *httptest.Server, practiceID, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, sendContractURL(srv, practiceID, engagementID), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestPostSendContractHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-invalid-engagement-id"
	practiceID := seedMember(t, db, uid)

	srv := newContractServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := postSendContract(t, srv, practiceID, "not-a-uuid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPostSendContractHandler_EngagementNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-no-engagement"
	practiceID := seedMember(t, db, uid)
	otherPracticeID := seedPractice(t, db, "Other Practice")
	otherEngagementID := seedEngagement(t, db, otherPracticeID)

	srv := newContractServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := postSendContract(t, srv, practiceID, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestPostSendContractHandler_NoContract(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-no-contract"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

	srv := newContractServer(fakeVerifier{uid: uid}, db)
	defer srv.Close()

	resp := postSendContract(t, srv, practiceID, engagementID)
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
			practiceID := seedMember(t, db, uid)
			engagementID := seedEngagement(t, db, practiceID)
			seedContract(t, db, engagementID, status, mergeFieldProse)

			srv := newContractServer(fakeVerifier{uid: uid}, db)
			defer srv.Close()

			resp := postSendContract(t, srv, practiceID, engagementID)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
			}
		})
	}
}

// TestPostSendContractHandler_Success proves the draft -> sent transition,
// that the response reflects the new status while keeping the Contract's
// prose/mergeFields/values intact, and that it triggers exactly one
// Pusher.Send call to the Client's registered subscription with a
// body-free payload (just the Engagement id -- no Contract content).
func TestPostSendContractHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-success"
	practiceID := seedMember(t, db, uid)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)
	seedPushSubscription(t, db, "client", clientID, "https://push.example.com/client-recipient")

	pusher := push.NewFakePusher()
	srv := newContractServerWithPusher(fakeVerifier{uid: uid}, db, pusher)
	defer srv.Close()

	resp := postSendContract(t, srv, practiceID, engagementID)
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

	getResp := getContract(t, srv, practiceID, engagementID)
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
// delivery is best-effort, not part of Send's own success criteria.
func TestPostSendContractHandler_NoSubscriptionNoPush(t *testing.T) {
	db := testdb.New(t)
	const uid = "send-no-subscription"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	pusher := push.NewFakePusher()
	srv := newContractServerWithPusher(fakeVerifier{uid: uid}, db, pusher)
	defer srv.Close()

	resp := postSendContract(t, srv, practiceID, engagementID)
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
	practiceID := seedMember(t, db, uid)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)
	seedPushSubscription(t, db, "client", clientID, "https://push.example.com/gone")

	pusher := push.NewFakePusher()
	pusher.Err = errors.New("simulated push service failure")
	srv := newContractServerWithPusher(fakeVerifier{uid: uid}, db, pusher)
	defer srv.Close()

	resp := postSendContract(t, srv, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if calls := pusher.Calls(); len(calls) != 1 {
		t.Fatalf("Pusher.Send call count = %d, want 1 (still attempted): %+v", len(calls), calls)
	}
}
