package activityfeed_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

func TestPracticeHandler_NamesAnOperatorAsThePersonWithDoulaCloud(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-operator"
	practiceID := testdb.SeedPractice(t, db, "Feed Operator Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	testdb.SeedOperatorActivity(t, db, practiceID, activity.SubjectPractice, practiceID, "credits_refunded", "Mark Goho",
		time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC))

	got := readFeed(t, db, identityUID, practiceID)
	if len(got.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(got.Items))
	}
	if got.Items[0].ActorKind != "operator" || got.Items[0].ActorName != "Mark Goho at DoulaCloud" {
		t.Fatalf("actor = (%q, %q), want (operator, Mark Goho at DoulaCloud)", got.Items[0].ActorKind, got.Items[0].ActorName)
	}
}
