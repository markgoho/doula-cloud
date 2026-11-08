package engagementrequest

import (
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/dbtx"
	"doula-cloud/api/internal/staffauth"
)

// WithdrawHandler withdraws the caller's own pending Engagement Request.
// ADR-0017: withdraw exists because the alternative route out of a typo
// is asking an Admin to refuse it, which stamps a false "refused" on a
// woman's permanent record. Any Staff member may call this, but only on
// a Request she herself made -- unlike approve/refuse, no role gate here.
// Must be mounted behind staffauth.Middleware.
func WithdrawHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		tx, staffID, requestID := c.Tx, c.StaffID, c.ID("requestId")

		rows, ok := dbtx.ExecRows(r.Context(), w, tx,
			// decided_by records the requester herself -- ADR-0017: "which
			// is honest -- she decided it -- and keeps the CHECK uniform."
			`UPDATE engagement_requests
			    SET state = 'withdrawn', decided_by = $1, decided_at = now()
			  WHERE id = $2 AND requested_by = $1 AND state = 'pending'`,
			staffID, requestID,
		)
		if !ok {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}
		if rows == 0 {
			writeRequestNotActionable(w, r, tx, requestID, staffID)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, DecisionResponse{RequestID: requestID, State: "withdrawn"})
	}, staffauth.PathID("request", "requestId"))
}
