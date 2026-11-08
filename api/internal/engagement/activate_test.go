package engagement_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// ActivateFromIntake is ADR-0015's intake -> active move. Its manual
// door is transition_test.go's subject; these tests drive it directly
// against a transaction, which is how the automatic door reaches it
// (#895) and the only way to assert what it does when the Engagement is
// already past 'intake' -- a state TransitionHandler answers for itself
// before ever calling in. What the two Visit endpoints do with it is
// package visit's own tests.

// activationRows reads back everything the move is supposed to leave
// behind, in one place: the Engagement's status, and the count and actor
// of its care_phase_changed activity entries -- the one record of the
// move since #1423 folded engagement_events' 'status_changed' row into
// it.
type activationRows struct {
	status           string
	carePhaseEntries int
	carePhaseActor   *string
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
		`SELECT count(*), max(actor_staff_id::text) FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'care_phase_changed'`,
		engagementID,
	).Scan(&got.carePhaseEntries, &got.carePhaseActor); err != nil {
		t.Fatalf("read care phase entries: %v", err)
	}
	return got
}

// The move itself: an Engagement at 'intake' reaches 'active' and leaves
// both of ADR-0015's records behind, each naming the Staff member who
// scheduled rather than a null system actor.
func TestActivateFromIntake_MovesIntakeToActive(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	e := fixture.Engagement(t, db, p, fixture.Status(fixture.StatusIntake))

	tx := db.BeginAs(t, testdb.Practice(p.ID))
	moved, err := engagement.ActivateFromIntake(t.Context(), tx, p.ID, e.ID, doula.ID)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !moved {
		t.Fatal("moved = false, want true: this call is the one that made the move")
	}

	got := readActivationRows(t, tx, e.ID)
	if got.status != engagement.StatusActive {
		t.Fatalf("status = %q, want %q", got.status, engagement.StatusActive)
	}
	if got.carePhaseEntries != 1 {
		t.Fatalf("care_phase_changed entries = %d, want 1", got.carePhaseEntries)
	}
	if got.carePhaseActor == nil || *got.carePhaseActor != doula.ID {
		t.Fatalf("care_phase_changed actor = %v, want the scheduling Staff member %q", got.carePhaseActor, doula.ID)
	}
}

// One-way and one-time: an Engagement that is not at 'intake' is left
// exactly as it is, and nothing is written -- no second status move for a
// second scheduled Visit, and never a reopening of a completed record.
func TestActivateFromIntake_LeavesAnEngagementPastIntakeAlone(t *testing.T) {
	for _, status := range []string{engagement.StatusActive, engagement.StatusCompleted} {
		t.Run(status, func(t *testing.T) {
			db := testdb.New(t)
			p := fixture.Practice(t, db)
			doula := fixture.Staff(t, db, p)
			e := fixture.Engagement(t, db, p, fixture.Status(status))

			tx := db.BeginAs(t, testdb.Practice(p.ID))
			moved, err := engagement.ActivateFromIntake(t.Context(), tx, p.ID, e.ID, doula.ID)
			if err != nil {
				t.Fatalf("activate: %v", err)
			}
			if moved {
				t.Fatalf("moved = true on a %q Engagement, want false: the move is one-way and one-time", status)
			}

			got := readActivationRows(t, tx, e.ID)
			if got.status != status {
				t.Fatalf("status = %q, want it untouched at %q", got.status, status)
			}
			if got.carePhaseEntries != 0 {
				t.Fatalf("care_phase_changed entries = %d, want 0", got.carePhaseEntries)
			}
		})
	}
}
