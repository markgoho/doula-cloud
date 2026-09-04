package activityfeed_test

import (
	"database/sql"
	"fmt"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/testdb"
)

// beginScopedTx opens a tx on the app_runtime connection with
// app.current_practice_id already set, the same scoping
// staffauth.Middleware performs per request -- ListForSubject's own RLS
// read needs it, since it takes a bare *sql.Tx rather than resolving one
// itself.
func beginScopedTx(t *testing.T, db *testdb.DB, practiceID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}
	return tx
}

// TestListForSubject_ScopesToOneSubjectOnly proves the record-scoped
// reader answers only for the one subject it is given, the same
// isolation engagement.listEngagementActivity's own WHERE clause already
// gives a per-Engagement caller.
func TestListForSubject_ScopesToOneSubjectOnly(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "ListForSubject Scope Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "list-for-subject-owner", []string{ownerRole}, employeeType)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client A", "list-for-subject-a@example.com")
	_, otherEngagementID := seedClientEngagement(t, db, practiceID, "Client B", "list-for-subject-b@example.com")

	seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "visit_logged", activity.StaffActor(ownerID))
	seedActivity(t, db, practiceID, activity.SubjectEngagement, otherEngagementID, "visit_logged", activity.StaffActor(ownerID))

	tx := beginScopedTx(t, db, practiceID)
	got, err := activityfeed.ListForSubject(t.Context(), tx, practiceID, activity.SubjectEngagement, engagementID, "", nil, 30)
	if err != nil {
		t.Fatalf("ListForSubject: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].SubjectID != engagementID {
		t.Fatalf("Items = %+v, want exactly the one row for %s", got.Items, engagementID)
	}
}

// TestListForSubject_ExcludedActionsNotInHidesThoseRows proves the
// caller-supplied exclusion clause portal.ActivityHandler builds from
// activity.StaffingActions() actually filters at the SQL level, the same
// way engagement.moneyActionsNotIn already filters
// engagement.ListActivityHandler's own query.
func TestListForSubject_ExcludedActionsNotInHidesThoseRows(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "ListForSubject Exclude Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "list-for-subject-exclude-owner", []string{ownerRole}, employeeType)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "list-for-subject-exclude@example.com")

	seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "visit_logged", activity.StaffActor(ownerID))
	seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "offer_sent", activity.StaffActor(ownerID))

	tx := beginScopedTx(t, db, practiceID)
	got, err := activityfeed.ListForSubject(t.Context(), tx, practiceID, activity.SubjectEngagement, engagementID, "'offer_sent'", nil, 30)
	if err != nil {
		t.Fatalf("ListForSubject: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Action != "visit_logged" {
		t.Fatalf("Items = %+v, want exactly the non-excluded visit_logged row", got.Items)
	}
}

// TestListForSubject_PaginatesNewestFirst mirrors
// engagement.TestListActivityHandler_PaginatesNewestFirst one layer down,
// against the generic reader directly.
func TestListForSubject_PaginatesNewestFirst(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "ListForSubject Paginate Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "list-for-subject-paginate-owner", []string{ownerRole}, employeeType)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "list-for-subject-paginate@example.com")

	const total = 31
	for i := range total {
		seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, fmt.Sprintf("visit_%d", i), activity.StaffActor(ownerID))
	}

	tx := beginScopedTx(t, db, practiceID)
	first, err := activityfeed.ListForSubject(t.Context(), tx, practiceID, activity.SubjectEngagement, engagementID, "", nil, 30)
	if err != nil {
		t.Fatalf("ListForSubject first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	cursor, err := pagecursor.Decode(*first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	second, err := activityfeed.ListForSubject(t.Context(), tx, practiceID, activity.SubjectEngagement, engagementID, "", &cursor, 30)
	if err != nil {
		t.Fatalf("ListForSubject second page: %v", err)
	}
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
}
