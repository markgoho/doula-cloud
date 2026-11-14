package activityfeed_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

func TestPracticeHandler_OnCallChangedRowCarriesItsBeforeAndAfter(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-on-call-changed"
	practiceID := testdb.SeedPractice(t, db, "Feed On Call Changed Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	actor := activity.StaffActor(ownerID)

	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectPractice, practiceID, string(activity.ActionPracticeOnCallChanged), actor,
		jsontext.Value(`{"before":{"startRule":"gestational_week","startWeek":37,"graceDays":14},"after":{"startRule":"gestational_week","startWeek":38,"graceDays":14}}`))

	got := readFeed(t, db, identityUID, practiceID)
	if len(got.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(got.Items))
	}
	var change struct {
		Before struct {
			StartWeek int `json:"startWeek"`
		} `json:"before"`
		After struct {
			StartWeek int `json:"startWeek"`
		} `json:"after"`
	}
	if err := json.Unmarshal(got.Items[0].Diff, &change); err != nil {
		t.Fatalf("the row's diff %q is not the rule's change: %v", got.Items[0].Diff, err)
	}
	if change.Before.StartWeek != 37 || change.After.StartWeek != 38 {
		t.Errorf("change = %+v, want week 37 to week 38", change)
	}
}
