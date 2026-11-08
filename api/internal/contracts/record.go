package contracts

import (
	"context"
	"encoding/json/jsontext"
	"fmt"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/staffauth"
)

// recordEngagementActivity records action against c's Engagement, with
// c's Staff member as the actor.
func recordEngagementActivity(ctx context.Context, c staffauth.Call, action activity.EngagementAction, diff jsontext.Value) error {
	if err := activity.Record(ctx, c.Tx, activity.Entry{
		PracticeID:  c.PracticeID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   c.EngagementID,
		Action:      string(action),
		Diff:        diff,
		Actor:       activity.StaffActor(c.StaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("contracts: record %s: %w", action, err)
	}
	return nil
}
