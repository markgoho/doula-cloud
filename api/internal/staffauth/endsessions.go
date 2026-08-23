package staffauth

import (
	"database/sql"
	"errors"
	"net/http"

	"doula-cloud/api/internal/authn"
)

// EndSessionsHandler lets a Practice Owner end every session a Staff
// member holds, on every device -- offboarding, or a lost phone. This is
// deliberately not what ordinary sign-out does: sign-out ends only the
// browser making the request, this ends all of them (#154). Must be
// mounted behind staffauth.Middleware, which is what makes the 403s for
// a non-Owner or an Owner at a different Practice automatic: the caller
// must already hold a membership at :practiceId to reach RequireOwner at
// all.
func EndSessionsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireOwner(w, r)
		if !ok {
			return
		}
		targetStaffID := r.PathValue("staffId")

		var identityUID string
		err := tx.QueryRowContext(r.Context(),
			`SELECT s.identity_uid FROM staff s
			 JOIN practice_memberships pm ON pm.staff_id = s.id
			 WHERE pm.practice_id = $1 AND pm.staff_id = $2`,
			practiceID, targetStaffID,
		).Scan(&identityUID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no membership found for that staff member at this practice", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := authn.EndAllSessions(r.Context(), tx, identityUID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
