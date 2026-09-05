package staffauth

import (
	"encoding/json"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
)

// mfaRequiredRequest is PutMFARequiredHandler's body -- one boolean,
// PUT semantics (replace the switch's whole state).
type mfaRequiredRequest struct {
	Required bool `json:"required"`
}

// PutMFARequiredHandler lets a Practice Owner throw or clear "require
// MFA for all staff" (#606). RequireConfirmed guards it -- #606's brief:
// the cutover is immediate, no grace period, so the confirmation is what
// makes that fair, and the client is expected to have already shown the
// affected-Staff count from GetMFAImpactHandler before asking for it.
//
// Idempotent by construction: a retry with the same body reads the same
// current value, updates nothing, and records nothing new -- the audit
// row is written only when the value actually changes, the same
// "records an audit event only for an axis that actually changed" rule
// UpdateMembershipHandler already follows.
func PutMFARequiredHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireOwner(w, r)
		if !ok {
			return
		}
		if !RequireConfirmed(w, r) {
			return
		}

		var req mfaRequiredRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var before bool
		if err := tx.QueryRowContext(r.Context(),
			`SELECT require_mfa_for_all_staff FROM practices WHERE id = $1`, practiceID,
		).Scan(&before); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if before != req.Required {
			if _, err := tx.ExecContext(r.Context(),
				`UPDATE practices SET require_mfa_for_all_staff = $1 WHERE id = $2`,
				req.Required, practiceID,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
				return
			}

			action := "mfa_required_disabled"
			if req.Required {
				action = "mfa_required_enabled"
			}
			actorStaffID, _ := StaffID(r.Context())
			if err := activity.Record(r.Context(), tx, activity.Entry{
				PracticeID:  practiceID,
				SubjectKind: "practice",
				SubjectID:   practiceID,
				Action:      action,
				Diff:        json.RawMessage("{}"),
				Actor:       activity.StaffActor(actorStaffID),
			}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// mfaImpactResponse is GetMFAImpactHandler's body. Required carries the
// switch's own current value alongside the count, so this one read is
// enough for the settings screen to render both the toggle's state and
// what throwing it would do -- nothing else exposes
// require_mfa_for_all_staff to the app.
type mfaImpactResponse struct {
	Required            bool `json:"required"`
	WithoutSecondFactor int  `json:"withoutSecondFactor"`
}

// GetMFAImpactHandler reports how many of a Practice's Staff currently
// hold no enrolled second factor -- what PutMFARequiredHandler's
// confirmation states before an Owner throws the switch (#606's AC).
// Mounted Owner-only (staffauth.GatedRouter.Get's role declaration), so
// no further role check is needed here.
func GetMFAImpactHandler(accounts authn.AccountManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireTx(w, r)
		if !ok {
			// coverage:ignore reason: Middleware always sets a tx before this handler runs
			return
		}

		var required bool
		if err := tx.QueryRowContext(r.Context(),
			`SELECT require_mfa_for_all_staff FROM practices WHERE id = $1`, practiceID,
		).Scan(&required); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Every Staff member at the Practice, not only those the switch
		// would newly bar: an Owner with no factor is already refused
		// today, and belongs in "how many Staff it will affect" just as
		// much as a Doula the switch is about to newly bar.
		rows, err := tx.QueryContext(r.Context(),
			`SELECT s.identity_uid FROM staff s
			 JOIN practice_memberships pm ON pm.staff_id = s.id
			 WHERE pm.practice_id = $1`,
			practiceID,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		defer func() { _ = rows.Close() }()

		var uids []string
		for rows.Next() {
			var uid string
			if err := rows.Scan(&uid); err != nil {
				// coverage:ignore reason: DB scan failure, not exercised by unit tests
				apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
			uids = append(uids, uid)
		}
		if err := rows.Err(); err != nil {
			// coverage:ignore reason: DB iteration failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		count, err := accounts.CountWithoutSecondFactor(r.Context(), uids)
		if err != nil {
			// coverage:ignore reason: Admin SDK failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mfaImpactResponse{Required: required, WithoutSecondFactor: count})
	})
}
