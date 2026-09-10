package engagement

import (
	"context"
	"database/sql"
	"doula-cloud/api/internal/activity"
	"encoding/json"
	"errors"
	"fmt"
)

// activateFromIntake is ADR-0015's `intake -> active` move, and the only
// place it happens. Both ways of reaching it run this function: the
// manual PATCH .../engagements/{engagementId}/status (#253) and the
// automatic move a scheduled Visit makes (#895). One body, so the pair of
// audit rows the move leaves behind -- the engagement_events
// 'status_changed' row and the ActionCarePhaseChanged activity entry --
// can never differ depending on which door the Engagement came through.
//
// The write itself is the guard. `WHERE status = 'intake'` decides
// atomically whether this caller is the one making the move, so a
// concurrent second write finds no row and reports moved=false rather
// than laying down a second pair of audit rows claiming the same
// transition. That is the ordinary, expected answer on the automatic
// path -- every Visit scheduled after the first one finds the Engagement
// already 'active' -- and it is what makes ADR-0015's "one-way, one-time"
// rule hold without a separate read-then-check.
//
// ending_reason and ending_note are read back rather than written: this
// move does not touch them, and recordStatusEvent wants both sides of
// every fact the row carries, so the value it returns stands on both
// sides unchanged.
func activateFromIntake(ctx context.Context, tx *sql.Tx, practiceID, engagementID, actorStaffID string) (moved bool, err error) {
	var endingReason, endingNote *string
	err = tx.QueryRowContext(ctx,
		`UPDATE engagements SET status = $1 WHERE id = $2 AND status = $3
		  RETURNING ending_reason, ending_note`,
		StatusActive, engagementID, StatusIntake,
	).Scan(&endingReason, &endingNote)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("engagement: activate from intake: %w", err)
	}

	if err := recordStatusEvent(ctx, tx, statusEvent{
		practiceID:           practiceID,
		engagementID:         engagementID,
		previousStatus:       StatusIntake,
		status:               StatusActive,
		previousEndingReason: endingReason,
		endingReason:         endingReason,
		previousEndingNote:   endingNote,
		endingNote:           endingNote,
		actorStaffID:         &actorStaffID,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, err
	}

	diff, err := json.Marshal(map[string]any{"statusBefore": StatusIntake, "statusAfter": StatusActive})
	if err != nil {
		// coverage:ignore reason: marshaling two constant strings never fails
		return false, fmt.Errorf("engagement: activation diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionCarePhaseChanged),
		Diff:        diff,
		Actor:       activity.StaffActor(actorStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("engagement: record activation activity: %w", err)
	}
	return true, nil
}

// ActivateOnVisitScheduled is ADR-0015's one automatic status move:
// "`intake` -> `active` happens by itself, the first time a Visit is
// scheduled." Every write that leaves a Visit on this Engagement with a
// non-null scheduled_at calls it, in the same transaction as the Visit
// write, and an Engagement that is not at 'intake' is left exactly as it
// is -- no field write, no audit row. Nothing here is idempotency-keyed
// or retried: the conditional UPDATE inside activateFromIntake is what
// makes a second call a no-op.
//
// actorStaffID is the Staff member who made the scheduling request, not
// whoever the Visit is assigned to. ADR-0015 is explicit that "the event
// records the person who scheduled, not a null system actor -- a doula
// did that", and an Admin booking a Visit for a colleague is the person
// who scheduled it.
//
// This move carries no role gate of its own, and that is ADR-0015's
// ruling rather than an omission: the only cell its permissions table
// spends on this transition is `intake -> active (manual)`. The
// automatic move is a consequence of scheduling a Visit, so it is gated
// by whoever may schedule one -- which for the Visit write paths is
// staffauth.AttachingWrite plus, where a colleague is named, the
// assignment's own rule. A contractor Doula who may schedule a Visit
// therefore activates the Engagement by scheduling it, while still being
// refused the manual move; she is not closing the Practice's record,
// only saying care is booked.
func ActivateOnVisitScheduled(ctx context.Context, tx *sql.Tx, practiceID, engagementID, actorStaffID string) error {
	_, err := activateFromIntake(ctx, tx, practiceID, engagementID, actorStaffID)
	return err
}
