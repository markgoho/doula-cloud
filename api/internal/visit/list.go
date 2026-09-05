package visit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
)

// pageSize is the fixed number of Visits returned per page, matching
// message.pageSize's reasoning.
const pageSize = 30

// Visit is one row of a Visit list: who it's assigned to and when it was
// created.
type Visit struct {
	VisitID   string    `json:"visitId"`
	StaffID   string    `json:"staffId"`
	StaffName string    `json:"staffName"`
	CreatedAt time.Time `json:"createdAt"`
}

// ListResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4.
type ListResponse struct {
	Items      []Visit `json:"items"`
	NextCursor *string `json:"nextCursor,omitempty"`
	HasMore    bool    `json:"hasMore"`
}

// ListHandler lists Visits under an Engagement, most recent first,
// cursor-paginated, regardless of which Doula it's assigned to -- same
// "any Staff at the Practice can see it" visibility as
// engagement.ListHandler, narrowed the same way by ADR-0008's attachment
// rule for a contractor Doula. Must be mounted behind
// staffauth.Middleware.
//
// Ordering flipped from ascending (oldest first) to (created_at, id) DESC
// -- #446, matching docs/api-design.md section 4's own worked example,
// which is literally this query. The frontend reverses a page for
// display, the same pattern message.ListHandler already established.
func ListHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		reader, err := staffauth.ResolveReader(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
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

		list, err := listVisits(r.Context(), tx, engagementID, after)
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
			next := pagecursor.Encode(last.CreatedAt, last.VisitID)
			resp.NextCursor = &next
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// listVisits is filtered by engagementID explicitly, on top of the RLS
// scoping staffauth.Middleware already set up on tx -- the app layer's own
// filter, so a bug in either one alone can't leak rows.
func listVisits(ctx context.Context, tx *sql.Tx, engagementID string, after *pagecursor.Cursor) ([]Visit, error) {
	query := `SELECT v.id, s.id, s.name, v.created_at
		 FROM visits v
		 JOIN staff s ON s.id = v.staff_id
		 WHERE v.engagement_id = $1`
	args := []any{engagementID}
	if after != nil {
		query += ` AND (v.created_at, v.id) < ($2, $3) ORDER BY v.created_at DESC, v.id DESC LIMIT $4`
		args = append(args, after.At, after.ID, pageSize+1)
	} else {
		query += ` ORDER BY v.created_at DESC, v.id DESC LIMIT $2`
		args = append(args, pageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("visit: list visits: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []Visit{}
	for rows.Next() {
		var v Visit
		if err := rows.Scan(&v.VisitID, &v.StaffID, &v.StaffName, &v.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("visit: scan visit row: %w", err)
		}
		list = append(list, v)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: iterate visit rows: %w", err)
	}
	return list, nil
}
