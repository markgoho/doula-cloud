package offer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/staffauth"
)

// DecisionResponse reports the state an Offer ended up in. It is the same
// shape for accept, decline, and withdraw so a caller reads one field to
// find out what happened.
type DecisionResponse struct {
	OfferID string `json:"offerId"`
	State   string `json:"state"`
}

// AcceptHandler takes an Offer. Acceptance is what mints the granted
// attachment, with the fee and terms copied onto it so nothing can later
// rewrite what she agreed to (#229), and it is what closes every other
// open Offer on the same Engagement as superseded -- fan-out is uncapped
// and the first yes wins.
//
// The race is this handler's to own, not the database's: 00030's two
// partial unique indexes enforce "at most one open Offer per target",
// which is a different statement from "only one of N concurrent
// acceptances wins". The row lock on the Engagement below is what makes
// the second one true -- two accepts of two different Offer rows would
// otherwise each pass their own state = 'offered' guard.
//
// Must be mounted behind staffauth.Middleware.
func AcceptHandler() http.Handler {
	return decisionHandler(accept)
}

// DeclineHandler turns an Offer down. There is no reason field: #229
// settled that a decline is durable and repeatable, and asking why turns
// a no into a negotiation. Declining does not bar a re-offer -- 00030's
// partial index only covers state = 'offered', so the Practice may offer
// the same Engagement again, and #229 explicitly wants it to be able to.
func DeclineHandler() http.Handler {
	return decisionHandler(decline)
}

// decisionHandler is the shape accept and decline share: resolve the
// caller, lock the Engagement the Offer sits under, run the decision.
func decisionHandler(decide func(context.Context, *sql.Tx, string, string) (DecisionResponse, int, string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _, ok := staffauth.RequireTx(w, r)
		if !ok {
			// coverage:ignore reason: Middleware always sets a tx before this handler runs
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		offerID := r.PathValue("offerId")
		if !staffauth.ParseUUID(w, "offer", offerID) {
			return
		}

		resp, status, msg := decide(r.Context(), tx, offerID, staffID)
		if status != http.StatusOK {
			http.Error(w, msg, status)
			return
		}
		writeJSON(w, resp)
	})
}

// accept runs the whole acceptance in one locked window.
func accept(ctx context.Context, tx *sql.Tx, offerID, staffID string) (DecisionResponse, int, string) {
	engagementID, status, msg := lockOwnOffer(ctx, tx, offerID, staffID)
	if status != http.StatusOK {
		return DecisionResponse{}, status, msg
	}

	var amountCents sql.NullInt64
	var terms sql.NullString
	err := tx.QueryRowContext(ctx,
		`UPDATE engagement_offers
		    SET state = 'accepted', decided_at = now(), decided_by = $1
		  WHERE id = $2 AND staff_id = $1 AND state = 'offered'
		 RETURNING amount_cents, terms`,
		staffID, offerID,
	).Scan(&amountCents, &terms)
	if errors.Is(err, sql.ErrNoRows) {
		return DecisionResponse{}, http.StatusConflict, closedMessage(ctx, tx, offerID)
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}
	practiceID, _ := staffauth.PracticeID(ctx)
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionOfferAccepted),
		Actor:       activity.StaffActor(staffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}

	// Every other open Offer on this Engagement loses, named to the
	// person whose yes closed it -- the supersession has a human cause
	// and records it, unlike completion's cascade (ADR-0008).
	superseded, err := tx.QueryContext(ctx,
		`UPDATE engagement_offers
		    SET state = 'superseded', decided_at = now(), decided_by = $1
		  WHERE engagement_id = $2 AND state = 'offered' AND id <> $3
		 RETURNING id`,
		staffID, engagementID, offerID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}
	supersededIDs, err := scanIDs(superseded)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}
	for _, supersededOfferID := range supersededIDs {
		diff, err := json.Marshal(map[string]string{"supersededOfferId": supersededOfferID, "acceptedOfferId": offerID})
		if err != nil {
			// coverage:ignore reason: a map of strings always marshals cleanly, not exercised by unit tests
			return DecisionResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
		}
		if err := activity.Record(ctx, tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionOfferSuperseded),
			Diff:        diff,
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return DecisionResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
		}
	}

	// attached_by is the accepter herself: an Offer's attachment is
	// opened by her agreement, not by the Practice reaching in.
	if err := staffauth.Grant(ctx, tx, engagementID, staffID, staffID, nullableInt64(amountCents), nullableString(terms)); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}

	return DecisionResponse{OfferID: offerID, State: "accepted"}, http.StatusOK, ""
}

