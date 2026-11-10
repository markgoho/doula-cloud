package client_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// keptRecord is the survivor's name in every merge below.
const keptRecord = "Kept Record"

// mergedPair seeds a survivor the Staff member is on and a full record,
// then merges the record into it.
func (e *env) mergedPair() (survivorID, absorbedID string) {
	e.T.Helper()
	survivorID = e.attachedClient(keptRecord, "kept@example.com")
	absorbedID = seedFullClient(e.T, e.DB, e.Practice.ID, e.Staff.ID)
	e.merge(absorbedID, client.MergeRequest{GivenName: keptRecord, OtherClientID: survivorID}, http.StatusOK)
	return survivorID, absorbedID
}

// TestErase_SurvivorAndAbsorbedRecordShareOneErasure is the duplicated
// transition the audit found: redactRecord stamped the survivor with the
// clock's instant and redact_absorbed_client stamped the record merged
// into it with now(), so under a moved clock one erasure carried two
// instants. Both now run redact_client_record (00171), and the absorbed
// record takes the survivor's instant and author.
func TestErase_SurvivorAndAbsorbedRecordShareOneErasure(t *testing.T) {
	moved := time.Date(2031, time.March, 4, 10, 30, 0, 0, time.UTC)
	e := newEnv(t, signedInAs(fixture.Owner()), fixture.At(moved))
	survivorID, absorbedID := e.mergedPair()

	e.erasure(survivorID)

	const erasure = `SELECT erased_at, standing::text, erased_by_kind::text, erased_by_staff_id::text FROM clients WHERE id = $1`
	for id, standing := range map[string]string{survivorID: "erased", absorbedID: "merged_then_erased"} {
		var at time.Time
		var gotStanding, kind, by string
		if err := e.DB.Admin.QueryRowContext(t.Context(), erasure, id).Scan(&at, &gotStanding, &kind, &by); err != nil {
			t.Fatalf("read erasure of %s: %v", id, err)
		}
		if !at.Equal(moved) || gotStanding != standing || kind != "staff" || by != e.Staff.ID {
			t.Errorf("record %s = erased %v, %s, by %s %s; want erased %v, %s, by staff %s", id, at, gotStanding, kind, by, moved, standing, e.Staff.ID)
		}
	}
}

// TestEraseHandler_RefusesATombstone: a merged record is erased only by
// its survivor's erasure. Before the gate, a direct erasure of the
// tombstone was answered by the lock's empty read.
func TestEraseHandler_RefusesATombstone(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	_, absorbedID := e.mergedPair()

	got := apierrtest.Expect(t, http.MethodPost, e.clientURL(absorbedID, "/erasure"), http.StatusConflict, e.SignedIn)

	if _, want := clientstanding.ActErase.Check(clientstanding.Merged); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
	if standing := testdb.Scalar[string](t, e.DB.Admin, `SELECT standing::text FROM clients WHERE id = $1`, absorbedID); standing != "merged" {
		t.Errorf("tombstone standing = %q, want merged and untouched", standing)
	}
}

// TestDetailHandler_CarriesTheStanding: the one field a screen reads to
// decide whether the record can take a write, on the tombstone's short
// answer as on the full one.
func TestDetailHandler_CarriesTheStanding(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	survivorID, absorbedID := e.mergedPair()

	read := func(id string) client.DetailResponse {
		return reqtest.Expect[client.DetailResponse](t, http.MethodGet, e.clientURL(id, ""), http.StatusOK, e.SignedIn)
	}
	if got := read(survivorID).Standing; got != clientstanding.Live {
		t.Errorf("survivor standing = %q, want live", got)
	}
	if got := read(absorbedID).Standing; got != clientstanding.Merged {
		t.Errorf("tombstone standing = %q, want merged", got)
	}
	e.erasure(survivorID)
	if got := read(survivorID).Standing; got != clientstanding.Erased {
		t.Errorf("erased survivor standing = %q, want erased", got)
	}
	if got := read(absorbedID).Standing; got != clientstanding.MergedThenErased {
		t.Errorf("erased tombstone standing = %q, want merged_then_erased", got)
	}
}

// TestEditHandler_RefusesAnErasedRecordInWords: the gate's sentence, not
// the old lowercase one, and the schema refuses the same write on its
// own -- clients_update (00171) admits only a live record.
func TestEditHandler_RefusesAnErasedRecordInWords(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	clientID := e.attachedClient("Gone Record", "gone@example.com")
	e.erasure(clientID)

	got := apierrtest.Expect(t, http.MethodPut, e.clientURL(clientID, ""), http.StatusConflict, e.SignedIn,
		reqtest.JSON(client.EditRequest{GivenName: "Back Again"}))
	if _, want := clientstanding.ActEdit.Check(clientstanding.Erased); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}

	tx := e.DB.BeginAs(t, testdb.Practice(e.Practice.ID))
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(t.Context(), `UPDATE clients SET given_name = 'Back Again' WHERE id = $1`, clientID)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 0 {
		t.Errorf("app_runtime updated %d erased rows, want 0", n)
	}
}
