package contracts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// PutContractAmountRequest is PutContractAmountHandler's body: the new
// amount, in cents, to override a Contract's rate-card-derived amount
// to.
type PutContractAmountRequest struct {
	AmountCents int64 `json:"amountCents"`
}

// ContractAmountResponse is PutContractAmountHandler's body.
type ContractAmountResponse struct {
	AmountCents int64 `json:"amountCents"`
}

// contractAmountDiff is PutContractAmountHandler's activity diff shape,
// mirroring practicerate.rateDiff.
type contractAmountDiff struct {
	AmountCentsBefore int64 `json:"amountCentsBefore"`
	AmountCentsAfter  int64 `json:"amountCentsAfter"`
}

// PutContractAmountHandler lets a Practice Owner or Admin override the
// amount a Contract carries -- #967's AC: "An Owner or an Admin may
// override a Contract's amount for a Client whose situation is unusual.
// A Doula may not." Gated by TransitionOverrideAmount: only a Draft
// Contract's amount may be overridden, the same precondition every other
// merge-field edit already carries (TransitionEdit) -- once Sent or
// beyond, nothing about the Contract is editable through these routes,
// which satisfies #967's AC ("a signed Contract's amount never changes
// for any reason") with room to spare. Idempotent by construction, the
// same "records an audit event only when the value actually changes"
// rule PutRateHandler follows: a retry with the same amount writes
// nothing and records nothing new. Must be mounted behind
// staffauth.Middleware.
//
// Owner and Admin is declared at the mount, not checked here (#970): the
// handler no longer calls RequireOwnerOrAdmin, because a Doula is refused
// by the gate before this runs. Widening or narrowing this route means
// editing its ir.ExemptGated role list in mount.go, the same way
// PutTemplateHandler's Owner-only rule moved there.
func PutContractAmountHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var req PutContractAmountRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if req.AmountCents <= 0 {
			apierr.WriteError(w, "amountCents must be a positive integer", http.StatusBadRequest)
			return
		}

		id, _, status, _, before, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if ok, refusal := TransitionOverrideAmount.Check(Status(status)); !ok {
			apierr.WriteError(w, refusal, http.StatusConflict)
			return
		}

		if before != req.AmountCents {
			if _, err := tx.ExecContext(r.Context(),
				`UPDATE contracts SET amount_cents = $1 WHERE id = $2`,
				req.AmountCents, id,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}

			diffJSON, err := json.Marshal(contractAmountDiff{AmountCentsBefore: before, AmountCentsAfter: req.AmountCents})
			if err != nil {
				// coverage:ignore reason: marshal of a fixed, always-serializable struct never fails
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}

			staffID, _ := staffauth.StaffID(r.Context())
			if err := activity.Record(r.Context(), tx, activity.Entry{
				PracticeID:  practiceID,
				SubjectKind: activity.SubjectEngagement,
				SubjectID:   engagementID,
				Action:      string(activity.ActionContractAmountOverridden),
				Diff:        diffJSON,
				Actor:       activity.StaffActor(staffID),
			}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		apierr.WriteJSON(w, http.StatusOK, ContractAmountResponse(req))
	})
}
