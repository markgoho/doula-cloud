package contracts_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

// readContract is decodeContract that also closes the body.
func readContract(t *testing.T, resp *http.Response) contracts.ContractResponse {
	t.Helper()
	defer resp.Body.Close()
	return decodeContract(t, resp)
}

// TestContractTrail_SentAndSigned is #2104: the Staff Contract read says
// when the Contract was sent and by whom, and when it was signed and
// under what name -- the send's own response included, so the page
// shows it without reading again -- and a send recorded before this
// Contract existed (a voided predecessor's) is not this Contract's.
func TestContractTrail_SentAndSigned(t *testing.T) {
	db := testdb.New(t)
	const uid = "trail-sent-signed"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "trail@example.com")
	// A predecessor's send, recorded before this Contract was created.
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID,
		string(activity.ActionContractSent), activity.StaffActor(staffID))
	seedContractWithValues(t, db, engagementID,
		contracts.MergeFieldValues{clientNameKey: jamieName, priceKey: testPriceValue})
	testdb.SeedPendingPortalInvite(t, db, clientID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	draft := readContract(t, getContract(t, srv, session, practiceID, engagementID))
	if draft.Sent != nil || draft.Signed != nil {
		t.Fatalf("a Draft: sent = %+v, signed = %+v, want neither", draft.Sent, draft.Signed)
	}

	sent := readContract(t, postSendContract(t, srv, session, practiceID, engagementID))
	if sent.Sent == nil || sent.Sent.By != "Test Staff "+uid || sent.Sent.At.IsZero() {
		t.Fatalf("the send's response: sent = %+v, want its instant, by the sender", sent.Sent)
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE contracts SET status = 'voided', signed_at = now(), signer_full_name = 'Jordan Q. Client'
		 WHERE engagement_id = $1`, engagementID,
	); err != nil {
		t.Fatalf("sign and void: %v", err)
	}
	voided := readContract(t, getContract(t, srv, session, practiceID, engagementID))
	if voided.Signed == nil || voided.Signed.By != "Jordan Q. Client" || voided.Signed.At.IsZero() {
		t.Errorf("signed = %+v, want the instant and the typed legal name", voided.Signed)
	}
	if voided.Sent == nil || !voided.Sent.At.Equal(sent.Sent.At) {
		t.Errorf("sent = %+v, want the same send a void leaves in place", voided.Sent)
	}
}
