package contracts_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// pendingNotices is the pending contract_notice_outbox rows as
// "kind triggered_by", joined and ordered.
func (e *env) pendingNotices() string {
	e.T.Helper()
	return testdb.Scalar[string](e.T, e.DB.Admin,
		`SELECT COALESCE(string_agg(kind::text || ' ' || triggered_by_staff_id, ',' ORDER BY kind::text), '')
		   FROM contract_notice_outbox WHERE status = 'pending'`)
}

// #2951: sending a Contract mails the Client, queued in the send's own
// transaction and attributed to the Staff member who sent it.
func TestPostSendContractHandler_QueuesTheEmailToTheClient(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedFilledDraft()

	reqtest.ExpectStatus(t, http.MethodPost, e.send(), http.StatusOK, e.SignedIn)

	if got, want := e.pendingNotices(), "sent "+e.Staff.ID; got != want {
		t.Fatalf("queued = %q, want %q", got, want)
	}
}

func TestPostSendContractHandler_ARefusedSendQueuesNoEmail(t *testing.T) {
	e := newEnv(t)
	e.seedFilledDraft()

	reqtest.ExpectStatus(t, http.MethodPost, e.send(), http.StatusConflict, e.SignedIn)

	if got := e.pendingNotices(); got != "" {
		t.Fatalf("queued = %q, want nothing", got)
	}
}
