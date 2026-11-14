package activityfeed_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

// #2891: a clients_exported row carries its diff, so the feed can say what
// was downloaded, as an invoices_exported row does.
func TestPracticeHandler_ClientsExportedRowCarriesItsScope(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-clients-exported"
	practiceID := testdb.SeedPractice(t, db, "Feed Clients Exported Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)

	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectPractice, practiceID, string(activity.ActionClientsExported), activity.StaffActor(ownerID),
		jsontext.Value(`{"owes":true,"order":"owed","rows":9,"searched":false}`))

	got := readFeed(t, db, identityUID, practiceID)
	if len(got.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(got.Items))
	}
	var scope struct {
		Order string `json:"order"`
		Rows  int    `json:"rows"`
	}
	if err := json.Unmarshal(got.Items[0].Diff, &scope); err != nil {
		t.Fatalf("the export row's diff %q is not its scope: %v", got.Items[0].Diff, err)
	}
	if scope.Order != "owed" || scope.Rows != 9 {
		t.Errorf("scope = %+v, want owed and 9 rows", scope)
	}
}
