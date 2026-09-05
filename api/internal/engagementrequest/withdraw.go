package engagementrequest

import (
	"database/sql"
	"errors"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// WithdrawHandler withdraws the caller's own pending Engagement Request.
// ADR-0017: withdraw exists because the alternative route out of a typo
// is asking an Admin to refuse it, which stamps a false "refused" on a
// woman's permanent record. Any Staff member may call this, but only on
// a Request she herself made -- unlike approve/refuse, no role gate here.
// Must be mounted behind staffauth.Middleware.
func WithdrawHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		requestID := r.PathValue("requestId")
		if !staffauth.ParseUUID(w, "request", requestID) {
			return
		}

		result, err := tx.ExecContext(r.Context(),
			// decided_by records the requester herself -- ADR-0017: "which
			// is honest -- she decided it -- and keeps the CHECK uniform."
			`UPDATE engagement_requests
			    SET state = 'withdrawn', decided_by = $1, decided_at = now()
			  WHERE id = $2 AND requested_by = $1 AND state = 'pending'`,
			staffID, requestID,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if rows == 0 {
			writeWithdrawErr(w, r, tx, requestID, staffID)
			return
		}

		writeJSON(w, http.StatusOK, DecisionResponse{RequestID: requestID, State: "withdrawn"})
	})
}

// writeWithdrawErr reports why the withdraw UPDATE affected zero rows:
// no such Request at this Practice (RLS scopes the read), one requested
// by someone else, or one already decided.
func writeWithdrawErr(w http.ResponseWriter, r *http.Request, tx *sql.Tx, requestID, staffID string) {
	var requestedBy, state string
	err := tx.QueryRowContext(r.Context(),
		`SELECT requested_by::text, state::text FROM engagement_requests WHERE id = $1`, requestID,
	).Scan(&requestedBy, &state)
	if errors.Is(err, sql.ErrNoRows) {
		apierr.WriteError(w, "engagement request not found", http.StatusNotFound)
		return
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if requestedBy != staffID {
		apierr.WriteError(w, "only the staff member who made this request may withdraw it", http.StatusForbidden)
		return
	}
	apierr.WriteError(w, "that request is no longer pending -- it is "+state, http.StatusConflict)
}
