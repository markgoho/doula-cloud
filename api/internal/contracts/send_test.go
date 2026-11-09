package contracts_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
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

func (e *env) send() string {
	return e.contractURL("/send")
}

// requireNothingSent proves a refused Send wrote nothing: the Contract is
// still a draft, no contract_sent activity row exists, and no push fired.
func (e *env) requireNothingSent() {
	e.T.Helper()
	if got := e.contract().Status; got != statusDraft {
		e.T.Fatalf("status after refused Send = %q, want draft (unchanged)", got)
	}
	if calls := e.pusher.Calls(); len(calls) != 0 {
		e.T.Fatalf("Pusher.Send call count = %d, want 0 (refused before any write)", len(calls))
	}
	activityCount := testdb.Scalar[int](e.T, e.DB.Admin, `SELECT count(*) FROM activity WHERE subject_id = $1 AND action = 'contract_sent'`, e.Engagement.ID)
	if activityCount != 0 {
		e.T.Fatalf("contract_sent activity rows = %d, want 0 (refused before any write)", activityCount)
	}
}

// seedFilledDraft seeds a draft whose merge fields are all filled in, so
// Send's completeness check passes.
func (e *env) seedFilledDraft() {
	e.T.Helper()
	seedContractWithValues(e.T, e.DB, e.Engagement.ID,
		contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: testPriceValue})
}

func TestPostSendContractHandler_RefusesAnEngagementItCannotReach(t *testing.T) {
	e := newEnv(t)
	e.refusesEngagementsItCannotReach(http.MethodPost, "/send")
}

func TestPostSendContractHandler_NoContract(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodPost, e.send(), http.StatusNotFound, e.SignedIn)
}

// TestPostSendContractHandler_ContractorWithoutAttachmentNotFound is
// #970's AC for send specifically: staffauth.AnyStaff at the mount adds
// no role restriction of its own, so an unattached contractor still gets
// 404 from AttachingWrite's reach test before the handler ever runs.
func TestPostSendContractHandler_ContractorWithoutAttachmentNotFound(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))

	reqtest.ExpectStatus(t, http.MethodPost, e.send(), http.StatusNotFound, e.SignedIn)
}

// TestPostSendContractHandler_NonDraftRejected proves Send 409s once a
// Contract has already moved past 'draft' -- it's a one-way transition.
func TestPostSendContractHandler_NonDraftRejected(t *testing.T) {
	for _, status := range []string{statusSent, statusSigned, statusVoided} {
		t.Run(status, func(t *testing.T) {
			e := newEnv(t)
			e.seedContract(status)

			reqtest.ExpectStatus(t, http.MethodPost, e.send(), http.StatusConflict, e.SignedIn)
		})
	}
}

// TestPostSendContractHandler_RefusedInviteHoldsTheSendBack proves #2522:
// an invite whose accept was refused holds Send back as never-invited
// does, and says the way on without naming the other record.
func TestPostSendContractHandler_RefusedInviteHoldsTheSendBack(t *testing.T) {
	e := newEnv(t)
	e.seedDraft()
	testdb.SeedPendingPortalInvite(t, e.DB, e.Engagement.Client.ID)
	e.Exec(`UPDATE client_portal_users SET access_refused_at = now() WHERE client_id = $1`, e.Engagement.Client.ID)

	out := apierrtest.Expect(t, http.MethodPost, e.send(), http.StatusConflict, e.SignedIn)

	if out.Code != apierr.CodeFailedPrecondition || !strings.Contains(out.Message, "signs in through another record") {
		t.Fatalf("refusal = %+v, want FAILED_PRECONDITION naming the other record's sign-in", out)
	}
	e.requireNothingSent()
}

// TestPostSendContractHandler_NoPortalInvite proves the #255 precondition:
// Send refuses with 409 FAILED_PRECONDITION when the Engagement's Client
// has never been sent a portal invite, before any write -- the Contract
// stays a draft, no contract-sent activity entry is written, and no push
// fires.
func TestPostSendContractHandler_NoPortalInvite(t *testing.T) {
	e := newEnv(t)
	e.seedDraft()
	testdb.SeedPushSubscription(t, e.DB, "client", e.Engagement.Client.ID, "https://push.example.com/no-invite-recipient")

	out := apierrtest.Expect(t, http.MethodPost, e.send(), http.StatusConflict, e.SignedIn)

	if out.Code != apierr.CodeFailedPrecondition {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeFailedPrecondition)
	}
	if !strings.Contains(out.Message, "invited") {
		t.Fatalf("message = %q, want it to name the missing invite", out.Message)
	}
	e.requireNothingSent()
}

