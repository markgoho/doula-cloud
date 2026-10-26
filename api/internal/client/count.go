package client

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// CountResponse is how many Client records the reader can be shown, and
// how many of those Clients are in active care: an Engagement whose status
// is active (the Glossary's Engagement entry). The Overview reads it for
// its Clients card (#2095) and for whether the Practice has any Client
// yet, which it used to answer by loading a whole page of the list.
type CountResponse struct {
	Records int `json:"records"`
	Active  int `json:"active"`
}

// countClientsQuery counts in one statement, over the same rows
// listClients reads with all=true: the Practice's records, less any
// record merged into another, which survives only as a pointer (ADR-0040).
// The active count is a semi-join on engagements_client_idx (00070,
// client_id then status), so a Client with two active Engagements counts
// once.
//
// Each subquery names e.practice_id = $1 itself rather than leaving the
// Practice to the engagements RLS policy: without it the planner priced
// the policy's own subplan at 31,173 (a third of jit_above_cost) and
// scanned every Engagement it could see; with it, a Bitmap Heap Scan on
// engagements_practice_idx (00059) and a cost of 281.
//
// Measured with EXPLAIN (ANALYZE, BUFFERS) as app_runtime under the
// Practice's RLS (2026-10-26). At 5,000 Clients and 2,500 Engagements for
// one Practice (TestCountClientsQueryPlanAtScale, count_perf_test.go), far
// past a fourteen-doula agency's few hundred records: Aggregate -> Seq
// Scan on clients, with one hashed SubPlan over a Bitmap Heap Scan on
// engagements; 1.2 ms planning, 3.2 ms execution, no JIT section. On the
// demo Practice (dev:full, Rooted Birth Collective: 33 records, 22 in
// active care): the same shape over a Bitmap Index Scan on
// engagements_practice_idx, 1.5 ms planning, 0.3 ms execution, five
// shared buffers. It is one row back to
// the BFF either way, where the list's page it replaces on the Overview
// was thirty Client rows with their joins.
const countClientsQuery = `SELECT count(*),
	       count(*) FILTER (WHERE c.id IN (SELECT e.client_id FROM engagements e WHERE e.practice_id = $1 AND e.status = 'active'))
	  FROM clients c
	 WHERE c.practice_id = $1 AND c.merged_into IS NULL`

// countAttachedClientsQuery is countClientsQuery narrowed the way
// listAttachedClients narrows the list (ADR-0008's contractor column): a
// contractor Doula is told how many Clients she holds an open, granted
// attachment to, never how many the Practice has.
const countAttachedClientsQuery = `SELECT count(*),
	       count(*) FILTER (WHERE c.id IN (SELECT e.client_id FROM engagements e WHERE e.practice_id = $1 AND e.status = 'active'))
	  FROM clients c
	 WHERE c.practice_id = $1 AND c.merged_into IS NULL
	   AND c.id IN (
	       SELECT e.client_id FROM engagements e
	         JOIN engagement_attachments ea ON ea.engagement_id = e.id
	        WHERE ea.staff_id = $2 AND ea.origin = 'granted' AND ea.ended_at IS NULL)`

// CountHandler serves CountResponse. Any Staff member, as the list is:
// the contractor narrowing happens inside, through staffauth.Reader, the
// same seat ListHandler decides it from. Must be mounted behind
// staffauth.Middleware.
func CountHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		resp, err := countClients(r.Context(), tx, practiceID, staffID, reader.IsAmbientContractor())
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
	if err := row.Scan(&resp.Records, &resp.Active); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return CountResponse{}, fmt.Errorf("client: count clients: %w", err)
	}
	return resp, nil
}
