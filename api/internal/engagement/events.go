package engagement

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"doula-cloud/api/internal/activity"
)

// recordFact writes the one activity entry a change to an Engagement's
// mutable facts leaves behind -- status, kind or birth outcome, ADR-0015's
// facts -- in the caller's own transaction, so the change and its record
// land together or not at all.
//
// diff carries both sides of every fact the change touched, under
// activity's xBefore/xAfter convention, with a nil pointer marshaling to
// JSON null rather than being left out: "how did this thing come to be?"
// is answered by reading one row, never by replaying the ones before it.
// That is the shape engagement_events (00090) held in columns until #1423
// folded it into activity, the one ledger ADR-0022 names.
//
// The actor is always a Staff member. ADR-0015 rules that even the
// automatic intake -> active move a scheduled Visit makes (#895) "records
// the person who scheduled, not a null system actor -- a doula did that",
// so every writer has a real Staff id to pass.
func recordFact(ctx context.Context, tx *sql.Tx, practiceID, engagementID, actorStaffID string, action activity.EngagementAction, diff map[string]any) error {
	raw, err := json.Marshal(diff)
	if err != nil {
		// coverage:ignore reason: every caller passes strings and string pointers, which always marshal
		return fmt.Errorf("engagement: %s diff: %w", action, err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(action),
		Diff:        raw,
		Actor:       activity.StaffActor(actorStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("engagement: record %s: %w", action, err)
	}
	return nil
}
