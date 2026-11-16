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
// Erasure voids her Draft (#2990), so the record is stamped erased
// without it: the refusal is the guard for a Draft that outlived one.
func TestPostSendContractHandler_RefusesASendForAnErasedClient(t *testing.T) {
	e := newEnv(t)
	e.seedFilledDraft()
	e.Exec(`UPDATE clients SET erased_at = now(), erased_by_kind = 'system' WHERE id = $1`, e.Engagement.Client.ID)

	got := apierrtest.Expect(t, http.MethodPost, e.send(), http.StatusConflict, e.SignedIn)

	if _, want := clientstanding.ActSendContract.Check(clientstanding.Erased); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
	e.requireNothingSent()
}

// TestListContracts_AnErasedClientsContractIsVoidedByErasure: a Sent
// Contract leaves the Sent and Not signed views when its Client is erased,
// and sits in Voided as voided by erasure, under no Staff member's name
// (#2990, #2997).
func TestListContracts_AnErasedClientsContractIsVoidedByErasure(t *testing.T) {
	e := newEnv(t)
	e.seedContract(statusSent)
	fixture.EraseClient(t, e.DB, e.Engagement.Client.ID)

	for _, view := range []string{"", "?view=sent"} {
		if got := e.list(view).Items; len(got) != 0 {
			t.Errorf("list%s = %d Contracts, want none", view, len(got))
		}
	}
	got := e.list("?view=voided").Items
	if len(got) != 1 || !got[0].VoidedByErasure || got[0].VoidedByName != "" {
		t.Fatalf("voided view = %+v, want one Contract voided by erasure", got)
	}
	if page := e.contract(); page.Status != "voided" {
		t.Errorf("Engagement Contract status = %q, want voided", page.Status)
	}
}
