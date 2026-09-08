package engagementrequest

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// RefuseRequest carries the required reason -- engagement_requests_refusal_reason
// (00042) enforces it in the database too, so a violation here can never
// slip past Go into a row with no reason.
type RefuseRequest struct {
	Reason string `json:"reason"`
}

// RefuseHandler refuses a pending Engagement Request. Owner or Admin
// only; a reason is required. Must be mounted behind staffauth.Middleware.
//
// Owner and Admin is declared at the mount, not checked here (#1016,
// following #970's and #990's own move): the handler no longer calls
// staffauth.RequireOwnerOrAdmin, because a Doula is refused by the gate
// before this runs. Widening or narrowing this route means editing its
// role list in mount.go.
func RefuseHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		approverStaffID, _ := staffauth.StaffID(r.Context())

		requestID := r.PathValue("requestId")
		if !staffauth.ParseUUID(w, "request", requestID) {
			return
		}

		var body RefuseRequest
		if !apierr.DecodeJSON(w, r, &body) {
			return
		}
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			apierr.WriteError(w, "reason is required", http.StatusBadRequest)
			return
		}

		result, err := tx.ExecContext(r.Context(),
			`UPDATE engagement_requests
			    SET state = 'refused', decided_by = $1, decided_at = now(), reason = $2
			  WHERE id = $3 AND state = 'pending'`,
			approverStaffID, reason, requestID,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if rows == 0 {
			writeRequestNotDecidable(w, r, tx, requestID)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, DecisionResponse{RequestID: requestID, State: "refused"})
	})
}

// writeRequestNotDecidable reports why a Request could not be acted on:
// no such Request at this Practice (RLS already scopes the read), or one
// that has already been decided or withdrawn. Shared by the decision
// handlers, where it explains an UPDATE that affected zero rows, and by
// DetailHandler, where it explains a Request that is no longer pending --
// the same two answers either way.
func writeRequestNotDecidable(w http.ResponseWriter, r *http.Request, tx *sql.Tx, requestID string) {
	var state string
	err := tx.QueryRowContext(r.Context(), `SELECT state::text FROM engagement_requests WHERE id = $1`, requestID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		apierr.WriteError(w, "engagement request not found", http.StatusNotFound)
		return
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	apierr.WriteError(w, "that request is no longer pending -- it is "+state, http.StatusConflict)
}