// TestPostSendContractHandler_Success proves the draft -> sent transition,
// that the response reflects the new status while keeping the Contract's
// prose/mergeFields/values intact, and that it triggers exactly one
// Pusher.Send call to the Client's registered subscription with a
// body-free payload (just the Engagement id -- no Contract content). The
// Client holds a pending (not yet accepted) portal invite -- #255's own
// AC that Send succeeds before acceptance, not only after it.
func TestPostSendContractHandler_Success(t *testing.T) {
	e := newEnv(t)
	e.seedFilledDraft()
	testdb.SeedPushSubscription(t, e.DB, "client", e.Engagement.Client.ID, "https://push.example.com/client-recipient")
	testdb.SeedPendingPortalInvite(t, e.DB, e.Engagement.Client.ID)

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.send(), http.StatusOK, e.SignedIn)

	if out.Status != statusSent {
		t.Fatalf("status = %q, want sent", out.Status)
	}
	if out.Prose != mergeFieldProse {
		t.Fatalf("prose = %q, want the original snapshot unchanged", out.Prose)
	}
	if got := e.contract().Status; got != statusSent {
		t.Fatalf("GET status after Send = %q, want sent (the transition persisted)", got)
	}

	calls := e.pusher.Calls()
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
	if payload.EngagementID != e.Engagement.ID {
		t.Fatalf("push payload engagementId = %q, want %q", payload.EngagementID, e.Engagement.ID)
	}
	var raw map[string]jsontext.Value
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
	e := newEnv(t, withPortalLogin())
	e.seedFilledDraft()

	reqtest.ExpectStatus(t, http.MethodPost, e.send(), http.StatusOK, e.SignedIn)

	if calls := e.pusher.Calls(); len(calls) != 0 {
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
	e := newEnv(t)
	e.seedFilledDraft()
	testdb.SeedPushSubscription(t, e.DB, "client", e.Engagement.Client.ID, "https://push.example.com/gone")
	testdb.SeedPendingPortalInvite(t, e.DB, e.Engagement.Client.ID)
	e.pusher.Err = errors.New("simulated push service failure")

	reqtest.ExpectStatus(t, http.MethodPost, e.send(), http.StatusOK, e.SignedIn)

	if calls := e.pusher.Calls(); len(calls) != 1 {
		t.Fatalf("Pusher.Send call count = %d, want 1 (still attempted): %+v", len(calls), calls)
	}
}

// TestPostSendContractHandler_BlankMergeFieldRejected proves #258's
// completeness precondition: Send refuses 409 FAILED_PRECONDITION when a
// merge field parsed out of the prose has no value at all, before any
// write -- the Contract stays a draft, no contract-sent activity entry
// is written, and no push fires. mergeFieldProse parses two keys;
// client_name is never filled and is refused, while price resolves
// automatically from amount_cents (#967) and is never among the missing
// fields, however empty merge_field_values is.
func TestPostSendContractHandler_BlankMergeFieldRejected(t *testing.T) {
	e := newEnv(t)
	e.seedDraft()
	testdb.SeedPendingPortalInvite(t, e.DB, e.Engagement.Client.ID)

	out := apierrtest.Expect(t, http.MethodPost, e.send(), http.StatusConflict, e.SignedIn)

	if out.Code != apierr.CodeFailedPrecondition {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeFailedPrecondition)
	}
	if _, ok := out.Details[clientNameKey]; !ok {
		t.Fatalf("details = %+v, want an entry for %q", out.Details, clientNameKey)
	}
	if _, ok := out.Details[priceKey]; ok {
		t.Fatalf("details = %+v, want no entry for %q -- it resolves automatically from amount_cents (#967)", out.Details, priceKey)
	}
	e.requireNothingSent()
}

// TestPostSendContractHandler_WhitespaceOnlyMergeFieldRejected proves a
// value that is only whitespace counts as missing, the same as an empty
// string or an absent key, and that a Contract with one filled key and
// one blank key is refused naming only the blank one. Uses client_name
// as the blank field and price as the filled one: price can no longer be
// blank at all (#967 resolves it from amount_cents on every read,
// regardless of what seedContractWithValues stores for it), so it is the
// "filled" side of this proof rather than a candidate for it.
func TestPostSendContractHandler_WhitespaceOnlyMergeFieldRejected(t *testing.T) {
	e := newEnv(t)
	seedContractWithValues(t, e.DB, e.Engagement.ID, contracts.MergeFieldValues{clientNameKey: "   "})
	testdb.SeedPendingPortalInvite(t, e.DB, e.Engagement.Client.ID)

	out := apierrtest.Expect(t, http.MethodPost, e.send(), http.StatusConflict, e.SignedIn)

	if _, ok := out.Details[clientNameKey]; !ok {
		t.Fatalf("details = %+v, want an entry for %q", out.Details, clientNameKey)
	}
	if _, ok := out.Details[priceKey]; ok {
		t.Fatalf("details = %+v, want no entry for the auto-resolved %q", out.Details, priceKey)
	}
}
