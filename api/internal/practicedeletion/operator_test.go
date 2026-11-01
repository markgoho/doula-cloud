package practicedeletion_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/testdb"
)

func TestWorker_FinalizeKeepsAnOperatorActivityRowAndTheActItPointsAt(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "owner-finalize-operator", []string{ownerRole}, "employee")
	actID := testdb.SeedOperatorActivity(t, db, practiceID, activity.SubjectPractice, practiceID, "founding_grant_issued", "Mark Goho",
		time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC))
	markPendingDeletion(t, db, practiceID, time.Now())

	seedOutboxRow(t, db, practiceID, "finalize", time.Now().Add(-time.Minute))
	runWorker(t, db, newTestWorker(&mail.FakeSender{}))

	var deleted bool
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT deleted_at IS NOT NULL FROM practices WHERE id = $1`, practiceID).Scan(&deleted); err != nil || !deleted {
		t.Fatalf("practice deleted = %v, err %v, want finalized", deleted, err)
	}
	var activityRows, actRows int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE practice_id = $1 AND actor_kind = 'operator' AND actor_operator_act_id = $2`, practiceID, actID,
	).Scan(&activityRows); err != nil {
		t.Fatalf("count activity: %v", err)
	}
	if err := db.Ops.QueryRowContext(t.Context(), `SELECT count(*) FROM operator_acts WHERE id = $1`, actID).Scan(&actRows); err != nil {
		t.Fatalf("count operator_acts: %v", err)
	}
	if activityRows != 1 || actRows != 1 {
		t.Fatalf("after finalization: %d operator activity rows, %d operator_acts rows; want 1 and 1", activityRows, actRows)
	}
}
