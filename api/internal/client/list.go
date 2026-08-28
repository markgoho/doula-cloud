package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/staffauth"
)

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
}

// ListHandler lists Clients at the current Practice, Client-shaped.
// Default filter is "Clients with work" -- a Client with at least one
// Engagement, or a pending Engagement Request -- per ADR-0017; ?all=true
// returns everyone. A contractor Doula's list is narrowed to Clients she
// holds an open, granted attachment to (through any of their
// Engagements) regardless of the filter, the same ADR-0008 carve-out
// Reader.CanAccessClient enforces. Must be mounted behind
// staffauth.Middleware.
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

		var list []ListItem
		if reader.IsContractor() {
			list, err = listAttachedClients(r.Context(), tx, practiceID, staffID, withWorkOnly)
		} else {
			list, err = listClients(r.Context(), tx, practiceID, withWorkOnly)
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(list); err != nil {
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

// listClients is the ambient-reach query -- Owner, Admin, or employee
// Doula -- filtered by practiceID explicitly, on top of the RLS scoping
// staffauth.Middleware already set up on tx.
func listClients(ctx context.Context, tx *sql.Tx, practiceID string, withWorkOnly bool) ([]ListItem, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT c.id, c.given_name, c.preferred_name, COALESCE(c.email, ''), `+hasWorkExpr+`,
		        pu.id IS NOT NULL, pu.identity_uid IS NOT NULL, latest.status
		 FROM clients c
		 LEFT JOIN client_portal_users pu ON pu.client_id = c.id
		 LEFT JOIN LATERAL (
		     SELECT o.status FROM portal_invite_outbox o
		     WHERE o.client_portal_user_id = pu.id
		     ORDER BY o.created_at DESC LIMIT 1
		 ) latest ON true
		 WHERE c.practice_id = $1 AND (NOT $2 OR `+hasWorkExpr+`)
		 ORDER BY COALESCE(c.preferred_name, c.given_name)`,
		practiceID, withWorkOnly,
	)
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
func listAttachedClients(ctx context.Context, tx *sql.Tx, practiceID, staffID string, withWorkOnly bool) ([]ListItem, error) {
	// DISTINCT is wrapped in a subquery rather than applied at the top
	// level: Postgres requires a plain SELECT DISTINCT's ORDER BY
	// expressions to appear literally in the select list, and sorting by
	// name needs client.PreferredName's given/preferred pair, not the
	// COALESCE expression this query used to select directly.
	rows, err := tx.QueryContext(ctx,
		`SELECT * FROM (
		     SELECT DISTINCT c.id, c.given_name, c.preferred_name, COALESCE(c.email, '') AS email, `+hasWorkExpr+` AS has_work,
		            pu.id IS NOT NULL AS has_portal_user, pu.identity_uid IS NOT NULL AS accepted, latest.status
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
		       AND (NOT $3 OR `+hasWorkExpr+`)
		 ) attached
		 ORDER BY COALESCE(preferred_name, given_name)`,
		practiceID, staffID, withWorkOnly,
	)
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
		if err := rows.Scan(
			&item.ClientID, &givenName, &preferredName, &item.Email, &item.HasWork,
			&hasPortalUser, &accepted, &outboxStatus,
		); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan list item: %w", err)
		}
		item.Name = PreferredName(givenName, preferredName.String)
		item.PortalInviteStatus = portalInviteStatus(hasPortalUser, accepted, outboxStatus)
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
