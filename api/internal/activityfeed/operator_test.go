package activityfeed_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/testdb"
)

func TestPracticeHandler_NamesAnOperatorAsThePersonWithDoulaCloud(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-operator"
	practiceID := testdb.SeedPractice(t, db, "Feed Operator Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	testdb.SeedOperatorActivity(t, db, practiceID, activity.SubjectPractice, practiceID, "credits_refunded", "Mark Goho",
		time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(got.Items))
	}
	if got.Items[0].ActorKind != "operator" || got.Items[0].ActorName != "Mark Goho at DoulaCloud" {
		t.Fatalf("actor = (%q, %q), want (operator, Mark Goho at DoulaCloud)", got.Items[0].ActorKind, got.Items[0].ActorName)
	}
}
