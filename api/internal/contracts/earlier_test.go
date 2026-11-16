package contracts_test

import (
	"net/http"
	"slices"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// TestGetContract_NamesTheCurrentContractAndTheVoidedOnesItReplaced proves
// a Staff read carries the current Contract's id and each voided Contract
// before it, with the day, who voided it and the granted request's reason.
func TestGetContract_NamesTheCurrentContractAndTheVoidedOnesItReplaced(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	asking := e.Colleague(fixture.Doula(), fixture.StaffName("Kimiko Ito"))
	e.readyToDraft(mergeFieldProse)
	voidedID, _ := seedSignedContract(t, e.DB, e.Engagement.ID)
	seedVoidRequest(t, e.DB, e.Engagement.ID, asking.ID, "signed the wrong template")
	reqtest.ExpectStatus(t, http.MethodPost, e.voidURL(), http.StatusOK, e.SignedIn)
	reqtest.ExpectStatus(t, http.MethodPost, e.contractURL(""), http.StatusCreated, e.SignedIn)

	out := e.contract()

	if out.ID == "" || out.ID == voidedID {
		t.Fatalf("id = %q, want the new Draft's id and not %q", out.ID, voidedID)
	}
	if len(out.Earlier) != 1 {
		t.Fatalf("earlier = %+v, want the one voided Contract", out.Earlier)
	}
	got := out.Earlier[0]
	if got.ID != voidedID || got.VoidedBy != e.Staff.Name || got.RequestedBy != "Kimiko Ito" || got.Reason != "signed the wrong template" || got.VoidedAt.IsZero() {
		t.Fatalf("earlier[0] = %+v, want %s voided by %s on Kimiko Ito's request", got, voidedID, e.Staff.Name)
	}
}

// TestGetContract_AnEarlierContractVoidedByErasureSaysSo proves history
// carries the cause, not a Staff member's name, for a Contract an erasure
// voided (#2997).
func TestGetContract_AnEarlierContractVoidedByErasureSaysSo(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	earlierID, _ := seedSignedContract(t, e.DB, e.Engagement.ID)
	if _, err := e.DB.Admin.ExecContext(t.Context(), `UPDATE contracts SET status = 'voided', voided_at = now(), voided_by_erasure = true WHERE id = $1`, earlierID); err != nil {
		t.Fatalf("void earlier Contract: %v", err)
	}
	e.seedContract(statusDraft)

	out := e.contract()

	if len(out.Earlier) != 1 || out.Earlier[0].ID != earlierID || !out.Earlier[0].VoidedByErasure || out.Earlier[0].VoidedBy != "" {
		t.Fatalf("earlier = %+v, want the one Contract voided by erasure and no name", out.Earlier)
	}
}

// TestGetContract_ACurrentVoidedContractIsNotAlsoEarlier proves the
// Contract the section already draws is not listed again as history.
func TestGetContract_ACurrentVoidedContractIsNotAlsoEarlier(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	voidedID, _ := seedSignedContract(t, e.DB, e.Engagement.ID)
	reqtest.ExpectStatus(t, http.MethodPost, e.voidURL(), http.StatusOK, e.SignedIn)

	out := e.contract()

	if out.ID != voidedID || len(out.Earlier) != 0 {
		t.Fatalf("id = %q, earlier = %+v, want the voided Contract's id and no history", out.ID, out.Earlier)
	}
}

// newEnvWithDraft is an Owner's env with a Draft made through the API.
func newEnvWithDraft(t *testing.T) *env {
	t.Helper()
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.readyToDraft(mergeFieldProse)
	reqtest.ExpectStatus(t, http.MethodPost, e.contractURL(""), http.StatusCreated, e.SignedIn)
	return e
}

// TestPostContractHandler_RecordsWhoMadeIt proves a new Draft is stamped
// with the Staff member who made it, and the list says so.
func TestPostContractHandler_RecordsWhoMadeIt(t *testing.T) {
	e := newEnvWithDraft(t)

	out := e.list("?view=draft")

	if len(out.Items) != 1 || out.Items[0].MadeByName != e.Staff.Name {
		t.Fatalf("items = %+v, want the Draft made by %s", out.Items, e.Staff.Name)
	}
}

// TestListContracts_NamesWhoVoidedItAndTheDoulasByID proves a voided row
// says who voided it and a row's Doulas come with their ids, in the order
// of their names.
func TestListContracts_NamesWhoVoidedItAndTheDoulasByID(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	first := e.Colleague(fixture.Doula(), fixture.StaffName("Amy Adams"))
	second := e.Colleague(fixture.Doula(), fixture.StaffName("Zed Zimmer"))
	_, contractID := e.seedListed(statusVoided, listBase, fixture.WithDoula(second), fixture.WithDoula(first))
	e.Exec(`UPDATE contracts SET voided_by = $2 WHERE id = $1`, contractID, e.Staff.ID)

	out := e.list("?view=voided")

	if len(out.Items) != 1 {
		t.Fatalf("items = %+v, want the one voided row", out.Items)
	}
	item := out.Items[0]
	if item.VoidedByName != e.Staff.Name {
		t.Fatalf("voidedByName = %q, want %q", item.VoidedByName, e.Staff.Name)
	}
	if !slices.Equal(item.DoulaIDs, []string{first.ID, second.ID}) || !slices.Equal(item.DoulaNames, []string{first.Name, second.Name}) {
		t.Fatalf("doulas = %v %v, want Amy then Zed with their ids", item.DoulaNames, item.DoulaIDs)
	}
}
