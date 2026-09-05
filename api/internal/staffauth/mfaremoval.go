package staffauth

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
)

// RemoveSecondFactorHandler lets a signed-in Staff member remove her own
// TOTP enrolment (#606) -- the voluntary mirror of #615's three recovery
// paths, for a person who still holds her factor and simply wants it
// gone (a phone upgrade, say). Self-only, same "no {practiceId}, no
// staff id" shape as UpdateWorkStateHandler.
//
// Guarded by RequireRecentAuth, the same step-up #605's Owner-vouch
// action uses: removing a factor is exactly the sensitive action that
// exists for. Reuses clearEnrolmentAndRecord, #615's own convergence
// point -- Admin SDK clear, end every live session, queue the notice,
// record the audit row -- with AuthEventRemoved marking this as
// self-caused (actor_staff_id = staff_id) rather than one of the three
// recovery paths.
func RemoveSecondFactorHandler(verifier authn.Verifier, accounts authn.AccountManager, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		staffID, found, err := setIdentityAndResolveStaff(r.Context(), tx, uid)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !found {
			apierr.WriteError(w, MsgNoMatchingStaffAccount, http.StatusForbidden)
			return
		}

		if !RequireRecentAuth(w, r, verifier, tx, staffID) {
			return
		}

		if err := clearEnrolmentAndRecord(r.Context(), tx, accounts, staffID, uid, AuthEventRemoved, staffID, ""); err != nil {
			// coverage:ignore reason: DB/Admin SDK failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true
		w.WriteHeader(http.StatusNoContent)
	})
}
