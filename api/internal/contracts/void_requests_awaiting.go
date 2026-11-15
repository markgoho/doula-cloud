package contracts

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/personname"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/staffauth"
)

// voidRequestsAwaitingPageSize is the number of void requests per page: a
// work list somebody clears, not a report somebody scrolls.
const voidRequestsAwaitingPageSize = 30

// VoidRequestAwaitingItem is one row of the Practice-wide "void requests
// waiting on you" roll-up (#971): the Contract, Engagement and Client the
// request hangs off, who asked and why, and how long it has been waiting.
type VoidRequestAwaitingItem struct {
	RequestID       string `json:"requestId"`
	ContractID      string `json:"contractId"`
	EngagementID    string `json:"engagementId"`
	ClientID        string `json:"clientId"`
	ClientName      string `json:"clientName"`
	RequestedByName string `json:"requestedByName"`
	Reason          string `json:"reason"`
	// DaysAgo is the whole Practice days from the request to now: 0 for today.
	DaysAgo   int       `json:"daysAgo"`
	CreatedAt time.Time `json:"createdAt"`
}

// VoidRequestsAwaitingResponse is the standard cursor-pagination envelope
// from docs/api-design.md section 4, plus the count of every open request.
type VoidRequestsAwaitingResponse struct {
	Items      []VoidRequestAwaitingItem `json:"items"`
	NextCursor *string                   `json:"nextCursor,omitempty"`
	HasMore    bool                      `json:"hasMore"`
	Total      int                       `json:"total"`
}

// VoidRequestsAwaitingHandler lists every open void request at the
// Practice, oldest first -- #971's own "sees the void requests waiting
// on them". Owner and Admin only: void itself is Owner/Admin-only (#970), so
// nobody else has anything to act on here. Must be mounted behind
// staffauth.Middleware.
func VoidRequestsAwaitingHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		ctx := r.Context()
		zone, err := practicetimezone.Load(ctx, c.Tx, c.PracticeID)
		if err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		now := clock.Now(ctx)

		page, ok := pagecursor.Fetch(w, r, pagecursor.MsgInvalidCursor, voidRequestsAwaitingPageSize,
			func(after *pagecursor.Cursor) ([]VoidRequestAwaitingItem, error) {
				return listVoidRequestsAwaiting(ctx, c.Tx, c.PracticeID, after, zone, now)
			},
			func(v VoidRequestAwaitingItem) (time.Time, string) { return v.CreatedAt, v.RequestID })
		if !ok {
			return
		}

		var total int
		if err := c.Tx.QueryRowContext(ctx,
			`SELECT count(*) FROM contract_void_requests WHERE practice_id = $1 AND status = 'open'`, c.PracticeID,
		).Scan(&total); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, VoidRequestsAwaitingResponse{Items: page.Items, HasMore: page.HasMore, NextCursor: page.Next, Total: total})
	})
}

// listVoidRequestsAwaiting reads one page of open void requests, oldest
// first. practice_id lives directly on contract_void_requests (00098), so no
// join back through contracts/engagements is needed for the Practice filter
// itself, only for the Client's name.
func listVoidRequestsAwaiting(ctx context.Context, tx *sql.Tx, practiceID string, after *pagecursor.Cursor, zone *time.Location, now time.Time) ([]VoidRequestAwaitingItem, error) {
	query := `SELECT cvr.id, cvr.contract_id, cvr.engagement_id, cl.id, cl.given_name, cl.preferred_name, cl.family_name, s.name, cvr.reason, cvr.created_at
	            FROM contract_void_requests cvr
	            JOIN engagements e ON e.id = cvr.engagement_id
	            JOIN clients cl ON cl.id = e.client_id
	            JOIN staff s ON s.id = cvr.requested_by
	           WHERE cvr.practice_id = $1 AND cvr.status = 'open'`
	args := []any{practiceID}
	if after != nil {
		query += ` AND (cvr.created_at, cvr.id) > ($2, $3) ORDER BY cvr.created_at, cvr.id LIMIT $4`
		args = append(args, after.At, after.ID, voidRequestsAwaitingPageSize+1)
	} else {
		query += ` ORDER BY cvr.created_at, cvr.id LIMIT $2`
		args = append(args, voidRequestsAwaitingPageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: list void requests awaiting: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []VoidRequestAwaitingItem{}
	for rows.Next() {
		var item VoidRequestAwaitingItem
		var givenName string
		var preferredName, familyName sql.NullString
		if err := rows.Scan(&item.RequestID, &item.ContractID, &item.EngagementID, &item.ClientID,
			&givenName, &preferredName, &familyName, &item.RequestedByName, &item.Reason, &item.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("contracts: scan void request awaiting: %w", err)
		}
		item.ClientName = personname.Full(givenName, preferredName.String, familyName.String)
		item.DaysAgo = practiceDaysBetween(item.CreatedAt, now, zone)
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: iterate void requests awaiting: %w", err)
	}
	return list, nil
}
