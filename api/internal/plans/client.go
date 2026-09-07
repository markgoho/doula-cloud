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
