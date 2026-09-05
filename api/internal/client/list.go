package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
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

	// EmailSuppressed is whether this Client's address currently sits on
	// email_suppressions with no cleared_at (#785, ADR-0029). It is what
	// separates "the invite failed, send another" from "the invite failed
	// and every further send to this address is refused before Mailgun is
	// asked" -- the outbox status alone cannot tell those apart, because
	// the same 'bounced'/'dead_lettered' value survives a Staff member
	// clearing the suppression on **Blocked email addresses** (#744).
	// Without it the list instructs Staff to re-invite an address whose
	// re-invite is guaranteed to dead-letter.
	EmailSuppressed bool `json:"emailSuppressed"`

	// PendingRequestKinds is the kind ('birth', 'postpartum') of each of
	// this Client's pending Engagement Requests -- empty when she has
	// none. Can hold both at once: engagement_requests_one_pending is
	// keyed on (client_id, kind), so a Client can have a pending Request
	// of each kind at the same time (ADR-0017). This is what makes a
	// pending Request visible on the list row (#499) without a second
	// round trip to her detail history.
	PendingRequestKinds []string `json:"pendingRequestKinds,omitempty"`

	// OpenEngagements is one line per open (non-`completed`) Engagement
	// this Client holds -- #264 (RA-G6): "what needs me today" without
	// opening each Engagement in turn. A Client can hold more than one
	// concurrent open Engagement (ADR-0017), so this is a list, never a
	// single flattened value. Empty/absent for a Client with none (every
	// Engagement completed, or only a pending Request -- already covered
	// by PendingRequestKinds above).
	OpenEngagements []OpenEngagement `json:"openEngagements,omitempty"`

	createdAt time.Time
}

