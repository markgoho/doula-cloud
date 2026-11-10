package client

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// CountResponse is how many Client records the reader can be shown. The
// Overview reads it for whether the Practice has any Client yet (#2095),
// which it used to answer by loading a whole page of the list. Its count
// of Clients in active care went with the Quick links that read it
// (#2401).
type CountResponse struct {
	Records int `json:"records"`
}

// countClientsQuery counts the same rows listClients reads with
// all=true: the Practice's records, less any record merged into another,
// which survives only as a pointer (ADR-0040). One row back to the BFF,
// where the list's page it replaces on the Overview was thirty Client
// rows with their joins. TestCountClientsQueryPlanAtScale prices it.
const countClientsQuery = `SELECT count(*)
	  FROM clients c
	 WHERE c.practice_id = $1 AND c.standing IN ('live', 'erased')`

// countAttachedClientsQuery is countClientsQuery narrowed the way
// listAttachedClients narrows the list (ADR-0008's contractor column): a
// contractor Doula is told how many Clients she holds an open, granted
// attachment to, never how many the Practice has.
const countAttachedClientsQuery = `SELECT count(*)
	  FROM clients c
	 WHERE c.practice_id = $1 AND c.standing IN ('live', 'erased')
	   AND c.id IN (
	       SELECT e.client_id FROM engagements e
	         JOIN engagement_attachments ea ON ea.engagement_id = e.id
	        WHERE ea.staff_id = $2 AND ea.origin = 'granted' AND ea.ended_at IS NULL)`

// CountHandler serves CountResponse. Any Staff member, as the list is:
// the contractor narrowing happens inside, through staffauth.Reader, the
// same seat ListHandler decides it from. Must be mounted behind
// staffauth.Middleware.
func CountHandler() http.Handler {
	return countHandler(countClients)
}

// countHandler serves what count reads for the caller: her Practice, her
// id, and whether she is narrowed to her own Clients. Shared by
// CountHandler and DoulaCountsHandler, which differ only in the count.
func countHandler[T any](count func(ctx context.Context, tx *sql.Tx, practiceID, staffID string, narrowed bool) (T, error)) http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		resp, err := count(r.Context(), c.Tx, c.PracticeID, c.StaffID, c.Reader.IsAmbientContractor())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

func countClients(ctx context.Context, tx *sql.Tx, practiceID, staffID string, isAttachedOnly bool) (CountResponse, error) {
	var resp CountResponse
	var row *sql.Row
	if isAttachedOnly {
		row = tx.QueryRowContext(ctx, countAttachedClientsQuery, practiceID, staffID)
	} else {
		row = tx.QueryRowContext(ctx, countClientsQuery, practiceID)
	}
	if err := row.Scan(&resp.Records); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return CountResponse{}, fmt.Errorf("client: count clients: %w", err)
	}
	return resp, nil
}
