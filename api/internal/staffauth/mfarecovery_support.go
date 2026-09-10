package staffauth

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/internalauth"
)

// authorizeInternal is the same guard every process-* and operator
// endpoint uses (billing.authorizeInternal, registerInternalRoutes) --
// no session, no Practice context, a caller identity only Doula Cloud's
// own operators and its Scheduler jobs can present (ADR-0037).
func authorizeInternal(w http.ResponseWriter, r *http.Request, auth *internalauth.Guard) bool {
	if !auth.Allow(r) {
		apierr.WriteError(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// SupportClearRequest is the body a Doula Cloud operator's own tooling
// sends: which Staff member's enrolment to clear, and the operator's own
// name -- staff_auth_events.actor_operator, docs/runbooks/mfa-recovery-
// support.md's "who ran it" requirement.
type SupportClearRequest struct {
	StaffID  string `json:"staffId"`
	Operator string `json:"operator"`
}

// SupportClearHandler is #605's last-resort path: an operator clears a
// sole Owner's enrolment after matching a live video call and government
// ID against the identity-verified representative on her Practice's
// Stripe Connect account (ADR-0007) -- proof this endpoint has no way to
// check itself, which is exactly why the AC asks for no product surface.
// Shaped like billing.FoundingGrantHandler: internal-caller-gated,
// invoked by an operator's own tooling, never a screen a Practice can
// reach -- "no self-service endpoint" means no Staff or Owner session
// can call this, not that it has to be an ad-hoc INSERT. No mandatory
// hold (#605: "a delay adds little against a determined attacker while
// costing a real doula a day of lockout during a birth"); notice fires
// at the moment of the reset, same as every other path.
func SupportClearHandler(accounts authn.AccountManager, db *sql.DB, auth *internalauth.Guard) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorizeInternal(w, r, auth) {
			return
		}

		var req SupportClearRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if !ParseUUID(w, "staff", req.StaffID) {
			return
		}
		operator := strings.TrimSpace(req.Operator)
		if operator == "" {
			apierr.WriteError(w, "operator is required", http.StatusBadRequest)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		// Authenticated by the internal guard, not a session -- neither
		// app.current_practice_id nor app.current_identity_uid is ever set
		// here, so staff's own RLS policies admit nothing. Same reuse of
		// 00033's trust flag as SpendMFARecoveryHandler, and for the same
		// reason: see that handler's own comment.
		if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var identityUID string
		err = tx.QueryRowContext(r.Context(), `SELECT identity_uid FROM staff WHERE id = $1`, req.StaffID).Scan(&identityUID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no staff member found for that id", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := clearEnrolmentAndRecord(r.Context(), tx, accounts, req.StaffID, identityUID, AuthEventSupport, "", operator); err != nil {
			// coverage:ignore reason: DB/Admin SDK failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.WriteHeader(http.StatusNoContent)
	})
}
