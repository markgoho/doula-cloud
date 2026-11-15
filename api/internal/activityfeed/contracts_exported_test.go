package activityfeed_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

// #2954: a contracts_exported row carries its diff, so the feed can say what
// was downloaded, as an invoices_exported row does.
func TestPracticeHandler_ContractsExportedRowCarriesItsScope(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-contracts-exported"
	practiceID := testdb.SeedPractice(t, db, "Feed Contracts Exported Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)

	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectPractice, practiceID, string(activity.ActionContractsExported), activity.StaffActor(ownerID),
		jsontext.Value(`{"view":"signed","sentFrom":"2026-10-01","sentTo":"2026-10-31","rows":12,"searched":false}`))

	got := readFeed(t, db, identityUID, practiceID)
	if len(got.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(got.Items))
	}
	var scope struct {
		View     string `json:"view"`
		SentFrom string `json:"sentFrom"`
		Rows     int    `json:"rows"`
	}
	if err := json.Unmarshal(got.Items[0].Diff, &scope); err != nil {
		t.Fatalf("the export row's diff %q is not its scope: %v", got.Items[0].Diff, err)
	}
	if scope.View != "signed" || scope.SentFrom != "2026-10-01" || scope.Rows != 12 {
		t.Errorf("scope = %+v, want signed from October 1 and 12 rows", scope)
	}
}
