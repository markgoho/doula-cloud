package engagement

import (
	"context"
	"database/sql"
	"fmt"
)

// statusEvent is one row TransitionHandler writes to engagement_events
// (00090) -- ADR-0015's audit table, shaped on
// practice_membership_events (00039): both sides of the fact that
// changed, plus a nullable actor. actorStaffID is a pointer rather than
// a bare string because a future automation this table anticipates (the
// standing rule engagement_events' own migration names) may record no
// human actor at all; every writer TransitionHandler drives today always
// has one.
type statusEvent struct {
	practiceID           string
	engagementID         string
	previousStatus       string
	status               string
	previousEndingReason *string
	endingReason         *string
	previousEndingNote   *string
	endingNote           *string
	actorStaffID         *string
}

// recordStatusEvent writes one 'status_changed' engagement_events row.
// Exported logic stays local to this file rather than activity's own
// Record: this is ADR-0015's own staff-only audit table, distinct from
// the (portal-visible-by-default) activity ledger TransitionHandler also
// writes to for the two moves ADR-0022 names -- see TransitionHandler's
// own doc comment. #293's own writer (birth_outcome) will call this
// table with event_type = 'birth_outcome_recorded' the same way.
func recordStatusEvent(ctx context.Context, tx *sql.Tx, e statusEvent) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO engagement_events
		     (practice_id, engagement_id, event_type, previous_status, status,
		      previous_ending_reason, ending_reason, previous_ending_note, ending_note, actor_staff_id)
		 VALUES ($1, $2, 'status_changed', $3, $4, $5, $6, $7, $8, $9)`,
		e.practiceID, e.engagementID, e.previousStatus, e.status,
		e.previousEndingReason, e.endingReason, e.previousEndingNote, e.endingNote, e.actorStaffID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("engagement: record status event: %w", err)
	}
	return nil
}

// outcomeEvent is one row RecordBirthOutcomeHandler writes to
// engagement_events -- the same table and the same both-sides shape
// statusEvent uses, under event_type 'birth_outcome_recorded'. One event
// type covers recording and correcting alike: a row whose previous side
// is null is a first recording, and one whose previous side is set is a
// correction, so the distinction is read off the row rather than
// asserted twice.
type outcomeEvent struct {
	practiceID               string
	engagementID             string
	previousBirthOutcome     *string
	birthOutcome             string
	previousPregnancyEndedOn *string
	pregnancyEndedOn         *string
	actorStaffID             *string
}

// recordOutcomeEvent writes one 'birth_outcome_recorded'
// engagement_events row.
func recordOutcomeEvent(ctx context.Context, tx *sql.Tx, e outcomeEvent) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO engagement_events
		     (practice_id, engagement_id, event_type,
		      previous_birth_outcome, birth_outcome,
		      previous_pregnancy_ended_on, pregnancy_ended_on, actor_staff_id)
		 VALUES ($1, $2, 'birth_outcome_recorded', $3, $4, $5::date, $6::date, $7)`,
		e.practiceID, e.engagementID,
		e.previousBirthOutcome, e.birthOutcome,
		e.previousPregnancyEndedOn, e.pregnancyEndedOn, e.actorStaffID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("engagement: record outcome event: %w", err)
	}
	return nil
}
