package client

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// DoulaCountsResponse is how many Clients each Doula holds a live attachment
// on, for the Clients list's "Doula" filter and its "Only my Clients" switch.
// Counts only, keyed by Staff id: no Client name or id leaves here. A Doula
// with no live Client is absent from Doulas, so an absent key reads as zero.
type DoulaCountsResponse struct {
	// Mine is the reader's own count.
	Mine int `json:"mine"`
	// Doulas is the count for each Staff member who holds at least one. An
	// ambient contractor is told only about herself.
	Doulas map[string]int `json:"doulas"`
}

// doulaCountsQuery counts, per Staff member, the Clients with an Engagement
// that is not completed on which she holds an open, granted attachment: the
// rule ?doula= applies (doulaExpr), less the merged records the list never
// shows. DISTINCT because one Client can hold two such Engagements.
// $2 narrows to one Staff member, or is NULL for all of them.
const doulaCountsQuery = `SELECT da.staff_id, count(DISTINCT c.id)
	  FROM clients c
	  JOIN engagements de ON de.client_id = c.id
	  JOIN engagement_attachments da ON da.engagement_id = de.id
	 WHERE c.practice_id = $1 AND c.merged_into IS NULL
	   AND de.status <> 'completed'
	   AND da.origin = 'granted' AND da.ended_at IS NULL
	   AND ($2::uuid IS NULL OR da.staff_id = $2::uuid)
	 GROUP BY da.staff_id`

// DoulaCountsHandler serves DoulaCountsResponse. Any Staff member, as the
// list is: an ambient contractor is narrowed inside, to her own count, since
// she does not see another Doula's list. Must be mounted behind
// staffauth.Middleware.
func DoulaCountsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, reader, ok := staffauth.RequireTxReader(w, r)
		if !ok {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		resp, err := countDoulaClients(r.Context(), tx, practiceID, staffID, reader.IsAmbientContractor())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

func countDoulaClients(ctx context.Context, tx *sql.Tx, practiceID, staffID string, onlyHers bool) (DoulaCountsResponse, error) {
	var only any
	if onlyHers {
		only = staffID
	}
	rows, err := tx.QueryContext(ctx, doulaCountsQuery, practiceID, only)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DoulaCountsResponse{}, fmt.Errorf("client: count doula clients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	resp := DoulaCountsResponse{Doulas: map[string]int{}}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return DoulaCountsResponse{}, fmt.Errorf("client: scan doula clients: %w", err)
		}
		resp.Doulas[id] = n
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB iteration failure, not exercised by unit tests
		return DoulaCountsResponse{}, fmt.Errorf("client: read doula clients: %w", err)
	}
	resp.Mine = resp.Doulas[staffID]
	return resp, nil
}
