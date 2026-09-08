package plans

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/engagement"
)

// birthPlanType is the only plan_type ClientGetBirthPlanHandler will ever
// query for -- hardcoded rather than taken from the URL, so Care Plan is
// structurally unreachable through this handler regardless of what
// 00013_plan_instances_client_birth_plan_read.sql's RLS policy allows.
const birthPlanType = "birth_plan"

// ClientGetBirthPlanHandler views the Birth Plan instance for the
// Client-portal caller's Engagement -- clientauth.Middleware has already
// confirmed the caller's Client owns :engagementId. Read-only: there is no
// corresponding PUT/POST route for this population. Must be mounted behind
// clientauth.Middleware.
func ClientGetBirthPlanHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		inputs, err := fetchBirthPlanInputs(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientauth.Middleware already confirmed the row exists
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !engagement.OffersBirthPlan(inputs) {
			// ADR-0015: a Birth Plan is offered where kind = birth (#311)
			// and the Engagement has a living or expected baby (#294).
			// Refused at the API independently of the portal's own nav/hub
			// gating, so a direct request never sees an empty "not yet"
			// document -- the same refusal a postpartum-only Engagement
			// gets, and the same one an Engagement whose pregnancy ended
			// in a loss gets, with nothing on the Plan Instance changed to
			// produce it.
			apierr.WriteError(w, "no birth plan found for this engagement", http.StatusNotFound)
			return
		}

		fields, answers, clientAcknowledgedAt, err := fetchInstance(r.Context(), tx, engagementID, birthPlanType)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no birth plan found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := InstanceResponse{EngagementID: engagementID, PlanType: birthPlanType, Fields: fields, Answers: answers.nonEmpty(), ClientAcknowledgedAt: clientAcknowledgedAt}
		apierr.WriteJSON(w, http.StatusOK, out)
	})
}

// ClientAcknowledgeBirthPlanHandler records that the Client-portal
// caller has read her Birth Plan (#301, v1: acknowledgement only -- she
// cannot edit a field or suggest a change through this endpoint or any
// other). Repeatable: acknowledging again after already having
// acknowledged just refreshes client_acknowledged_at, and there is no
// Client-facing way to undo it -- only a Staff edit that changes the
// stored answers clears it (PutInstanceHandler). Must be mounted behind
// clientauth.Middleware.
func ClientAcknowledgeBirthPlanHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())
		clientID, _ := clientauth.ClientID(r.Context())

		inputs, err := fetchBirthPlanInputs(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientauth.Middleware already confirmed the row exists
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !engagement.OffersBirthPlan(inputs) {
			// ADR-0015, the same gate ClientGetBirthPlanHandler carries:
			// a Client who is not offered a Birth Plan cannot stamp
			// client_acknowledged_at on one either. Hiding the read and
			// leaving the write open would let a stale portal tab record
			// that she read a document the product has stopped offering
			// her.
			apierr.WriteError(w, "no birth plan found for this engagement", http.StatusNotFound)
			return
		}

		fields, answers, _, err := fetchInstance(r.Context(), tx, engagementID, birthPlanType)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no birth plan found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// acknowledge_birth_plan (00084) is a SECURITY DEFINER function,
		// not a Client-tier RLS UPDATE policy: RLS scopes rows, not
		// columns, and a policy permissive enough for this write would
		// admit one that overwrites Staff's own answers just as easily.
		// The function re-checks ownership itself, the same caution
		// portal_account_reuse_for_accept takes for the same reason.
		var clientAcknowledgedAt sql.NullTime
		if err := tx.QueryRowContext(r.Context(),
			`SELECT acknowledge_birth_plan($1)`, engagementID,
		).Scan(&clientAcknowledgedAt); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// acknowledge_birth_plan returning NULL (its own ownership check
		// failed, zero rows updated) would make this an activity row for
		// a write that didn't happen -- but clientauth.Middleware already
		// proved this same engagementID belongs to this Client, on this
		// same tx, moments before fetchInstance ran above, and no handler
		// ever deletes a plan_instances row or reassigns an Engagement's
		// client_id. Not defended against here for that reason (CLAUDE.md:
		// don't validate against a state the code's own invariants rule
		// out) -- the function's own check is defense-in-depth for a
		// future bug in that boundary, not a case this handler expects.
		if err := recordBirthPlanAcknowledged(r.Context(), tx, engagementID, clientID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := InstanceResponse{EngagementID: engagementID, PlanType: birthPlanType, Fields: fields, Answers: answers.nonEmpty(), ClientAcknowledgedAt: nullTimePtr(clientAcknowledgedAt)}
		apierr.WriteJSON(w, http.StatusOK, out)
	})
}

// recordBirthPlanAcknowledged writes #301's activity row for the
// Client's own acknowledgement -- actor_kind 'client' (ADR-0022), same
// shape as contracts.recordContractSigned. clientauth.Middleware sets
// only app.current_client_id, never app.current_practice_id, so
// activity.ScopedTo widens tx to the resolved Practice as part of the
// one Record call below.
func recordBirthPlanAcknowledged(ctx context.Context, tx *sql.Tx, engagementID, clientID string) error {
	var practiceID string
	if err := tx.QueryRowContext(ctx,
		`SELECT practice_id FROM engagements WHERE id = $1`, engagementID,
	).Scan(&practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("plans: resolve engagement for birth plan acknowledged: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionBirthPlanAcknowledged),
		Actor:       activity.ClientActor(clientID),
	}, activity.ScopedTo(practiceID)); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("plans: record birth plan acknowledged: %w", err)
	}
	return nil
}

// fetchBirthPlanInputs reads the two facts engagement.OffersBirthPlan
// asks an Engagement for -- clientauth.Middleware has already confirmed
// the caller's Client owns this Engagement, so this is a plain lookup
// rather than a second ownership check. Neither fact leaves this
// package: they are read to answer the derived question and discarded,
// so the birth outcome never reaches a Client-facing response.
func fetchBirthPlanInputs(ctx context.Context, tx *sql.Tx, engagementID string) (engagement.BirthPlanInputs, error) {
	var kind string
	var birthOutcome *string
	err := tx.QueryRowContext(ctx,
		`SELECT kind::text, birth_outcome::text FROM engagements WHERE id = $1`,
		engagementID).Scan(&kind, &birthOutcome)
	// coverage:ignore reason: clientauth.Middleware already confirmed the row exists; a query failure here is a DB-level fault, not exercised by unit tests
	if err != nil {
		return engagement.BirthPlanInputs{}, fmt.Errorf("plans: fetch birth plan inputs: %w", err)
	}
	return engagement.BirthPlanInputs{Kind: engagement.Kind(kind), BirthOutcome: birthOutcome}, nil
}
