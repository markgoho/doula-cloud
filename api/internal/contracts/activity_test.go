package contracts_test

import (
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

// assertActivityActor proves an Engagement-scoped activity row for the
// given action exists and names wantStaffID as its actor, and hands the
// row's diff back for a caller that asserts more (#1017). Five inline
// blocks, spread across four tests in this package, repeated this
// SELECT/Scan/compare shape, each naming its action as a bare string
// literal, where a typo read as a missing row rather than a compile
// error -- taking the typed activity.EngagementAction is the point of
// the extraction, and activity.SubjectEngagement replaces the
// 'engagement' literal for the same reason.
//
// It returns the diff rather than returning nothing (the shape #1017
// suggested) so that a caller asserting the diff too -- amount_test.go's
// override, contract_test.go's created/priced pair -- still reads the
// row with one query, exactly as its inline block did.
func assertActivityActor(t *testing.T, db *testdb.DB, engagementID string, action activity.EngagementAction, wantStaffID string) []byte {
	t.Helper()
	var actorStaffID string
	var diff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id, diff FROM activity WHERE subject_kind = $1 AND subject_id = $2 AND action = $3`,
		activity.SubjectEngagement, engagementID, string(action),
	).Scan(&actorStaffID, &diff); err != nil {
		t.Fatalf("query %s activity: %v", action, err)
	}
	if actorStaffID != wantStaffID {
		t.Fatalf("%s actor_staff_id = %q, want %q", action, actorStaffID, wantStaffID)
	}
	return diff
}
