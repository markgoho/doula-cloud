package billing

import (
	"database/sql"
	"net/http"
	"time"

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

// MonthlyCreditsHandler is the founder's monthly read of the two counts
// ADR-0046 says the product keeps: Credits bought and Credits spent, one
// row per Practice per UTC month, from her creation month through the
// month asked for (?month=YYYY-MM, the current month at the latest).
// docs/environment.md, "The monthly Credits read", says how it is called
// and which spreadsheet columns its rows fill.
func MonthlyCreditsHandler(db *sql.DB, auth *internalauth.Guard) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.Require(w, r) {
			return
		}

		month, err := time.Parse(MonthLayout, r.URL.Query().Get("month"))
		if err != nil {
			apierr.WriteError(w, "Give the month as YYYY-MM.", http.StatusBadRequest)
			return
		}
		if month.After(monthStart(clock.Now(r.Context()))) {
			apierr.WriteError(w, "The month cannot be in the future.", http.StatusBadRequest)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		rows, err := MonthlyCredits(r.Context(), tx, month)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, rows)
	})
}
