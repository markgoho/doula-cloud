package offer

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/dbtx"
	"doula-cloud/api/internal/personname"
	"doula-cloud/api/internal/staffauth"
)

// DecisionResponse reports the state an Offer ended up in. It is the same
// shape for accept, decline, and withdraw so a caller reads one field to
// find out what happened.
//
// An accept also names the Engagement and its Client (#2268): she is
// attached by the time the response is written, so her reach now covers
// both, and the screen can say whose work is hers and link to it.
type DecisionResponse struct {
	OfferID        string `json:"offerId"`
	State          string `json:"state"`
	EngagementID   string `json:"engagementId,omitempty"`
	EngagementKind string `json:"engagementKind,omitempty"`
	ClientName     string `json:"clientName,omitempty"`
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
// otherwise each pass their own TransitionAccept guard.
//
// Must be mounted behind staffauth.Middleware.
func AcceptHandler() http.Handler {
	return decisionHandler(accept)
}

// DeclineHandler turns an Offer down. There is no reason field: #229
// settled that a decline is durable and repeatable, and asking why turns
// a no into a negotiation. Declining does not bar a re-offer -- 00030's
// partial index only covers an Offer stored as open, so the Practice may offer
// the same Engagement again, and #229 explicitly wants it to be able to.
func DeclineHandler() http.Handler {
	return decisionHandler(decline)
}

// decisionHandler is the shape accept and decline share: resolve the
// caller, lock the Engagement the Offer sits under, run the decision.
func decisionHandler(decide func(context.Context, *sql.Tx, string, string, string, string) (DecisionResponse, int, string)) http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		offerID := c.ID("offerId")
		engagementID, status, msg := lockOwnOffer(r.Context(), c.Tx, offerID, c.StaffID)
		if status != http.StatusOK {
			apierr.WriteError(w, msg, status)
			return
		}
		resp, status, msg := decide(r.Context(), c.Tx, c.PracticeID, engagementID, offerID, c.StaffID)
		if status != http.StatusOK {
			apierr.WriteError(w, msg, status)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	}, staffauth.PathID("offer", "offerId"))
}

// accept runs the whole acceptance in one locked window.
func accept(ctx context.Context, tx *sql.Tx, practiceID, engagementID, offerID, staffID string) (DecisionResponse, int, string) {
	now := clock.Now(ctx)
	var amountCents sql.NullInt64
	var terms sql.NullString
	err := tx.QueryRowContext(ctx,
		TransitionAccept.update(`id = $3 AND staff_id = $2`, `amount_cents, terms`),
		now, staffID, offerID,
	).Scan(&amountCents, &terms)
	if errors.Is(err, sql.ErrNoRows) {
		return DecisionResponse{}, http.StatusConflict, refusal(ctx, tx, TransitionAccept, offerID)
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionOfferAccepted),
		Actor:       activity.StaffActor(staffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}

	// Every other open Offer on this Engagement loses, named to the
	// person whose yes closed it -- the supersession has a human cause
	// and records it, unlike completion's cascade (ADR-0008). Only an
	// Offer still open now: one that had lapsed was recorded as expired
	// by lockOwnOffer, and the guard would pass it over even if it had
	// not been.
	supersededIDs, err := dbtx.Strings(ctx, tx,
		TransitionSupersede.update(`engagement_id = $3 AND id <> $4`, `id`),
		now, staffID, engagementID, offerID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}
	for _, supersededOfferID := range supersededIDs {
		diff, err := json.Marshal(map[string]string{"supersededOfferId": supersededOfferID, "acceptedOfferId": offerID})
		if err != nil {
			// coverage:ignore reason: a map of strings always marshals cleanly, not exercised by unit tests
			return DecisionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
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
			return DecisionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
		}
	}

	// attached_by is the accepter herself: an Offer's attachment is
	// opened by her agreement, not by the Practice reaching in.
	if err := staffauth.Grant(ctx, tx, engagementID, staffID, staffID, nullableInt64(amountCents), nullableString(terms)); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}

	// Read through her own reach, which the Attachment above has just
	// opened in this transaction.
	var kind, givenName string
	var preferredName, familyName sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT e.kind::text, c.given_name, c.preferred_name, c.family_name
		   FROM engagements e JOIN clients c ON c.id = e.client_id
		  WHERE e.id = $1`,
		engagementID,
	).Scan(&kind, &givenName, &preferredName, &familyName); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}

	return DecisionResponse{
		OfferID:        offerID,
		State:          "accepted",
		EngagementID:   engagementID,
		EngagementKind: kind,
		ClientName:     personname.Full(givenName, preferredName.String, familyName.String),
	}, http.StatusOK, ""
}

// decline turns the Offer down, and says yes a second time to a repeat of
// the same decline: #229's "durable, repeatable". Anything else -- an
// Offer already accepted, withdrawn, superseded, or expired -- is a 409,
// because those are real disagreements about what happened, not a
// re-sent click.
func decline(ctx context.Context, tx *sql.Tx, practiceID, engagementID, offerID, staffID string) (DecisionResponse, int, string) {
	result, err := tx.ExecContext(ctx,
		TransitionDecline.update(`id = $3 AND staff_id = $2`, ""),
		clock.Now(ctx), staffID, offerID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}
	rows, err := result.RowsAffected()
	if err != nil {
		// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}
	if rows == 0 {
		if state, err := currentState(ctx, tx, offerID); err == nil && state == StateDeclined {
			return DecisionResponse{OfferID: offerID, State: string(StateDeclined)}, http.StatusOK, ""
		}
		return DecisionResponse{}, http.StatusConflict, refusal(ctx, tx, TransitionDecline, offerID)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionOfferDeclined),
		Actor:       activity.StaffActor(staffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DecisionResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}
	return DecisionResponse{OfferID: offerID, State: string(StateDeclined)}, http.StatusOK, ""
}

// lockOwnOffer finds the Engagement an Offer belongs to, takes a row lock
// on that Engagement, and records every lapse among that Engagement's
// Offers -- in that order, so the lapse and the decision that follows it
// see one consistent view, and an acceptance's supersession finds a
// lapsed sibling already 'expired'. The Offer must be the caller's own:
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
		return "", http.StatusInternalServerError, apierr.MsgInternalError
	}

	if _, err := tx.ExecContext(ctx, `SELECT id FROM engagements WHERE id = $1 FOR UPDATE`, engagementID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", http.StatusInternalServerError, apierr.MsgInternalError
	}
	if err := recordLapses(ctx, tx, byEngagementID, engagementID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", http.StatusInternalServerError, apierr.MsgInternalError
	}
	return engagementID, http.StatusOK, ""
}

// refusal says what an Offer that t could not move is now, so the person
// clicking finds out whether she was too slow, whether the Practice took
// it back, or whether it simply ran out.
func refusal(ctx context.Context, tx *sql.Tx, t Transition, offerID string) string {
	state, err := currentState(ctx, tx, offerID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "This Offer is no longer open."
	}
	return t.refusal(state)
}

// currentState is what the Offer is now, through offer_state_at.
func currentState(ctx context.Context, tx *sql.Tx, offerID string) (State, error) {
	var state State
	err := tx.QueryRowContext(ctx,
		`SELECT offer_state_at(state, expires_at, $2)::text FROM engagement_offers WHERE id = $1`,
		offerID, clock.Now(ctx),
	).Scan(&state)
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
