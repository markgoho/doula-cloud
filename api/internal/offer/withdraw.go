package offer

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/staffauth"
)

// WithdrawHandler takes an Offer back. This is the Practice's half of
// #229's answer to a changed fact: an Offer is a copy and is never
// refreshed after it is sent, so a fee or a due date that moved means
// withdraw and re-offer, which records both events instead of quietly
// rewriting the first one. Owner or Admin only; must be mounted behind
// staffauth.Middleware.
func WithdrawHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			return
		}
		actorStaffID, _ := staffauth.StaffID(r.Context())

		offerID := r.PathValue("offerId")
		if !staffauth.ParseUUID(w, "offer", offerID) {
			return
		}

		if err := expireOpen(r.Context(), tx, byID, offerID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		result, err := tx.ExecContext(r.Context(),
			`UPDATE engagement_offers
			    SET state = 'withdrawn', decided_at = now(), decided_by = $1
			  WHERE id = $2 AND state = 'offered'`,
			actorStaffID, offerID,
		)
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
			http.Error(w, "no open offer found at this practice", http.StatusNotFound)
			return
		}

		writeJSON(w, DecisionResponse{OfferID: offerID, State: "withdrawn"})
	})
}

// CloseOnCompletion is the Engagement-completes half of the lifecycle:
// every Offer still open on engagementID goes 'withdrawn', and
// decided_by stays NULL.
//
// That NULL is deliberate and ADR-0008 is explicit about it: this is the
// one terminal state with no human actor at the moment it fires, and a
// build must not invent a system staff row to avoid the NULL, because
// that would misrecord a human action that did not happen. The
// Engagement's own completion, and who completed it, is the audit answer
// -- it is recorded on the Engagement, not copied onto every Offer the
// cascade touched.
//
// Called by the Engagement completion handler, in its transaction.
func CloseOnCompletion(ctx context.Context, tx *sql.Tx, engagementID string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE engagement_offers
		    SET state = 'withdrawn', decided_at = now(), decided_by = NULL
		  WHERE engagement_id = $1 AND state = 'offered'`,
		engagementID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("offer: close offers on completion: %w", err)
	}
	return nil
}
