package engagement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"doula-cloud/api/internal/activity"
)

// ActivateFromIntake is ADR-0015's `intake -> active` move, and the only
// place it happens. Both ways of reaching it run this function: the
// manual PATCH .../engagements/{engagementId}/status (#253), and the
// automatic one ADR-0015 rules -- "`intake` -> `active` happens by
// itself, the first time a Visit is scheduled" (#895) -- which every
// Visit write that leaves a scheduled_at non-null calls. One body, so the
// audit entry the move leaves behind -- ActionCarePhaseChanged, carrying
// both sides of the status -- can never differ depending on which door
// the Engagement came through.
//
// moved reports whether this call was the one that made the move. false
// is the ordinary answer on the automatic path -- an Engagement already
// 'active' or 'completed' is left exactly as it is, and nothing at all is
// written -- and the concurrent-writer case on the manual one, where the
// caller had already read 'intake'.
//
// actorStaffID is the Staff member who asked, never whoever the Visit is
// assigned to. ADR-0015: "The event records the person who scheduled, not
// a null system actor -- a doula did that", and an Admin booking a
// colleague onto a birth is the person who scheduled.
//
// The automatic call carries no role gate of its own, and that is
// ADR-0015's ruling rather than an omission: the only cell its
// permissions table spends on this transition is `intake -> active
// (manual)`. The automatic move is a consequence of scheduling a Visit,
// so it is gated by whoever may schedule one.
//
// The write itself is the guard. `WHERE status = 'intake'` decides
// atomically whether this caller is the one making the move, so a
// concurrent second write finds no row and reports moved=false rather
// than laying down a second audit entry claiming the same transition. That is the ordinary, expected answer on the automatic
// path -- every Visit scheduled after the first one finds the Engagement
// already 'active' -- and it is what makes ADR-0015's "one-way, one-time"
// rule hold without a separate read-then-check.
func ActivateFromIntake(ctx context.Context, tx *sql.Tx, practiceID, engagementID, actorStaffID string) (moved bool, err error) {
	var id string
	err = tx.QueryRowContext(ctx,
		`UPDATE engagements SET status = $1 WHERE id = $2 AND status = $3 RETURNING id`,
		StatusActive, engagementID, StatusIntake,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("engagement: activate from intake: %w", err)
	}

	if err := recordFact(ctx, tx, practiceID, engagementID, actorStaffID, activity.ActionCarePhaseChanged,
		map[string]any{"statusBefore": StatusIntake, "statusAfter": StatusActive}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, err
	}
	return true, nil
}
