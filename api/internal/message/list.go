package message

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/staffauth"
)

// pageSize is the fixed number of Messages returned per page -- no
// caller-supplied limit, keeping the query parameter surface small; a
// fixed size is enough for "paginated" to be true.
const pageSize = 30

// Message is one Message in a thread. Body and the attachment fields are
// scanned via COALESCE into zero values rather than sql.NullString/Int64:
// the schema (00008_messaging.sql) allows a NULL body for an
// attachment-only row and NULL attachment_* columns for a text-only row,
// and the "all or nothing" CHECK constraint means AttachmentFilename == ""
// is an unambiguous "no attachment" signal for the frontend, with no need
// for a pointer/NullString the API layer has no other use for.
// AttachmentContentType/AttachmentFilename are omitted from the JSON
// response when empty (no attachment) so existing text-only consumers see
// no shape change, per docs/api-design.md's additive-only rule.
type Message struct {
	MessageID             string    `json:"messageId"`
	SenderType            string    `json:"senderType"`
	SenderID              string    `json:"senderId"`
	SenderName            string    `json:"senderName"`
	Body                  string    `json:"body"`
	AttachmentContentType string    `json:"attachmentContentType,omitempty"`
	AttachmentFilename    string    `json:"attachmentFilename,omitempty"`
	CreatedAt             time.Time `json:"createdAt"`
}

// ListResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4.
type ListResponse struct {
	Items      []Message `json:"items"`
	NextCursor *string   `json:"nextCursor,omitempty"`
	HasMore    bool      `json:"hasMore"`
}

// ListHandler lists a thread's Messages, newest first, cursor-paginated.
// Newest-first (rather than oldest-first) so the first page a Staff member
// loads is the most recent activity, not the start of a possibly long-running
// Engagement; the frontend reverses for display. Narrowed by ADR-0008's
// attachment rule for a contractor Doula, same as engagement.ListHandler.
// Must be mounted behind staffauth.Middleware.
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
				http.Error(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		reader, err := staffauth.ResolveReader(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			http.Error(w, "engagement not found", http.StatusNotFound)
			return
		}

		var after *messageCursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := decodeCursor(raw)
			if err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		items, hasMore, err := listMessages(r.Context(), tx, engagementID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := ListResponse{Items: items, HasMore: hasMore}
		if hasMore {
			next := encodeCursor(items[len(items)-1].CreatedAt, items[len(items)-1].MessageID)
			resp.NextCursor = &next
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// listMessagesQuery and listMessagesAfterQuery share the same column list
// and JOINs; the only difference is the cursor's WHERE clause and LIMIT
// placeholder position, so two static queries are simpler and safer here
// than building one dynamically. sender_type is bound as $1/$2 rather than
// a literal 'staff'/'client' in the JOIN condition, so senderTypeStaff and
// senderTypeClient (context.go) are the single source of truth for those
// values across this package.
const listMessagesQuery = `SELECT m.id, m.sender_type, m.sender_id,
		COALESCE(s.name, c.name) AS sender_name, COALESCE(m.body, ''),
		COALESCE(m.attachment_content_type, ''), COALESCE(m.attachment_filename, ''), m.created_at
	FROM messages m
	LEFT JOIN staff s ON s.id = m.sender_id AND m.sender_type = $1
	LEFT JOIN clients c ON c.id = m.sender_id AND m.sender_type = $2
	WHERE m.engagement_id = $3
	ORDER BY m.created_at DESC, m.id DESC LIMIT $4`

const listMessagesAfterQuery = `SELECT m.id, m.sender_type, m.sender_id,
		COALESCE(s.name, c.name) AS sender_name, COALESCE(m.body, ''),
		COALESCE(m.attachment_content_type, ''), COALESCE(m.attachment_filename, ''), m.created_at
	FROM messages m
	LEFT JOIN staff s ON s.id = m.sender_id AND m.sender_type = $1
	LEFT JOIN clients c ON c.id = m.sender_id AND m.sender_type = $2
	WHERE m.engagement_id = $3 AND (m.created_at, m.id) < ($4, $5)
	ORDER BY m.created_at DESC, m.id DESC LIMIT $6`

// listMessages fetches one page of Messages under engagementID, filtered
// explicitly on top of the RLS scoping staffauth.Middleware already set
// up on tx -- the app layer's own filter, so a bug in either one alone
// can't leak rows. sender_id has no FK (it's polymorphic across staff and
// clients), so sender name resolution needs two LEFT JOINs gated on
// sender_type rather than visit.listVisits' single JOIN.
func listMessages(ctx context.Context, tx *sql.Tx, engagementID string, after *messageCursor) ([]Message, bool, error) {
	var rows *sql.Rows
	var err error
	if after != nil {
		rows, err = tx.QueryContext(ctx, listMessagesAfterQuery,
			senderTypeStaff, senderTypeClient, engagementID, after.createdAt, after.messageID, pageSize+1)
	} else {
		rows, err = tx.QueryContext(ctx, listMessagesQuery, senderTypeStaff, senderTypeClient, engagementID, pageSize+1)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, false, fmt.Errorf("message: list messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []Message{}
	for rows.Next() {
		var it Message
		if err := rows.Scan(&it.MessageID, &it.SenderType, &it.SenderID, &it.SenderName, &it.Body,
			&it.AttachmentContentType, &it.AttachmentFilename, &it.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, false, fmt.Errorf("message: scan message row: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, false, fmt.Errorf("message: iterate message rows: %w", err)
	}

	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}
	return items, hasMore, nil
}

// messageCursor is a page boundary: the (created_at, id) tuple of the
// last Message on the previous page, matching the DESC tiebreak
// listMessages orders by.
type messageCursor struct {
	createdAt time.Time
	messageID string
}

// encodeCursor packs a cursor as opaque base64 so callers never construct
// one by hand.
func encodeCursor(createdAt time.Time, messageID string) string {
	raw := createdAt.Format(time.RFC3339Nano) + "|" + messageID
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor reverses encodeCursor, rejecting anything malformed rather
// than letting a bad cursor silently return the wrong page.
func decodeCursor(s string) (messageCursor, error) {
	raw, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return messageCursor{}, fmt.Errorf("message: decode cursor: %w", err)
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return messageCursor{}, errors.New("message: malformed cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return messageCursor{}, fmt.Errorf("message: parse cursor timestamp: %w", err)
	}
	return messageCursor{createdAt: createdAt, messageID: parts[1]}, nil
}
