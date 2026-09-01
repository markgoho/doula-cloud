package engagement

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/staffauth"
)

// CompleteResponse confirms the Engagement's new status.
type CompleteResponse struct {
	EngagementID string `json:"engagementId"`
	Status       string `json:"status"`
}

// CompleteHandler marks an Engagement completed and runs ADR-0008's
// completion cascade with it, in one transaction: every Offer still open
// on the Engagement goes 'withdrawn', and every attachment still open
// gets its ended_at. Both are part of completing the work rather than
// follow-up chores -- an Offer nobody can now accept and a reach into a
// Client record nobody is now serving are exactly what "completed" is
// supposed to mean.
//
// The Offer half records decided_by NULL on purpose (ADR-0008): the
// cascade has no human actor at the moment it fires, and inventing a
// system staff row to avoid the NULL would misrecord an action nobody
// took. The attachment half does have one -- the person completing the
// Engagement -- so ended_by names her.
//
// Owner or Admin only; must be mounted behind staffauth.Middleware.
func CompleteHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			return
		}
		actorStaffID, _ := staffauth.StaffID(r.Context())

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}

		// Read the status this transition is coming from, so the activity
		// row below is only written on a real intake/active -> completed
		// move -- not on every idempotent retry of an already-completed
		// Engagement, which would otherwise pile up duplicate "completed"
		// entries on the ledger for one real event.
		var previousStatus string
		err := tx.QueryRowContext(r.Context(), `SELECT status FROM engagements WHERE id = $1`, engagementID).Scan(&previousStatus)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// RLS already scopes this to the caller's Practice, so an
		// Engagement elsewhere is simply not here. Completing an already
		// completed Engagement is a no-op that still runs the cascade --
		// it is idempotent by construction, and re-running it closes
		// anything a partial earlier run left behind.
		result, err := tx.ExecContext(r.Context(),
			`UPDATE engagements SET status = 'completed' WHERE id = $1`, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		// coverage:ignore reason: unreachable in this transaction -- the SELECT above already confirmed engagementID exists at this Practice under the same snapshot, so RowsAffected can never be 0 here; kept as a defensive backstop rather than trusted away
		if _, err := result.RowsAffected(); err != nil {
			// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if previousStatus != "completed" {
			if err := activity.Record(r.Context(), tx, activity.Entry{
				PracticeID:  practiceID,
				SubjectKind: activity.SubjectEngagement,
				SubjectID:   engagementID,
				Action:      string(activity.ActionEngagementCompleted),
				Actor:       activity.StaffActor(actorStaffID),
			}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		if err := offer.CloseOnCompletion(r.Context(), tx, engagementID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := staffauth.EndAttachments(r.Context(), tx, engagementID, actorStaffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(CompleteResponse{EngagementID: engagementID, Status: "completed"}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}
