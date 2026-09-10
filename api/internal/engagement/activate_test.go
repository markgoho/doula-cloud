package engagement_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// ActivateFromIntake is ADR-0015's intake -> active move. Its manual
// door is transition_test.go's subject; these tests drive it directly
// against a transaction, which is how the automatic door reaches it
// (#895) and the only way to assert what it does when the Engagement is
// already past 'intake' -- a state TransitionHandler answers for itself
// before ever calling in. What the two Visit endpoints do with it is
// package visit's own tests.

// beginPracticeTx opens an app-role transaction scoped to practiceID the
// same way staffauth.Middleware scopes a request's own, so RLS behaves
// for these calls exactly as it does behind a route.
func beginPracticeTx(t *testing.T, db *testdb.DB, practiceID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	return tx
}

// activationRows reads back everything the move is supposed to leave
// behind, in one place: the Engagement's status, the count and actor of
// its 'status_changed' engagement_events rows, and the count of
// care_phase_changed activity entries.
type activationRows struct {
	status           string
	statusEvents     int
	statusEventActor *string
	carePhaseEntries int
}

func readActivationRows(t *testing.T, tx *sql.Tx, engagementID string) activationRows {
	t.Helper()
	var got activationRows
	if err := tx.QueryRowContext(t.Context(),
		`SELECT status::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&got.status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*), max(actor_staff_id::text) FROM engagement_events
		  WHERE engagement_id = $1 AND event_type = 'status_changed'`, engagementID,
	).Scan(&got.statusEvents, &got.statusEventActor); err != nil {
		t.Fatalf("read status events: %v", err)
	}
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'care_phase_changed'`,
		engagementID,
	).Scan(&got.carePhaseEntries); err != nil {
		t.Fatalf("read care phase entries: %v", err)
	}
	return got
}

// The move itself: an Engagement at 'intake' reaches 'active' and leaves
// both of ADR-0015's records behind, each naming the Staff member who
// scheduled rather than a null system actor.
func TestActivateFromIntake_MovesIntakeToActive(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "auto-activating-doula", []string{doulaRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Hannah Sorensen", "hannah@example.com", engagement.StatusIntake)

	tx := beginPracticeTx(t, db, practiceID)
	moved, err := engagement.ActivateFromIntake(t.Context(), tx, practiceID, engagementID, staffID)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !moved {
		t.Fatal("moved = false, want true: this call is the one that made the move")
	}

	got := readActivationRows(t, tx, engagementID)
	if got.status != engagement.StatusActive {
		t.Fatalf("status = %q, want %q", got.status, engagement.StatusActive)
	}
	if got.statusEvents != 1 {
		t.Fatalf("status_changed events = %d, want 1", got.statusEvents)
	}
	if got.statusEventActor == nil || *got.statusEventActor != staffID {
		t.Fatalf("status event actor = %v, want the scheduling Staff member %q", got.statusEventActor, staffID)
	}
	if got.carePhaseEntries != 1 {
		t.Fatalf("care_phase_changed entries = %d, want 1", got.carePhaseEntries)
	}
}

// One-way and one-time: an Engagement that is not at 'intake' is left
// exactly as it is, and nothing is written -- no second status move for a
// second scheduled Visit, and never a reopening of a completed record.
func TestActivateFromIntake_LeavesAnEngagementPastIntakeAlone(t *testing.T) {
	for _, status := range []string{engagement.StatusActive, engagement.StatusCompleted} {
		t.Run(status, func(t *testing.T) {
			db := testdb.New(t)
			practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "doula-at-"+status, []string{doulaRole}, employeeType)
			_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client "+status, status+"-auto@example.com", status)

			tx := beginPracticeTx(t, db, practiceID)
			moved, err := engagement.ActivateFromIntake(t.Context(), tx, practiceID, engagementID, staffID)
			if err != nil {
				t.Fatalf("activate: %v", err)
			}
			if moved {
				t.Fatalf("moved = true on a %q Engagement, want false: the move is one-way and one-time", status)
			}

			got := readActivationRows(t, tx, engagementID)
			if got.status != status {
				t.Fatalf("status = %q, want it untouched at %q", got.status, status)
			}
			if got.statusEvents != 0 {
				t.Fatalf("status_changed events = %d, want 0", got.statusEvents)
			}
			if got.carePhaseEntries != 0 {
				t.Fatalf("care_phase_changed entries = %d, want 0", got.carePhaseEntries)
			}
		})
	}
}
