package engagement

import (
	"encoding/json"
	"net/http"

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
		tx, _, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			return
		}
		actorStaffID, _ := staffauth.StaffID(r.Context())

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
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
		rows, err := result.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if rows == 0 {
			http.Error(w, "engagement not found", http.StatusNotFound)
			return
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