// OpenEngagement is one rollup line of ListItem.OpenEngagements.
// ContractStatus and DoulaName are nil when no Contract has been created
// yet, or no Doula holds an open, granted attachment on this Engagement,
// respectively -- the frontend renders DoulaName's absence explicitly
// (matching how it already renders "Never invited" for
// ListItem.PortalInviteStatus's own nil), per the ticket's ask that an
// unset Doula never read as a blank cell.
//
// InvoiceStatus/InvoiceAmountCents and FeeCents are populated in SQL
// regardless of who is asking, then shaped away entirely (never merely
// blanked) by shapeOpenEngagement for a Reader ADR-0006/ADR-0008 bar from
// them -- the same "fetch full, shape by role" split
// contracts.ReadContract already uses for the same reason: Go, not SQL,
// is where the role check is easiest to see and to test.
type OpenEngagement struct {
	EngagementID     string  `json:"engagementId"`
	EngagementStatus string  `json:"engagementStatus"`
	ContractStatus   *string `json:"contractStatus,omitempty"`
	DoulaName        *string `json:"doulaName,omitempty"`

	// InvoiceStatus/InvoiceAmountCents: Owner and Admin only (ADR-0006).
	// Never set for an employee or contractor Doula, even when a
	// contractor is attached to this exact Engagement -- ADR-0008 gives
	// her only her own fee, never the Practice's Invoice.
	InvoiceStatus      *string `json:"invoiceStatus,omitempty"`
	InvoiceAmountCents *int64  `json:"invoiceAmountCents,omitempty"`

	// FeeCents is the Reader's own agreed fee (ADR-0008's
	// engagement_attachments.fee_amount_cents), set only when the Reader
	// is a contractor Doula holding an open, granted attachment on this
	// Engagement -- never another Doula's fee, and never set at all for
	// an employee Doula.
	FeeCents *int64 `json:"feeCents,omitempty"`
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
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		withWorkOnly := r.URL.Query().Get("all") != "true"

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
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
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		hasMore := len(list) > pageSize
		if hasMore {
			list = list[:pageSize]
		}

		if err := attachOpenEngagements(r.Context(), tx, list, reader, staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
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
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
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

// emailSuppressedExpr is shared by both list queries: whether this
// Client's address is suppressed right now (#785). Compared lower-cased
// against a column 00068 already stores lower-cased, the same comparison
// mailsuppress.Sender makes at send time -- the list must agree with the
// guard that will actually refuse the next invite. A Client with no
// address on file compares against ” and is never suppressed.
const emailSuppressedExpr = `EXISTS (
	SELECT 1 FROM email_suppressions s
	WHERE s.address = lower(COALESCE(c.email, '')) AND s.cleared_at IS NULL
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
		        pu.id IS NOT NULL, pu.identity_uid IS NOT NULL, latest.status, ` + emailSuppressedExpr + `, ` + pendingRequestKindsExpr + `, c.created_at
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
		            ` + emailSuppressedExpr + ` AS email_suppressed,
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

// rawOpenEngagement is one row of fetchOpenEngagements' unshaped read --
// every field SQL can produce, before shapeOpenEngagement decides which
// of them reader may actually see.
type rawOpenEngagement struct {
	clientID           string
	engagementID       string
	engagementStatus   string
	contractStatus     sql.NullString
	doulaName          sql.NullString
	invoiceStatus      sql.NullString
	invoiceAmountCents sql.NullInt64
	feeCents           sql.NullInt64
}

// attachOpenEngagements fills ListItem.OpenEngagements for every row of
// list, in one extra query keyed on the whole page's Client ids -- not
// one query per Engagement per Client -- so the rollup stays a bounded
// number of round trips against a 14-doula Practice's book, per #264's
// AC. A no-op when list is empty (the "See everyone" first page, or a
// Practice with no Clients yet).
func attachOpenEngagements(ctx context.Context, tx *sql.Tx, list []ListItem, reader staffauth.Reader, staffID string) error {
	if len(list) == 0 {
		return nil
	}
	clientIDs := make([]string, len(list))
	for i, item := range list {
		clientIDs[i] = item.ClientID
	}
	raws, err := fetchOpenEngagements(ctx, tx, clientIDs, staffID, reader.IsContractor())
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	byClient := map[string][]rawOpenEngagement{}
	for _, raw := range raws {
		byClient[raw.clientID] = append(byClient[raw.clientID], raw)
	}
	for i := range list {
		for _, raw := range byClient[list[i].ClientID] {
			list[i].OpenEngagements = append(list[i].OpenEngagements, shapeOpenEngagement(reader, raw))
		}
	}
	return nil
}

// openEngagementRollupQueryTemplate is fetchOpenEngagements' query, with
// two %-verbs standing in for placeholder indices that shift depending on
// how many clientIDs the caller passes -- staffIdx and attachedOnlyIdx
// below always come after every clientID placeholder, never before.
const openEngagementRollupQueryTemplate = `
	SELECT e.client_id, e.id, e.status, ct.status,
	       (SELECT string_agg(s.name, ', ' ORDER BY s.name)
	        FROM engagement_attachments ea
	        JOIN staff s ON s.id = ea.staff_id
	        WHERE ea.engagement_id = e.id AND ea.origin = 'granted' AND ea.ended_at IS NULL),
	       inv.status, inv.amount_cents, att.fee_amount_cents
	FROM engagements e
	LEFT JOIN LATERAL (
	    -- At most one Contract row per Engagement, because there can be
	    -- several. 00020_contracts_recreate_after_void.sql dropped the
	    -- table-wide UNIQUE (engagement_id) on purpose and put a partial
	    -- unique index in its place -- one non-voided row, and "any
	    -- number of voided rows" -- so #72's void-then-recreate flow
	    -- gives an Engagement a voided Contract plus a fresh Draft. A
	    -- plain join then returns both, and this function emits two
	    -- OpenEngagements carrying the same engagementId, which the
	    -- route's {#each ... (line.engagementId)} rejects as a duplicate
	    -- key -- a hard error in production, not a degraded row.
	    --
	    -- The ordering prefers a live Contract and falls back to a
	    -- voided one rather than filtering voided out: an Engagement
	    -- whose only Contract is voided must still show that, since
	    -- contractStatusLabel maps the voided status and "no Contract
	    -- yet" is a different answer for the reader than "the Contract
	    -- was voided".
	    SELECT c.id, c.status FROM contracts c
	    WHERE c.engagement_id = e.id
	    ORDER BY (c.status = 'voided'), c.created_at DESC LIMIT 1
	) ct ON true
	LEFT JOIN LATERAL (
	    SELECT i.status, i.amount_cents FROM invoices i
	    WHERE i.contract_id = ct.id ORDER BY i.created_at DESC LIMIT 1
	) inv ON true
	LEFT JOIN engagement_attachments att
	    ON att.engagement_id = e.id AND att.staff_id = $%d
	   AND att.origin = 'granted' AND att.ended_at IS NULL
	WHERE e.client_id IN (%s) AND e.status <> 'completed'
	  AND (NOT $%d OR att.id IS NOT NULL)
	ORDER BY e.client_id, e.created_at`

// fetchOpenEngagements reads every open (non-`completed`) Engagement
// belonging to any of clientIDs, with its Contract's status, its
// attached Doula(s)' names, its latest Invoice's status/amount, and
// staffID's own fee on it if she is attached -- one query regardless of
// how many Clients or Engagements the page holds.
//
// staffID is always the caller's own id, whatever her role: passing it
// unconditionally (rather than only when she is a contractor) is what
// lets an owner-contractor's (ADR-0017's "solo Practice") own fee join
// through the same LEFT JOIN an actual contractor's does, with no second
// code path to keep in sync. attachedOnly narrows the result set itself
// to Engagements staffID is attached to (true for a contractor's own
// list, per staffauth.Reader.CanAccessEngagement's "only what she is
// attached to") -- false lets every open Engagement on the Client
// through regardless of attachment, for every other role. Which of the
// fetched fields actually reach the caller is shapeOpenEngagement's job,
// not this query's: fetching InvoiceStatus/FeeCents unconditionally
// mirrors contracts.ReadContract's own "fetch full, shape by role" split.
func fetchOpenEngagements(ctx context.Context, tx *sql.Tx, clientIDs []string, staffID string, attachedOnly bool) ([]rawOpenEngagement, error) {
	placeholders := make([]string, len(clientIDs))
	args := make([]any, 0, len(clientIDs)+2)
	for i, id := range clientIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, id)
	}
	staffIdx := len(args) + 1
	attachedOnlyIdx := len(args) + 2
	args = append(args, staffID, attachedOnly)

	query := fmt.Sprintf(openEngagementRollupQueryTemplate, staffIdx, strings.Join(placeholders, ", "), attachedOnlyIdx) //nolint:gosec // every interpolated value is a numeric placeholder index this function computed itself, never request input -- the actual clientIDs/staffID/attachedOnly values ride the args slice as bind parameters below

	rows, err := tx.QueryContext(ctx, query, args...) //nolint:gosec // query is built from placeholder indices only (see the nolint above); every real value is bound through args
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: fetch open engagements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []rawOpenEngagement
	for rows.Next() {
		var raw rawOpenEngagement
		if err := rows.Scan(
			&raw.clientID, &raw.engagementID, &raw.engagementStatus, &raw.contractStatus,
			&raw.doulaName, &raw.invoiceStatus, &raw.invoiceAmountCents, &raw.feeCents,
		); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan open engagement: %w", err)
		}
		out = append(out, raw)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate open engagements: %w", err)
	}
	return out, nil
}

// shapeOpenEngagement is the one place raw's Invoice and fee fields turn
// into what reader is actually entitled to see, per ADR-0006/ADR-0008:
// Invoice status/money reaches an Owner or Admin only; a contractor's own
// fee reaches a contractor only (raw.feeCents is already staffID's own
// row, never another Doula's, by construction of fetchOpenEngagements'
// join). Contract status, Doula name, and Engagement status carry no
// gate -- every role on ADR-0006's table may read them.
func shapeOpenEngagement(reader staffauth.Reader, raw rawOpenEngagement) OpenEngagement {
	oe := OpenEngagement{
		EngagementID:     raw.engagementID,
		EngagementStatus: raw.engagementStatus,
	}
	if raw.contractStatus.Valid {
		status := raw.contractStatus.String
		oe.ContractStatus = &status
	}
	if raw.doulaName.Valid {
		name := raw.doulaName.String
		oe.DoulaName = &name
	}
	if reader.Has("owner") || reader.Has("admin") {
		if raw.invoiceStatus.Valid {
			status := raw.invoiceStatus.String
			oe.InvoiceStatus = &status
		}
		if raw.invoiceAmountCents.Valid {
			amount := raw.invoiceAmountCents.Int64
			oe.InvoiceAmountCents = &amount
		}
	}
	if reader.IsContractor() && raw.feeCents.Valid {
		fee := raw.feeCents.Int64
		oe.FeeCents = &fee
	}
	return oe
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
			&hasPortalUser, &accepted, &outboxStatus, &item.EmailSuppressed, &pendingKinds, &createdAt,
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
