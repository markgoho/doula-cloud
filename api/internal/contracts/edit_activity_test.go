package contracts_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

const scopeKey = "scope_of_service"

const scopeProse = "Agreement for {{client_name}}: {{scope_of_service}} at {{price}}."

func countContractEdited(t *testing.T, db *testdb.DB, engagementID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = $2`,
		engagementID, string(activity.ActionContractEdited),
	).Scan(&count); err != nil {
		t.Fatalf("count contract_edited rows: %v", err)
	}
	return count
}

// TestPutContractHandler_RecordsWhoChangedTheDraft proves #2520: a save
// that moves a merge field records contract_edited against the Engagement
// with the Staff actor, naming the fields it moved and never their values.
func TestPutContractHandler_RecordsWhoChangedTheDraft(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-records-edit"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, scopeProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	const scope = "Two prenatal Visits, the birth, one postpartum Visit"
	resp := putContract(t, srv, session, practiceID, engagementID,
		contracts.MergeFieldValues{clientNameKey: testClientName, scopeKey: scope})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	diff := assertActivityActor(t, db, engagementID, activity.ActionContractEdited, staffID)
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
	db := testdb.New(t)
	const uid = "put-unchanged-edit"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractWithValues(t, db, engagementID, contracts.MergeFieldValues{clientNameKey: testClientName, scopeKey: ""})

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContract(t, srv, session, practiceID, engagementID, contracts.MergeFieldValues{clientNameKey: testClientName})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := countContractEdited(t, db, engagementID); got != 0 {
		t.Fatalf("contract_edited rows = %d, want 0", got)
	}
}

// TestPutContractHandler_ClearingAFieldIsAChange proves a value removed
// from the save counts as a change to that field.
func TestPutContractHandler_ClearingAFieldIsAChange(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-cleared-edit"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractWithValues(t, db, engagementID, contracts.MergeFieldValues{clientNameKey: testClientName})

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContract(t, srv, session, practiceID, engagementID, contracts.MergeFieldValues{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := countContractEdited(t, db, engagementID); got != 1 {
		t.Fatalf("contract_edited rows = %d, want 1", got)
	}
}