// scanIDs drains rows of its single "id" column, closing it either way.
func scanIDs(rows *sql.Rows) ([]string, error) {
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("offer: scan superseded offer id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("offer: iterate superseded offer ids: %w", err)
	}
	return ids, nil
}

// decline turns the Offer down, and says yes a second time to a repeat of
// the same decline: #229's "durable, repeatable". Anything else -- an
// Offer already accepted, withdrawn, superseded, or expired -- is a 409,
// because those are real disagreements about what happened, not a
// re-sent click.
func decline(ctx context.Context, tx *sql.Tx, offerID, staffID string) (DecisionResponse, int, string) {
	engagementID, status, msg := lockOwnOffer(ctx, tx, offerID, staffID)
	if status != http.StatusOK {
		return DecisionResponse{}, status, msg
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE engagement_offers
		    SET state = 'declined', decided_at = now(), decided_by = $1
		  WHERE id = $2 AND staff_id = $1 AND state = 'offered'`,
		staffID, offerID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}
	rows, err := result.RowsAffected()
	if err != nil {
		// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}
	if rows == 0 {
		if state, err := currentState(ctx, tx, offerID); err == nil && state == stateDeclined {
			return DecisionResponse{OfferID: offerID, State: stateDeclined}, http.StatusOK, ""
		}
		return DecisionResponse{}, http.StatusConflict, closedMessage(ctx, tx, offerID)
	}
	practiceID, _ := staffauth.PracticeID(ctx)
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionOfferDeclined),
		Actor:       activity.StaffActor(staffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}
	return DecisionResponse{OfferID: offerID, State: stateDeclined}, http.StatusOK, ""
}

// lockOwnOffer finds the Engagement an Offer belongs to, takes a row lock
// on that Engagement, and expires the Offer if it has run past its own
// expires_at -- in that order, so the expiry check and the decision that
// follows it see one consistent view. The Offer must be the caller's own:
// RLS scopes engagement_offers to the Practice, not to a person, so
// "someone else's Offer at my Practice" is refused here rather than in a
// policy.
func lockOwnOffer(ctx context.Context, tx *sql.Tx, offerID, staffID string) (engagementID string, status int, msg string) {
	err := tx.QueryRowContext(ctx,
		`SELECT engagement_id FROM engagement_offers WHERE id = $1 AND staff_id = $2`,
		offerID, staffID,
	).Scan(&engagementID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", http.StatusNotFound, "offer not found"
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", http.StatusInternalServerError, staffauth.MsgInternalError
	}

	if _, err := tx.ExecContext(ctx, `SELECT id FROM engagements WHERE id = $1 FOR UPDATE`, engagementID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", http.StatusInternalServerError, staffauth.MsgInternalError
	}
	if err := expireOpen(ctx, tx, byID, offerID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", http.StatusInternalServerError, staffauth.MsgInternalError
	}
	return engagementID, http.StatusOK, ""
}

// closedMessage says what an Offer that could not be decided is now, so
// the person clicking finds out whether she was too slow, whether the
// Practice took it back, or whether it simply ran out.
func closedMessage(ctx context.Context, tx *sql.Tx, offerID string) string {
	state, err := currentState(ctx, tx, offerID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "that offer is no longer open"
	}
	return "that offer is no longer open -- it is " + state
}

func currentState(ctx context.Context, tx *sql.Tx, offerID string) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, `SELECT state::text FROM engagement_offers WHERE id = $1`, offerID).Scan(&state)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("offer: read offer state: %w", err)
	}
	return state, nil
}

func nullableInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func nullableString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
