package contracts

import (
	"context"
	"encoding/json/jsontext"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/staffauth"
)

// stampTransition moves ct to status with stamp, which names the instant
// and the Staff member the move is stamped with, and records action. It
// writes the refusal and returns false when either write fails.
func stampTransition(w http.ResponseWriter, r *http.Request, c staffauth.Call, ct contractRow, stamp string, status Status, action activity.EngagementAction) bool {
	if _, err := c.Tx.ExecContext(r.Context(), stamp, string(status), clock.Now(r.Context()), c.StaffID, ct.ID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if err := recordEngagementActivity(r.Context(), c, action, nil); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	return true
}

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
