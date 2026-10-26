package activitypage_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activitypage"
	"doula-cloud/api/internal/testdb"
)

// TestLatest_NamesTheNewestRowOfOneAction is #2104's read: of every row
// for a subject, the newest of the one action asked about, with its
// actor named the way the ledger names it -- and nothing at all once
// NotBefore leaves every row behind, or when the action never happened.
func TestLatest_NamesTheNewestRowOfOneAction(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Latest Practice")
	ownerID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "activitypage-latest-owner", "Renata Alvarez", []string{ownerRole}, employeeType)
	doulaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "activitypage-latest-doula", "Priya Raman", []string{doulaRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "activitypage-latest@example.com", "active")

	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "care_plan_edited", activity.StaffActor(ownerID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "care_plan_edited", activity.StaffActor(doulaID))
	// A newer row of another action is not the answer.
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "birth_plan_edited", activity.StaffActor(ownerID))

	tx := beginScopedTx(t, db, practiceID)
	query := activitypage.LatestQuery{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      "care_plan_edited",
	}
	stamp, err := activitypage.Latest(t.Context(), tx, query)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if stamp == nil || stamp.By != "Priya Raman" || stamp.At.IsZero() {
		t.Fatalf("stamp = %+v, want the newer edit, by Priya Raman, with its instant", stamp)
	}

	later := stamp.At.Add(time.Second)
	query.NotBefore = &later
	if stamp, err := activitypage.Latest(t.Context(), tx, query); err != nil || stamp != nil {
		t.Errorf("NotBefore after every row: stamp = %+v, err = %v, want nil, nil", stamp, err)
	}

	query.NotBefore = nil
	query.Action = "contract_sent"
	if stamp, err := activitypage.Latest(t.Context(), tx, query); err != nil || stamp != nil {
		t.Errorf("an action that never happened: stamp = %+v, err = %v, want nil, nil", stamp, err)
	}
}
