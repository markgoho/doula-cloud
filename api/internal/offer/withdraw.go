package offer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/staffauth"
)

// WithdrawHandler takes an Offer back. This is the Practice's half of
// #229's answer to a changed fact: an Offer is a copy and is never
// refreshed after it is sent, so a fee or a due date that moved means
// withdraw and re-offer, which records both events instead of quietly
// rewriting the first one. Owner or Admin only; must be mounted behind
// staffauth.Middleware.
//
// Owner and Admin is declared at the mount, not checked here (#1016,
// following #970's and #990's own move): the handler no longer calls
// staffauth.RequireOwnerOrAdmin, because a Doula is refused by the gate
// before this runs. Widening or narrowing this route means editing its
// role list in mount.go.
func WithdrawHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		tx, practiceID, actorStaffID, offerID := c.Tx, c.PracticeID, c.StaffID, c.ID("offerId")

		if err := recordLapses(r.Context(), tx, byID, offerID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var engagementID string
		err := tx.QueryRowContext(r.Context(),
			TransitionWithdraw.update(`id = $3`, `engagement_id`),
			clock.Now(r.Context()), actorStaffID, offerID,
		).Scan(&engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no open offer found at this practice", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionOfferWithdrawn),
			Actor:       activity.StaffActor(actorStaffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, DecisionResponse{OfferID: offerID, State: string(StateWithdrawn)})
	}, staffauth.PathID("offer", "offerId"))
}

// CloseOnCompletion is the Engagement-completes half of the lifecycle
// (TransitionCloseOnCompletion): every Offer still open on engagementID
// goes 'withdrawn', decided_by stays NULL, and decided_by_kind says it
// was the completion. An Offer that had already lapsed is recorded as
// expired first, so it is never withdrawn by a cascade that came after
// it ran out.
//
// That NULL is deliberate and ADR-0008 is explicit about it: there is no
// human actor at the moment it fires, and a build must not invent a
// system staff row to avoid the NULL, because that would misrecord a
// human action that did not happen. The Engagement's own completion, and
// who completed it, is the audit answer -- it is recorded on the
// Engagement, not copied onto every Offer the cascade touched.
//
// Called by the Engagement completion handler, in its transaction.
func CloseOnCompletion(ctx context.Context, tx *sql.Tx, engagementID string) error {
	if err := recordLapses(ctx, tx, byEngagementID, engagementID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if _, err := tx.ExecContext(ctx,
		TransitionCloseOnCompletion.update(`engagement_id = $3`, ""),
		clock.Now(ctx), nil, engagementID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("offer: close offers on completion: %w", err)
	}
	return nil
}
