package contracts_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestContractTrail_SentAndSigned is #2104: the Staff Contract read says
// when the Contract was sent and by whom, and when it was signed and
// under what name -- the send's own response included, so the page
// shows it without reading again -- and a send recorded before this
// Contract existed (a voided predecessor's) is not this Contract's.
func TestContractTrail_SentAndSigned(t *testing.T) {
	e := newEnv(t)
	// A predecessor's send, recorded before this Contract was created.
	testdb.SeedActivity(t, e.DB, e.Practice.ID, activity.SubjectEngagement, e.Engagement.ID,
		string(activity.ActionContractSent), activity.StaffActor(e.Staff.ID))
	e.seedFilledDraft()
	testdb.SeedPendingPortalInvite(t, e.DB, e.Engagement.Client.ID)

	draft := e.contract()
	if draft.Sent != nil || draft.Signed != nil {
		t.Fatalf("a Draft: sent = %+v, signed = %+v, want neither", draft.Sent, draft.Signed)
	}

	sent := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.send(), http.StatusOK, e.SignedIn)
	if sent.Sent == nil || sent.Sent.By != e.Staff.Name || sent.Sent.At.IsZero() {
		t.Fatalf("the send's response: sent = %+v, want its instant, by the sender", sent.Sent)
	}

	e.Exec(`UPDATE contracts SET status = 'voided', signed_at = now(), signer_full_name = 'Jordan Q. Client'
		 WHERE engagement_id = $1`, e.Engagement.ID)
	voided := e.contract()
	if voided.Signed == nil || voided.Signed.By != "Jordan Q. Client" || voided.Signed.At.IsZero() {
		t.Errorf("signed = %+v, want the instant and the typed legal name", voided.Signed)
	}
	if voided.Sent == nil || !voided.Sent.At.Equal(sent.Sent.At) {
		t.Errorf("sent = %+v, want the same send a void leaves in place", voided.Sent)
	}
}
