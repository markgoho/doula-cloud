package engagementrequest

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

// pageSize is the fixed number of Requests returned per page, matching
// client.pageSize: a fixed size keeps the query parameter surface small.
// A fourteen-doula agency will rarely fill one page, but ADR-0017 puts no
// ceiling on how long a Request may wait, so the list is paginated like
// every other growing collection (docs/api-design.md section 4).
const pageSize = 30

// ListItem is one row of the pending-Request inbox: who the ask is about,
// what was asked for, and who asked when. The Client's name arrives
// already resolved -- the inbox names her, it does not print her record --
// and RequestID is what the row links to, the approval screen (#502)
// being addressed by the Request id alone.
type ListItem struct {
	RequestID       string    `json:"requestId"`
	ClientID        string    `json:"clientId"`
	ClientName      string    `json:"clientName"`
	Kind            string    `json:"kind"`
	DueDate         *string   `json:"dueDate,omitempty"`
	RequestedByName string    `json:"requestedByName"`
	RequestedAt     time.Time `json:"requestedAt"`
}

// ListResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4.
type ListResponse struct {
	Items      []ListItem `json:"items"`
	NextCursor *string    `json:"nextCursor,omitempty"`
	HasMore    bool       `json:"hasMore"`
}

// ListHandler lists every pending Engagement Request at the Practice --
// where they gather, so an approver at a fourteen-doula agency is not
// hunting through Client records one at a time (#503). Owner or Admin
// only, the same seat DetailHandler and the two decision endpoints hold:
// this is a queue of decisions, and a Doula reading decisions she cannot
// make is a screen nobody asked for. Must be mounted behind
// staffauth.Middleware.
//
// Ordered oldest first, which is what makes it a queue rather than a
// feed: ADR-0017's "a pending Request stops a Doula from doing any work
// at all" means the longest wait is the one that has cost the most, and
// it belongs at the top. Decided Requests never appear -- a decided
// Request is history, and its history lives on the Client's record.
func ListHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			// coverage:ignore reason: belt-and-braces -- engagementrequest.Mount's
			// own OwnerAndAdmin declaration (g.Get) already refuses a non-owner/
			// admin caller before this handler runs, so !ok is unreachable through
			// the real mount.
			return
		}

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		list, err := listPending(r.Context(), tx, practiceID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		hasMore := len(list) > pageSize
		if hasMore {
			list = list[:pageSize]
		}
		resp := ListResponse{Items: list, HasMore: hasMore}
		if hasMore {
			last := list[len(list)-1]
			next := pagecursor.Encode(last.RequestedAt, last.RequestID)
			resp.NextCursor = &next
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// listPending reads one page of pending Requests, oldest first. The
// cursor comparison is `>` rather than client.listClients' `<` because
// this list ascends; pagecursor carries a position, not a direction.
//
// practice_id is filtered explicitly on top of the RLS scoping
// staffauth.Middleware already set on tx, the same belt-and-braces
// client.listClients uses, and the join to clients is for her name only.
func listPending(ctx context.Context, tx *sql.Tx, practiceID string, after *pagecursor.Cursor) ([]ListItem, error) {
	query := `SELECT r.id, c.id, c.given_name, c.preferred_name, r.kind::text, r.due_date::text,
	                 rs.name, r.requested_at
	            FROM engagement_requests r
	            JOIN clients c ON c.id = r.client_id
	            JOIN staff rs ON rs.id = r.requested_by
	           WHERE r.practice_id = $1 AND r.state = 'pending'`
	args := []any{practiceID}
	if after != nil {
		query += ` AND (r.requested_at, r.id) > ($2, $3) ORDER BY r.requested_at, r.id LIMIT $4`
		args = append(args, after.At, after.ID, pageSize+1)
	} else {
		query += ` ORDER BY r.requested_at, r.id LIMIT $2`
		args = append(args, pageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("engagementrequest: list pending: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []ListItem{}
	for rows.Next() {
		var item ListItem
		var givenName string
		var preferredName, dueDate sql.NullString
		if err := rows.Scan(&item.RequestID, &item.ClientID, &givenName, &preferredName,
			&item.Kind, &dueDate, &item.RequestedByName, &item.RequestedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("engagementrequest: scan pending: %w", err)
		}
		item.ClientName = client.PreferredName(givenName, preferredName.String)
		if dueDate.Valid {
			item.DueDate = &dueDate.String
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("engagementrequest: iterate pending: %w", err)
	}
	return list, nil
}
