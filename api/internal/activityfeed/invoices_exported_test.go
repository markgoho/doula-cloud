package activityfeed_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

// #2871: an invoices_exported row carries its diff, so the feed can say
// what was downloaded; no other row on the feed sends one.
func TestPracticeHandler_InvoicesExportedRowCarriesItsScope(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-invoices-exported"
	practiceID := testdb.SeedPractice(t, db, "Feed Invoices Exported Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	actor := activity.StaffActor(ownerID)

	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectPractice, practiceID, string(activity.ActionInvoicesExported), actor,
		jsontext.Value(`{"view":"unpaid","sentFrom":"2026-10-01","sentTo":"2026-10-31","rows":14,"searched":true}`))
	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectPractice, practiceID, string(activity.ActionPracticeNameChanged), actor,
		jsontext.Value(`{"name":"Renamed"}`))

	got := readFeed(t, db, identityUID, practiceID)
	if len(got.Items) != 2 {
		t.Fatalf("Items = %d, want 2", len(got.Items))
	}
	renamed, exported := got.Items[0], got.Items[1]
	if len(renamed.Diff) != 0 {
		t.Errorf("a row of another action sends diff %s, want none", renamed.Diff)
	}
	var scope struct {
		View string `json:"view"`
		Rows int    `json:"rows"`
	}
	if err := json.Unmarshal(exported.Diff, &scope); err != nil {
		t.Fatalf("the export row's diff %q is not its scope: %v", exported.Diff, err)
	}
	if scope.View != "unpaid" || scope.Rows != 14 {
		t.Errorf("scope = %+v, want unpaid and 14 rows", scope)
	}
}
