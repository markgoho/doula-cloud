package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
)

// awaitingPageSize matches pageSize's own reasoning: a fixed size, no
// caller-supplied limit.
const awaitingPageSize = 30

// AwaitingReplyItem is one Engagement whose thread's latest Message was
// sent by the Client -- awaiting a staff reply.
type AwaitingReplyItem struct {
	EngagementID  string    `json:"engagementId"`
	ClientName    string    `json:"clientName"`
	LastMessageAt time.Time `json:"lastMessageAt"`
}

// AwaitingReplyResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4.
type AwaitingReplyResponse struct {
	Items      []AwaitingReplyItem `json:"items"`
	NextCursor *string             `json:"nextCursor,omitempty"`
	HasMore    bool                `json:"hasMore"`
}

// AwaitingReplyHandler lists, Practice-wide, every Engagement whose thread's
// latest Message was sent by the Client -- the roll-up #455 asks for so a
// doula does not have to open every Engagement in turn to find out who is
// waiting. Computed from thread authorship, not read state: ADR-0028 (#454)
// settled that there is no notification bell, and read state is inherently
// per-person while this roll-up is Practice-scoped. Narrowed by the same
// attachment rule staffauth.Reader.CanAccessEngagement enforces for
// ListHandler above -- a contractor Doula sees only an Engagement she holds
// an open, granted attachment on -- expressed here as the same
// engagement_attachments predicate that check runs, following
// client.listAttachedClients' own two-query-branch shape for a
// Practice-wide, single-subject-kind list narrowed for a contractor,
// rather than #485's activitygate (built for a feed spanning several
// subject kinds, which this roll-up is not). Must be mounted behind
// staffauth.Middleware.
func AwaitingReplyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		reader, err := staffauth.ResolveReader(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
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

		var items []AwaitingReplyItem
		if reader.IsContractor() {
			items, err = listAttachedAwaitingReply(r.Context(), tx, practiceID, staffID, after)
		} else {
			items, err = listAwaitingReply(r.Context(), tx, practiceID, after)
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		hasMore := len(items) > awaitingPageSize
		if hasMore {
			items = items[:awaitingPageSize]
		}
		resp := AwaitingReplyResponse{Items: items, HasMore: hasMore}
		if hasMore {
			last := items[len(items)-1]
			next := pagecursor.Encode(last.LastMessageAt, last.EngagementID)
			resp.NextCursor = &next
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// awaitingReplyLateral is shared by both queries below: for each candidate
// Engagement e, the latest Message on its thread (any sender), leaning on
// messages_engagement_created_at (00066) for the per-Engagement
// newest-first lookup. The sender_type = $n filter sits OUTSIDE this
// LATERAL, in the caller's own WHERE clause -- filtering inside it would
// pick "the latest Client message" rather than "the latest message,
// checked for being the Client's", which would wrongly surface an
// Engagement where staff already replied after the Client's last word.
const awaitingReplyLateral = `JOIN LATERAL (
		SELECT sender_type, created_at
		FROM messages
		WHERE messages.engagement_id = e.id
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	) m ON true`

// listAwaitingReply is the ambient-reach query -- Owner, Admin, or employee
// Doula -- filtered by practiceID explicitly, on top of the RLS scoping
// staffauth.Middleware already set up on tx.
func listAwaitingReply(ctx context.Context, tx *sql.Tx, practiceID string, after *pagecursor.Cursor) ([]AwaitingReplyItem, error) {
	query := `SELECT e.id, c.given_name, c.preferred_name, m.created_at
		FROM engagements e
		JOIN clients c ON c.id = e.client_id
		` + awaitingReplyLateral + `
		WHERE e.practice_id = $1 AND m.sender_type = $2`
	args := []any{practiceID, senderTypeClient}
	if after != nil {
		query += ` AND (m.created_at, e.id) < ($3, $4) ORDER BY m.created_at DESC, e.id DESC LIMIT $5`
		args = append(args, after.At, after.ID, awaitingPageSize+1)
	} else {
		query += ` ORDER BY m.created_at DESC, e.id DESC LIMIT $3`
		args = append(args, awaitingPageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("message: list awaiting reply: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAwaitingReplyItems(rows)
}

// listAttachedAwaitingReply is listAwaitingReply narrowed to Engagements
// staffID holds an open (ended_at IS NULL), granted-origin
// engagement_attachments row on -- the literal predicate
// staffauth.Reader.CanAccessEngagement runs, applied here as a SQL filter
// rather than a per-row round trip since this list is already homogeneous
// (Engagement only, unlike #486's cross-subject-kind feed).
func listAttachedAwaitingReply(ctx context.Context, tx *sql.Tx, practiceID, staffID string, after *pagecursor.Cursor) ([]AwaitingReplyItem, error) {
	query := `SELECT e.id, c.given_name, c.preferred_name, m.created_at
		FROM engagements e
		JOIN clients c ON c.id = e.client_id
		` + awaitingReplyLateral + `
		WHERE e.practice_id = $1 AND m.sender_type = $2
		  AND EXISTS (
		      SELECT 1 FROM engagement_attachments ea
		      WHERE ea.engagement_id = e.id AND ea.staff_id = $3
		        AND ea.origin = 'granted' AND ea.ended_at IS NULL
		  )`
	args := []any{practiceID, senderTypeClient, staffID}
	if after != nil {
		query += ` AND (m.created_at, e.id) < ($4, $5) ORDER BY m.created_at DESC, e.id DESC LIMIT $6`
		args = append(args, after.At, after.ID, awaitingPageSize+1)
	} else {
		query += ` ORDER BY m.created_at DESC, e.id DESC LIMIT $4`
		args = append(args, awaitingPageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("message: list attached awaiting reply: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAwaitingReplyItems(rows)
}

func scanAwaitingReplyItems(rows *sql.Rows) ([]AwaitingReplyItem, error) {
	items := []AwaitingReplyItem{}
	for rows.Next() {
		var item AwaitingReplyItem
		var givenName string
		var preferredName sql.NullString
		if err := rows.Scan(&item.EngagementID, &givenName, &preferredName, &item.LastMessageAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("message: scan awaiting reply item: %w", err)
		}
		item.ClientName = client.PreferredName(givenName, preferredName.String)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("message: iterate awaiting reply items: %w", err)
	}
	return items, nil
}
