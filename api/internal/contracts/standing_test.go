package contracts_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestPostContractHandler_RefusesAContractForAnErasedClient: a Contract
// created after erasure would snapshot the placeholder name as its merge
// values, so it is refused before anything is resolved or written.
func TestPostContractHandler_RefusesAContractForAnErasedClient(t *testing.T) {
	e := newEnv(t)
	fixture.EraseClient(t, e.DB, e.Engagement.Client.ID)

	got := apierrtest.Expect(t, http.MethodPost, e.contractURL(""), http.StatusConflict, e.SignedIn)

	if _, want := clientstanding.ActCreateContract.Check(clientstanding.Erased); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
	if n := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM contracts WHERE engagement_id = $1`, e.Engagement.ID); n != 0 {
		t.Errorf("contracts = %d, want 0", n)
	}
}

// TestPostSendContractHandler_RefusesASendForAnErasedClient: the draft
// stays a draft, and the refusal names the erasure rather than the
// missing portal invite or merge values an erased record also lacks.
func TestPostSendContractHandler_RefusesASendForAnErasedClient(t *testing.T) {
	e := newEnv(t)
	e.seedFilledDraft()
	fixture.EraseClient(t, e.DB, e.Engagement.Client.ID)

	got := apierrtest.Expect(t, http.MethodPost, e.send(), http.StatusConflict, e.SignedIn)

	if _, want := clientstanding.ActSendContract.Check(clientstanding.Erased); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
	e.requireNothingSent()
}
