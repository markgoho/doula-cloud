package billing

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/internalauth"
)

// DormantPracticesHandler lists the Practices holding Credits nobody has
// touched for DormancyNoticeYears -- the list the annual balance notice
// and the December/January due-diligence mailings are worked from. Read
// only: it identifies balances, and nothing anywhere writes one off.
func DormantPracticesHandler(db *sql.DB, auth *internalauth.Guard) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.Require(w, r) {
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		dormant, err := DormantPractices(r.Context(), tx, clock.Now(r.Context()).AddDate(-DormancyNoticeYears, 0, 0))
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, dormant)
	})
}
