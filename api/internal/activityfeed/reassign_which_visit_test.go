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

// #2666: a visit_reassigned row carries the instants that say which Visit
// moved, so two reassignments of one Client's Visits do not read alike.
// A row written before #2666, and every other action, carries neither.
func TestPracticeHandler_ReassignmentRowsSayWhichVisitMoved(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-which-visit"
	practiceID := testdb.SeedPractice(t, db, "Feed Which Visit Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Feed Client", "feed-which-visit@example.com", "active")
	action := string(activity.ActionVisitReassigned)
	actor := activity.StaffActor(ownerID)

	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectEngagement, engagementID, action, actor,
		json.RawMessage(`{"visitScheduledAt":null,"visitCreatedAt":"2026-11-02T10:11:43.648104Z"}`))
	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectEngagement, engagementID, action, actor,
		json.RawMessage(`{"visitScheduledAt":"2026-11-13T21:00:00Z","visitCreatedAt":"2026-11-02T10:11:43.327441Z"}`))
	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectEngagement, engagementID, action, actor,
		json.RawMessage(`{"assignedStaffIdBefore":"a","assignedStaffIdAfter":"b"}`))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, err = %v", resp.StatusCode, err)
	}
	if len(got.Items) != 3 {
		t.Fatalf("Items = %d, want 3", len(got.Items))
	}

	old, dated, undated := got.Items[0], got.Items[1], got.Items[2]
	if old.VisitScheduledAt != nil || old.VisitAddedAt != nil {
		t.Errorf("an old row says %v / %v, want neither", old.VisitScheduledAt, old.VisitAddedAt)
	}
	want := time.Date(2026, 11, 13, 21, 0, 0, 0, time.UTC)
	if dated.VisitScheduledAt == nil || !dated.VisitScheduledAt.Equal(want) || dated.VisitAddedAt == nil {
		t.Errorf("dated row = %v / %v, want %v and an added instant", dated.VisitScheduledAt, dated.VisitAddedAt, want)
	}
	if undated.VisitScheduledAt != nil || undated.VisitAddedAt == nil {
		t.Errorf("undated row = %v / %v, want no date and an added instant", undated.VisitScheduledAt, undated.VisitAddedAt)
	}
}
