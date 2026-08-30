package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
)

// pageSize is the fixed number of Clients returned per page, matching
// message.pageSize's reasoning: a fixed size keeps the query parameter
// surface small.
const pageSize = 30

// ListItem is one row of the Client-shaped Clients list: one row per
// Client, never one per Client+Engagement pair (ADR-0017 -- a Client with
// two Engagements no longer appears twice).
type ListItem struct {
	ClientID string `json:"clientId"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	HasWork  bool   `json:"hasWork"`

	// PortalInviteStatus mirrors the pre-#397 engagement.ClientEngagement
	// field of the same name: nil when the Client was never invited,
	// "accepted" once identity_uid is set (taking precedence over
	// portal_invite_outbox, since accept.go never touches that row),
	// otherwise the most recent outbox row's status.
	PortalInviteStatus *string `json:"portalInviteStatus,omitempty"`

	// PendingRequestKinds is the kind ('birth', 'postpartum') of each of
	// this Client's pending Engagement Requests -- empty when she has
	// none. Can hold both at once: engagement_requests_one_pending is
	// keyed on (client_id, kind), so a Client can have a pending Request
	// of each kind at the same time (ADR-0017). This is what makes a
	// pending Request visible on the list row (#499) without a second
	// round trip to her detail history.
	PendingRequestKinds []string `json:"pendingRequestKinds,omitempty"`

	createdAt time.Time
}

// ListResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4.
type ListResponse struct {
	Items      []ListItem `json:"items"`
	NextCursor *string    `json:"nextCursor,omitempty"`
	HasMore    bool       `json:"hasMore"`
}

// ListHandler lists Clients at the current Practice, Client-shaped,
// cursor-paginated. Default filter is "Clients with work" -- a Client
// with at least one Engagement, or a pending Engagement Request -- per
// ADR-0017; ?all=true returns everyone. A contractor Doula's list is
// narrowed to Clients she holds an open, granted attachment to (through
// any of their Engagements) regardless of the filter, the same ADR-0008
// carve-out Reader.CanAccessClient enforces. Must be mounted behind
// staffauth.Middleware.
//
// Ordering moved from alphabetical (COALESCE(preferred_name, given_name))
// to (created_at, id) DESC -- #446: a cursor's tuple has to be the sort
// key, and pagecursor.Cursor only carries (time, id), so alphabetical
// order could not survive pagination without a second cursor shape the
// ticket forbids inventing.
func ListHandler() http.Handler {
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
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		withWorkOnly := r.URL.Query().Get("all") != "true"

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		var list []ListItem
		if reader.IsContractor() {
			list, err = listAttachedClients(r.Context(), tx, practiceID, staffID, withWorkOnly, after)
		} else {
			list, err = listClients(r.Context(), tx, practiceID, withWorkOnly, after)
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		hasMore := len(list) > pageSize
		if hasMore {
			list = list[:pageSize]
		}
		resp := ListResponse{Items: list, HasMore: hasMore}
		if hasMore {
			last := list[len(list)-1]
			next := pagecursor.Encode(last.createdAt, last.ClientID)
			resp.NextCursor = &next
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// hasWorkExpr is shared by both list queries below: a Client "has work"
// when she has at least one Engagement (any status) or a pending
// Engagement Request.
const hasWorkExpr = `(
	EXISTS (SELECT 1 FROM engagements e WHERE e.client_id = c.id)
	OR EXISTS (SELECT 1 FROM engagement_requests r WHERE r.client_id = c.id AND r.state = 'pending')
)`

// pendingRequestKindsExpr is also shared: the comma-joined kind of each of
// a Client's pending Requests, NULL when she has none. Postgres can serve
// this off engagement_requests_one_pending (client_id, kind) WHERE state =
// 'pending' -- the same partial index the ADR-0017 migration already
// carries for the one-pending-per-kind rule -- so this costs no new
// migration to stay quick against a fourteen-doula Practice's data.
const pendingRequestKindsExpr = `(
	SELECT string_agg(r.kind::text, ',' ORDER BY r.kind)
	FROM engagement_requests r WHERE r.client_id = c.id AND r.state = 'pending'
)`

// listClients is the ambient-reach query -- Owner, Admin, or employee
// Doula -- filtered by practiceID explicitly, on top of the RLS scoping
// staffauth.Middleware already set up on tx.
func listClients(ctx context.Context, tx *sql.Tx, practiceID string, withWorkOnly bool, after *pagecursor.Cursor) ([]ListItem, error) {
	query := `SELECT c.id, c.given_name, c.preferred_name, COALESCE(c.email, ''), ` + hasWorkExpr + `,
		        pu.id IS NOT NULL, pu.identity_uid IS NOT NULL, latest.status, ` + pendingRequestKindsExpr + `, c.created_at
		 FROM clients c
		 LEFT JOIN client_portal_users pu ON pu.client_id = c.id
		 LEFT JOIN LATERAL (
		     SELECT o.status FROM portal_invite_outbox o
		     WHERE o.client_portal_user_id = pu.id
		     ORDER BY o.created_at DESC LIMIT 1
		 ) latest ON true
		 WHERE c.practice_id = $1 AND (NOT $2 OR ` + hasWorkExpr + `)`
	args := []any{practiceID, withWorkOnly}
	if after != nil {
		query += ` AND (c.created_at, c.id) < ($3, $4) ORDER BY c.created_at DESC, c.id DESC LIMIT $5`
		args = append(args, after.At, after.ID, pageSize+1)
	} else {
		query += ` ORDER BY c.created_at DESC, c.id DESC LIMIT $3`
		args = append(args, pageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: list clients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanListItems(rows)
}

// listAttachedClients is listClients narrowed to Clients staffID holds an
// open (ended_at IS NULL), granted-origin engagement_attachments row on,
// via any of their Engagements -- ADR-0008's contractor column. DISTINCT
// because a contractor may be attached to more than one Engagement for
// the same Client.
func listAttachedClients(ctx context.Context, tx *sql.Tx, practiceID, staffID string, withWorkOnly bool, after *pagecursor.Cursor) ([]ListItem, error) {
	// DISTINCT is wrapped in a subquery rather than applied at the top
	// level: Postgres requires a plain SELECT DISTINCT's ORDER BY
	// expressions to appear literally in the select list, and sorting by
	// (created_at, id) needs the outer query's own comparison, not the
	// inner DISTINCT's column list.
	query := `SELECT * FROM (
		     SELECT DISTINCT c.id, c.given_name, c.preferred_name, COALESCE(c.email, '') AS email, ` + hasWorkExpr + ` AS has_work,
		            pu.id IS NOT NULL AS has_portal_user, pu.identity_uid IS NOT NULL AS accepted, latest.status,
		            ` + pendingRequestKindsExpr + ` AS pending_request_kinds, c.created_at
		     FROM clients c
		     JOIN engagements e ON e.client_id = c.id
		     JOIN engagement_attachments ea ON ea.engagement_id = e.id
		     LEFT JOIN client_portal_users pu ON pu.client_id = c.id
		     LEFT JOIN LATERAL (
		         SELECT o.status FROM portal_invite_outbox o
		         WHERE o.client_portal_user_id = pu.id
		         ORDER BY o.created_at DESC LIMIT 1
		     ) latest ON true
		     WHERE c.practice_id = $1 AND ea.staff_id = $2
		       AND ea.origin = 'granted' AND ea.ended_at IS NULL
		       AND (NOT $3 OR ` + hasWorkExpr + `)
		 ) attached`
	args := []any{practiceID, staffID, withWorkOnly}
	if after != nil {
		query += ` WHERE (created_at, id) < ($4, $5) ORDER BY created_at DESC, id DESC LIMIT $6`
		args = append(args, after.At, after.ID, pageSize+1)
	} else {
		query += ` ORDER BY created_at DESC, id DESC LIMIT $4`
		args = append(args, pageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: list attached clients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanListItems(rows)
}

func scanListItems(rows *sql.Rows) ([]ListItem, error) {
	list := []ListItem{}
	for rows.Next() {
		var item ListItem
		var givenName string
		var preferredName sql.NullString
		var hasPortalUser, accepted bool
		var outboxStatus sql.NullString
		var pendingKinds sql.NullString
		var createdAt time.Time
		if err := rows.Scan(
			&item.ClientID, &givenName, &preferredName, &item.Email, &item.HasWork,
			&hasPortalUser, &accepted, &outboxStatus, &pendingKinds, &createdAt,
		); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan list item: %w", err)
		}
		item.Name = PreferredName(givenName, preferredName.String)
		item.PortalInviteStatus = portalInviteStatus(hasPortalUser, accepted, outboxStatus)
		if pendingKinds.Valid {
			item.PendingRequestKinds = strings.Split(pendingKinds.String, ",")
		}
		item.createdAt = createdAt
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate list items: %w", err)
	}
	return list, nil
}

// portalInviteStatus derives ListItem.PortalInviteStatus, mirroring the
// pre-#397 engagement.portalInviteStatus logic exactly.
func portalInviteStatus(hasPortalUser, accepted bool, outboxStatus sql.NullString) *string {
	if !hasPortalUser {
		return nil
	}
	if accepted {
		status := "accepted"
		return &status
	}
	if outboxStatus.Valid {
		status := outboxStatus.String
		return &status
	}
	return nil
}
