package contracts_test

import (
	"encoding/json/v2"
	"net/http"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const scopeKey = "scope_of_service"

const scopeProse = "Agreement for {{client_name}}: {{scope_of_service}} at {{price}}."

func countContractEdited(t *testing.T, db *testdb.DB, engagementID string) int {
	t.Helper()
	count := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = $2`, engagementID, string(activity.ActionContractEdited))
	return count
}

func (e *env) countContractEdited() int {
	e.t.Helper()
	return countContractEdited(e.t, e.db, e.Engagement.ID)
}

// TestPutContractHandler_RecordsWhoChangedTheDraft proves #2520: a save
// that moves a merge field records contract_edited against the Engagement
// with the Staff actor, naming the fields it moved and never their values.
func TestPutContractHandler_RecordsWhoChangedTheDraft(t *testing.T) {
	e := newEnv(t)
	seedContract(t, e.db, e.Engagement.ID, statusDraft, scopeProse)

	const scope = "Two prenatal Visits, the birth, one postpartum Visit"
	reqtest.ExpectStatus(t, http.MethodPut, e.contractURL(""), http.StatusOK, e.signedIn,
		putting(contracts.MergeFieldValues{clientNameKey: testClientName, scopeKey: scope}))

	diff := assertActivityActor(t, e.db, e.Engagement.ID, activity.ActionContractEdited, e.Staff.ID)
	var parsed struct {
		Fields []string `json:"fields"`
	}
	if err := json.Unmarshal(diff, &parsed); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if !slices.Equal(parsed.Fields, []string{clientNameKey, scopeKey}) {
		t.Fatalf("diff fields = %v, want [client_name scope_of_service]", parsed.Fields)
	}
	if strings.Contains(string(diff), scope) {
		t.Fatalf("diff %s carries the scope's own words", diff)
	}
}

// TestPutContractHandler_UnchangedSaveRecordsNothing proves the choice
// recorded on #2520: a save whose values match what is stored, including
// a field cleared that was already empty, leaves no Activity row.
func TestPutContractHandler_UnchangedSaveRecordsNothing(t *testing.T) {
	e := newEnv(t)
	seedContractWithValues(t, e.db, e.Engagement.ID, contracts.MergeFieldValues{clientNameKey: testClientName, scopeKey: ""})

	reqtest.ExpectStatus(t, http.MethodPut, e.contractURL(""), http.StatusOK, e.signedIn,
		putting(contracts.MergeFieldValues{clientNameKey: testClientName}))

	if got := e.countContractEdited(); got != 0 {
		t.Fatalf("contract_edited rows = %d, want 0", got)
	}
}

// TestPutContractHandler_ClearingAFieldIsAChange proves a value removed
// from the save counts as a change to that field.
func TestPutContractHandler_ClearingAFieldIsAChange(t *testing.T) {
	e := newEnv(t)
	seedContractWithValues(t, e.db, e.Engagement.ID, contracts.MergeFieldValues{clientNameKey: testClientName})

	reqtest.ExpectStatus(t, http.MethodPut, e.contractURL(""), http.StatusOK, e.signedIn, putting(contracts.MergeFieldValues{}))

	if got := e.countContractEdited(); got != 1 {
		t.Fatalf("contract_edited rows = %d, want 1", got)
	}
}
