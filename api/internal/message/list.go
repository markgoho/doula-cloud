package message

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/pagecursor"
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
				apierr.WriteError(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}

		var after *messageCursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := decodeCursor(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		// "" is the Staff-side reader: unchanged from before
		// unresolvedStaffSenderName existed (see listMessages' own doc
		// comment).
		items, hasMore, err := listMessages(r.Context(), tx, engagementID, after, "")
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := ListResponse{Items: items, HasMore: hasMore}
		if hasMore {
			next := encodeCursor(items[len(items)-1].CreatedAt, items[len(items)-1].MessageID)
			resp.NextCursor = &next
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
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
		s.name, c.given_name, c.preferred_name, COALESCE(m.body, ''),
		COALESCE(m.attachment_content_type, ''), COALESCE(m.attachment_filename, ''), m.created_at
	FROM messages m
	LEFT JOIN staff s ON s.id = m.sender_id AND m.sender_type = $1
	LEFT JOIN clients c ON c.id = m.sender_id AND m.sender_type = $2
	WHERE m.engagement_id = $3
	ORDER BY m.created_at DESC, m.id DESC LIMIT $4`

const listMessagesAfterQuery = `SELECT m.id, m.sender_type, m.sender_id,
		s.name, c.given_name, c.preferred_name, COALESCE(m.body, ''),
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
//
// unresolvedStaffSenderName is what a Staff sender's row renders as when
// the LEFT JOIN above finds no row. For a Client-facing caller this is
// 00111's client_portal_sees_staff policy refusing a Doula who deleted
// her own login (ADR-0033) rather than reaching her redacted row.
// ListHandler (Staff-facing) passes "" -- unchanged from what the code
// produced before this parameter existed (#1198 AC2), and, confirmed by
// TestListHandler_UnchangedForASenderWhoDeletedHerLogin
// (handlers_test.go), the branch is very much reachable from that side
// too: staff_practice_visibility (00002) reaches a staff row only
// through a live practice_memberships row, and RemoveMembership deletes
// that row for a plain departure exactly as ADR-0033's login deletion
// does, so a Staff reader gets an empty name here as well -- not the
// redacted "Deleted Staff Member" this ticket assumed. That gap is real
// and is filed separately (#1322) rather than fixed here, since fixing
// it means changing what the Staff side shows, which AC2 forbids.
// ClientListHandler (#1198) passes activity.StaffActorDisplayName, the
// same word listPortalVisits already stands in with for the identical
// gap on the Client-portal side.
func listMessages(ctx context.Context, tx *sql.Tx, engagementID string, after *messageCursor, unresolvedStaffSenderName string) ([]Message, bool, error) {
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
		var staffName, clientGivenName, clientPreferredName sql.NullString
		if err := rows.Scan(&it.MessageID, &it.SenderType, &it.SenderID,
			&staffName, &clientGivenName, &clientPreferredName, &it.Body,
			&it.AttachmentContentType, &it.AttachmentFilename, &it.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, false, fmt.Errorf("message: scan message row: %w", err)
		}
		switch {
		case staffName.Valid:
			it.SenderName = staffName.String
		case it.SenderType == senderTypeStaff:
			it.SenderName = unresolvedStaffSenderName
		default:
			it.SenderName = client.PreferredName(clientGivenName.String, clientPreferredName.String)
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
// one by hand. The packing is pagecursor's, shared with offer and
// payments.
func encodeCursor(createdAt time.Time, messageID string) string {
	return pagecursor.Encode(createdAt, messageID)
}

// decodeCursor reverses encodeCursor, rejecting anything malformed rather
// than letting a bad cursor silently return the wrong page.
func decodeCursor(s string) (messageCursor, error) {
	c, err := pagecursor.Decode(s)
	if err != nil {
		return messageCursor{}, fmt.Errorf("message: decode cursor: %w", err)
	}
	return messageCursor{createdAt: c.At, messageID: c.ID}, nil
}
