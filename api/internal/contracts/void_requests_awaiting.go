package contracts

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
)

// voidRequestsAwaitingPageSize mirrors awaitingPageSize (awaiting.go):
// a work list somebody clears, not a report somebody scrolls.
const voidRequestsAwaitingPageSize = 30

// VoidRequestAwaitingItem is one row of the Practice-wide "void requests
// waiting on you" roll-up (#971): the Engagement and Client the request
// hangs off, who asked and why, and how long it has been waiting.
type VoidRequestAwaitingItem struct {
	RequestID       string    `json:"requestId"`
	EngagementID    string    `json:"engagementId"`
	ClientID        string    `json:"clientId"`
	ClientName      string    `json:"clientName"`
	RequestedByName string    `json:"requestedByName"`
	Reason          string    `json:"reason"`
	CreatedAt       time.Time `json:"createdAt"`
}

// VoidRequestsAwaitingResponse is the standard cursor-pagination envelope
// from docs/api-design.md section 4, matching AwaitingResponse.
type VoidRequestsAwaitingResponse struct {
	Items      []VoidRequestAwaitingItem `json:"items"`
	NextCursor *string                   `json:"nextCursor,omitempty"`
	HasMore    bool                      `json:"hasMore"`
}

// VoidRequestsAwaitingHandler lists every open void request at the
// Practice, oldest first -- #971's own "sees the void requests waiting
// on them", the same roll-up shape #426's AwaitingSignatureHandler
// already gives Contracts still needing a signature. Owner and Admin
// only: void itself is Owner/Admin-only (#970), so nobody else has
// anything to act on here. Must be mounted behind staffauth.Middleware.
func VoidRequestsAwaitingHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			// coverage:ignore reason: belt-and-braces -- contracts.Mount's own
			// OwnerAndAdmin declaration (g.Get) already refuses a non-owner/admin
			// caller before this handler runs, so !ok is unreachable through the
			// real mount.
			return
		}

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, MsgInvalidCursor, http.StatusBadRequest)
				return
			}
			after = &c
		}

		list, err := listVoidRequestsAwaiting(r.Context(), tx, practiceID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		hasMore := len(list) > voidRequestsAwaitingPageSize
		if hasMore {
			list = list[:voidRequestsAwaitingPageSize]
		}
		resp := VoidRequestsAwaitingResponse{Items: list, HasMore: hasMore}
		if hasMore {
			last := list[len(list)-1]
			next := pagecursor.Encode(last.CreatedAt, last.RequestID)
			resp.NextCursor = &next
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// listVoidRequestsAwaiting reads one page of open void requests, oldest
// first -- mirrors listAwaiting (awaiting.go). practice_id lives
// directly on contract_void_requests (00098), so no join back through
// contracts/engagements is needed for the Practice filter itself, only
// for the Client's name.
func listVoidRequestsAwaiting(ctx context.Context, tx *sql.Tx, practiceID string, after *pagecursor.Cursor) ([]VoidRequestAwaitingItem, error) {
	query := `SELECT cvr.id, cvr.engagement_id, cl.id, cl.given_name, cl.preferred_name, s.name, cvr.reason, cvr.created_at
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
		var preferredName sql.NullString
		if err := rows.Scan(&item.RequestID, &item.EngagementID, &item.ClientID,
			&givenName, &preferredName, &item.RequestedByName, &item.Reason, &item.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("contracts: scan void request awaiting: %w", err)
		}
		item.ClientName = client.PreferredName(givenName, preferredName.String)
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: iterate void requests awaiting: %w", err)
	}
	return list, nil
}
