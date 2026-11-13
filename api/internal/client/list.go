package client

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/invoicestatus"
	"doula-cloud/api/internal/overdue"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/paymentstanding"
	"doula-cloud/api/internal/personname"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/textmatch"
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
	// Name is personname.Full (#2111): the list sets every Client side by
	// side, and two who share a first name must read apart. The order is
	// not by name (ListHandler's own comment, #446).
	Name    string `json:"name"`
	Email   string `json:"email"`
	HasWork bool   `json:"hasWork"`

	// PortalInviteStatus is her portal access (portalinvite.State, from
	// portalinvite.Derive): nil when the Client was never invited.
	// PortalInviteDelivery is what the latest send of a live invitation
	// did (portal_invite_outbox.status), present only while
	// PortalInviteStatus is "invited" -- the two are separate fields
	// because they are two tables' vocabularies.
	PortalInviteStatus   *portalinvite.State `json:"portalInviteStatus,omitempty"`
	PortalInviteDelivery *string             `json:"portalInviteDelivery,omitempty"`

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

	// sortAt is this Client's place in the list's order, encoded as one time
	// by dueSortJoin. It is the cursor's time.
	sortAt time.Time
}

// OpenEngagement is one rollup line of ListItem.OpenEngagements.
// ContractStatus and DoulaNames are nil when no Contract has been created
// yet, or no Doula holds an open, granted attachment on this Engagement,
// respectively -- the frontend renders DoulaNames' absence as "No Doula
// yet", so an unset Doula never reads as a blank cell.
//
// InvoiceStatus/InvoiceAmountCents and FeeCents are populated in SQL
// regardless of who is asking, then shaped away entirely (never merely
// blanked) by shapeOpenEngagement for a Reader ADR-0008 bars from them --
// a "fetch full, shape by role" split: Go, not SQL, is where the role
// check is easiest to see and to test.
type OpenEngagement struct {
	EngagementID string `json:"engagementId"`
	// Kind is 'birth' or 'postpartum'. A Client can hold one open
	// Engagement of each kind at once (ADR-0017), so the kind is what
	// names a rollup line and its link to the Engagement (#1511).
	Kind             string `json:"kind"`
	EngagementStatus string `json:"engagementStatus"`
	// DueDate is the Engagement's due date as YYYY-MM-DD, absent when none
	// is on file. It is not money, so no role is barred from it.
	DueDate *string `json:"dueDate,omitempty"`
	// BirthOutcome ('live_birth', 'loss' or 'unknown', migration 00093) and
	// PregnancyEndedOn (YYYY-MM-DD) are what the Engagement records of how
	// the pregnancy ended, each absent until recorded. Not money, so no role
	// is barred from them.
	BirthOutcome     *string `json:"birthOutcome,omitempty"`
	PregnancyEndedOn *string `json:"pregnancyEndedOn,omitempty"`
	ContractStatus   *string `json:"contractStatus,omitempty"`
	// DoulaNames is a list so a comma inside one name never splits it.
	DoulaNames []string `json:"doulaNames,omitempty"`

	// InvoiceStatus/InvoiceAmountCents: Owner, Admin, and an employed
	// Doula (ADR-0008 as amended by #282). Never set for a contractor
	// Doula, even when she is attached to this exact Engagement -- she
	// gets only her own fee, never the Practice's Invoice.
	//
	// #741: a Contract can hold several Invoices (a deposit, then a
	// balance), so these two fields report the oldest still-`open`
	// (billed, unpaid) one when the Contract has any -- "is anything
	// outstanding", per docs/journeys/practice-owner.md's RA-G7 -- and
	// only fall back to the newest Invoice of any other status once
	// nothing is outstanding. Before #741 this was always the newest
	// Invoice regardless of status, so a paid balance could hide an
	// unpaid deposit raised earlier.
	InvoiceStatus      *string `json:"invoiceStatus,omitempty"`
	InvoiceAmountCents *int64  `json:"invoiceAmountCents,omitempty"`
	// DaysOverdue (#2130) is how many whole Practice days the Invoice
	// InvoiceStatus names is past its due day, and 0 when it is not late
	// or not open: counted by overdue.Days at the request's one instant,
	// the same rule and the same count every other Staff read of an
	// Invoice carries (#2122, ADR-0038), so one Invoice reads Overdue on
	// this list exactly when it does on its Engagement and on the
	// Invoices list. Set beside InvoiceStatus and gated with it.
	DaysOverdue *int `json:"daysOverdue,omitempty"`
	// PaymentProcessing (#2225) is whether a payment on the Invoice
	// InvoiceStatus names is on its way -- a bank debit, for days -- by
	// the rule every Staff read of an Invoice uses (payments'
	// paymentProcessingExpr). Set beside InvoiceStatus and gated with it.
	PaymentProcessing *bool `json:"paymentProcessing,omitempty"`
	// RefundedCents (#2849) is how much of the Invoice InvoiceStatus names
	// has gone back to the Client, the figure the Invoices list's row
	// carries, so a paid line says it was paid and how much went back.
	// Set beside InvoiceStatus and gated with it.
	RefundedCents *int64 `json:"refundedCents,omitempty"`

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
	// ClientCount (#2867) is every Client the same narrowing matches, for a
	// reader narrowed to her own attachments only hers. Only the first page
	// carries it; a request with a cursor omits it.
	ClientCount *int `json:"clientCount,omitempty"`
}

// ListHandler lists Clients at the current Practice, Client-shaped,
// cursor-paginated. Default filter is "Clients with work" -- a Client
// with at least one Engagement, or a pending Engagement Request -- per
// ADR-0017; ?all=true returns everyone. A row already merged into
// another (ADR-0017's amendment tombstone) never appears, under either
// filter. An ambient contractor Doula's list is
// narrowed to Clients she holds an open, granted attachment to (through
// any of their Engagements) regardless of the filter, the same ADR-0008
// carve-out Reader.CanAccessClient enforces -- an owner-contractor
// (ADR-0017's "solo Practice") holds the owner role's ambient reach
// instead, so her list is never narrowed. ?mine=true narrows any reader's
// list the same way, to the Clients she herself is attached to (#2624): a
// choice about her own view that grants nothing, since every row it can
// return the unnarrowed list already returns. Must be mounted behind
// staffauth.Middleware.
//
// Ordering is one key per Client from her open Engagements, then id: births
// not yet ended by due date ascending (a birth past its due date first, then
// the soonest), then ended pregnancies by the day they ended, most recent
// first, then Clients with neither. dueSortJoin holds the encoding. A cursor's
// tuple has to be the sort key (#446), and pagecursor.Cursor only carries
// (time, id), so the key rides as a time.
func ListHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		query := r.URL.Query()
		// The name is a Client's, and a URL is logged (#1829): it travels in
		// ListByNameHandler's body only, so a GET that carries one is refused
		// rather than ignored, which would answer with the unfiltered list.
		if query.Has("q") {
			apierr.WriteError(w, "Send the name in a POST to /clients/list, not in the address.", http.StatusBadRequest)
			return
		}
		doula, ok := doulaParam(w, query.Get("doula"))
		if !ok {
			return
		}
		serveList(w, r, c, ListRequest{
			All:        queryFlag(r, "all"),
			Mine:       queryFlag(r, "mine"),
			Owes:       queryFlag(r, "owes"),
			NoContract: queryFlag(r, "noContract"),
			Doula:      doula,
			Cursor:     query.Get("cursor"),
		})
	})
}

// ListRequest is the body of the Clients list read sent as a POST (#1829):
// the same filters and cursor ListHandler reads from its query string, plus
// the one it refuses there, a name.
type ListRequest struct {
	All        bool   `json:"all"`
	Mine       bool   `json:"mine"`
	Owes       bool   `json:"owes"`
	NoContract bool   `json:"noContract"`
	Doula      string `json:"doula"`
	Name       string `json:"name"`
	Cursor     string `json:"cursor"`
}

// ListByNameHandler is ListHandler with its filters in a request body, so a
// Client's name never reaches a URL, a request log or a Referer (#1829).
// It runs the same query with the same filters, cursor, order and role
// narrowing, and writes nothing. Must be mounted behind staffauth.Middleware.
func ListByNameHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		var req ListRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		doula, ok := doulaParam(w, req.Doula)
		if !ok {
			return
		}
		req.Doula = doula
		text, ok := nameParam(w, req.Name)
		if !ok {
			return
		}
		req.Name = text
		serveList(w, r, c, req)
	})
}

// serveList answers the Clients list for filters already read and checked
// by either entry point.
func serveList(w http.ResponseWriter, r *http.Request, c staffauth.Call, req ListRequest) {
	if req.Owes && c.Reader.IsAmbientContractor() {
		apierr.WriteError(w, "You contract for this Practice, so you cannot filter Clients by what they owe. Money is the Practice's to see.", http.StatusForbidden)
		return
	}
	filters := listFilters{
		doula: req.Doula,
		name:  req.Name,
		// Narrowing to who owes or who lacks a Contract is a question about
		// Clients with work, so all has nothing to add while either is on.
		withWorkOnly: !req.All || req.Owes || req.NoContract,
		owes:         req.Owes,
		noContract:   req.NoContract,
	}

	var after *pagecursor.Cursor
	if req.Cursor != "" {
		cur, err := pagecursor.Decode(req.Cursor)
		if err != nil {
			apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
			return
		}
		after = &cur
	}

	var list []ListItem
	var err error
	attached := c.Reader.IsAmbientContractor() || req.Mine
	if attached {
		list, err = listAttachedClients(r.Context(), c.Tx, c.PracticeID, c.StaffID, filters, after)
	} else {
		list, err = listClients(r.Context(), c.Tx, c.PracticeID, filters, after)
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	list, next, hasMore := pagecursor.Cut(list, pageSize, func(v ListItem) (time.Time, string) {
		return v.sortAt, v.ClientID
	})

	if err := attachOpenEngagements(r.Context(), c.Tx, c.PracticeID, list, c.Reader, c.StaffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	resp := ListResponse{Items: list, HasMore: hasMore, NextCursor: next}
	if after == nil {
		count, err := countListedClients(r.Context(), c.Tx, c.PracticeID, c.StaffID, attached, filters)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		resp.ClientCount = &count
	}
	apierr.WriteJSON(w, http.StatusOK, resp)
}

// dueSortJoin is shared by both list queries: due.sort_at is the Client's
// place in the list as one UTC-midnight timestamp, the cursor's time, so the
// composite order fits pagecursor.Cursor's (time, id). Each open Engagement
// gets a key and the Client takes her smallest:
//
//  1. Birth not yet ended (a due_date, no pregnancy_ended_on): the due date
//     itself, so a birth past its due date sorts first, then the soonest.
//  2. Pregnancy ended (a pregnancy_ended_on, whatever the due date):
//     5000-01-01 plus the days from that day to the anchor 3000-01-01. A more
//     recent end is fewer days from the anchor, so the newest birth sorts
//     first, and every such key lands after every group 1 key.
//  3. Neither date: no key. A Client with no keyed line gets 9999-12-31 and
//     follows everyone, by id.
//
// min() skips a line with no key, and a Client with lines in both groups takes
// group 1, being the smaller. The aggregate always yields one row, so the join
// never drops a Client.
const dueSortJoin = `
	LEFT JOIN LATERAL (
	    SELECT (COALESCE(min(CASE
	        WHEN de.pregnancy_ended_on IS NOT NULL THEN DATE '5000-01-01' + (DATE '3000-01-01' - de.pregnancy_ended_on)
	        ELSE de.due_date
	    END), DATE '9999-12-31'))::timestamp AT TIME ZONE 'UTC' AS sort_at
	    FROM engagements de WHERE de.client_id = c.id AND de.status <> 'completed'
	) due ON true`

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

// owesExpr is ?owes=true: an open Engagement whose live (non-voided)
// Contract holds an open Invoice with no payment on its way -- the rule the
// list's Owes and Overdue badges follow, where a payment on its way is a
// fact and not a call to act. Its aliases differ from the outer query's,
// which listAttachedClients already spends `e` on.
const owesExpr = `EXISTS (
	SELECT 1 FROM engagements oe
	JOIN contracts oc ON oc.engagement_id = oe.id AND oc.status <> 'voided'
	JOIN invoices oi ON oi.contract_id = oc.id
	WHERE oe.client_id = c.id AND oe.status <> 'completed'
	  AND oi.status = 'open' AND oi.payment_processing_since IS NULL
)`

// noContractExpr is ?noContract=true: an Engagement past Intake and not
// completed that has no Contract row at all. One whose only Contract is
// voided reads "Contract voided" on the list, not "No Contract yet", so it
// does not match.
const noContractExpr = `EXISTS (
	SELECT 1 FROM engagements ne
	WHERE ne.client_id = c.id AND ne.status NOT IN ('intake', 'completed')
	  AND NOT EXISTS (SELECT 1 FROM contracts nc WHERE nc.engagement_id = ne.id)
)`

// maxNameQueryRunes bounds ?q: a name a person looks for is short, and the
// bound keeps the pattern the database matches against short too.
const maxNameQueryRunes = 100

// listFilters is what the Clients list's address narrows it by.
type listFilters struct {
	withWorkOnly bool
	owes         bool
	noContract   bool
	// doula is the Staff id from ?doula, "" for none: Clients with an open
	// Engagement that Staff member holds an open, granted attachment on. It is
	// not money, so every reader may use it; for an ambient contractor it
	// only narrows the list she already has.
	doula string
	// name is ?q, trimmed, "" for none.
	name string
}

// doulaParam checks the doula filter, writing a 400 and returning false when it is not
// a uuid. The length check keeps uuid.Parse's looser spellings (braces,
// urn:uuid:) out, so one id has one address.
func doulaParam(w http.ResponseWriter, raw string) (string, bool) {
	if raw == "" {
		return "", true
	}
	if _, err := uuid.Parse(raw); err != nil || len(raw) != 36 {
		apierr.WriteError(w, "Choose a Doula from the list.", http.StatusBadRequest)
		return "", false
	}
	return strings.ToLower(raw), true
}

// nameParam trims the name filter, writing a 400 and returning false past
// maxNameQueryRunes.
func nameParam(w http.ResponseWriter, raw string) (string, bool) {
	text := strings.TrimSpace(raw)
	if utf8.RuneCountInString(text) > maxNameQueryRunes {
		apierr.WriteError(w, "Use 100 characters or fewer for the name.", http.StatusBadRequest)
		return "", false
	}
	return text, true
}

// nameMatchExpr is the name filter: the pattern, bound as $param, matches
// her name the way textmatch.ClientNameExpr reads it, or "" for none.
func nameMatchExpr(param int) string {
	return fmt.Sprintf(`($%d::text = '' OR `, param) + textmatch.ClientNameExpr("c", param) + `)`
}

// doulaExpr is ?doula, bound as $%[1]d: an open Engagement of hers with an
// open, granted attachment for that Staff member. Aliased apart from both
// list queries' own.
const doulaExpr = `($%[1]d::uuid IS NULL OR EXISTS (
	SELECT 1 FROM engagements de
	JOIN engagement_attachments da ON da.engagement_id = de.id
	WHERE de.client_id = c.id AND de.status <> 'completed'
	  AND da.staff_id = $%[1]d::uuid AND da.state = 'granted'
))`

// narrowing is the ?doula and ?q conditions as SQL, with their two bind
// arguments numbered from first, to be ANDed into either list query.
func (f listFilters) narrowing(first int) (string, []any) {
	var doula any
	if f.doula != "" {
		doula = f.doula
	}
	return fmt.Sprintf(doulaExpr, first) + " AND " + nameMatchExpr(first+1), []any{doula, textmatch.Contains(f.name)}
}

// queryFlag is whether the address carries name=true.
func queryFlag(r *http.Request, name string) bool {
	return r.URL.Query().Get(name) == "true"
}

// listClients is the ambient-reach query -- Owner, Admin, or employee
// Doula -- filtered by practiceID explicitly, on top of the RLS scoping
// staffauth.Middleware already set up on tx.
func listClients(ctx context.Context, tx *sql.Tx, practiceID string, filters listFilters, after *pagecursor.Cursor) ([]ListItem, error) {
	if err := disableJIT(ctx, tx); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return nil, err
	}
	query, args := listClientsQuery(practiceID, filters, after)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: list clients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanListItems(ctx, rows)
}

// disableJIT turns JIT compilation off for the rest of tx. The list sorts every
// Client by a computed due date, so its plan's estimated cost passes
// jit_above_cost (100,000) at about 1,200 Clients and jit_optimize_above_cost
// (500,000) with the ?doula narrowing at 5,000: measured 2026-11-08
// (TestListQueryPlanAtScale, list_perf_test.go), JIT then compiled 463
// functions for 3.2 s of a query that runs in tens of ms without it. The cost
// is inflated by the row-level security checks on every Client, not by the work.
func disableJIT(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `SELECT set_config('jit', 'off', true)`); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("client: disable jit: %w", err)
	}
	return nil
}

// listClientsQuery is listClients' SQL and its bind arguments, apart from
// the read so list_perf_test.go prices the very statement the handler runs.
//
// Measured with EXPLAIN (ANALYZE, BUFFERS) as app_runtime under the Practice's
// RLS (2026-11-08), one page of thirty at 5,000 Clients, each with one active
// Engagement due on one of 300 days and one in fourteen attached to the
// narrowed Doula (TestListQueryPlanAtScale): a Seq Scan on clients, which
// cannot be avoided because the order is a computed per-Client key, with the
// due-date lateral an Index Scan on engagements_client_idx once per Client
// (5,000 loops), then a top-N Sort. Total cost 412,938 for the default view and
// 579,743 with ?doula, both far over jit_above_cost, and the estimate is the
// RLS check on every Client, not the work. With JIT on (before disableJIT) that
// cost 189.8 ms to run and 3,233 ms with ?doula (463 functions compiled). With
// JIT off, execution and planning: default first page 43.4 ms and 10.8 ms;
// a later page 32.2 ms and 2.3 ms; ?doula 16.8 ms and 11.2 ms; ?q 15.0 ms and
// 3.0 ms; ?doula with ?q, ?owes and ?noContract 21.4 ms and 39.2 ms. Every case
// is under the tens of ms an index would be added for, and an index cannot
// shorten the Seq Scan or the 5,000 probes, so no migration was added.
// Re-run 2026-11-08 after the order became three groups (a CASE in the
// lateral's min(), dueSortJoin): the same nodes, the same Index Scan once per
// Client, total cost 412,838 for the default later page, and the default
// first page in the same tens of ms, so the figures above stand.
func listClientsQuery(practiceID string, filters listFilters, after *pagecursor.Cursor) (string, []any) {
	from, args := clientsFromWhere(practiceID, filters)
	//nolint:gosec // from is built from this file's own constants and a numeric placeholder index; every value rides args as a bind parameter
	query := `SELECT c.id, c.given_name, c.preferred_name, c.family_name, COALESCE(c.email, ''), ` + hasWorkExpr + `,
		        ` + portalinvite.HerAccessColumns + `, ` + emailSuppressedExpr + `, ` + pendingRequestKindsExpr + `, due.sort_at
		 ` + from
	if after != nil {
		query += ` AND (due.sort_at, c.id) > ($7, $8) ORDER BY due.sort_at, c.id LIMIT $9`
		args = append(args, after.At, after.ID, pageSize+1)
	} else {
		query += ` ORDER BY due.sort_at, c.id LIMIT $7`
		args = append(args, pageSize+1)
	}
	return query, args
}

// clientsFromWhere is the rows the ambient-reach list holds, from FROM on,
// with its bind arguments $1 to $6. The page and the count (#2867) both read
// it, so the total is of the rows and never of a looser set.
func clientsFromWhere(practiceID string, filters listFilters) (string, []any) {
	narrow, narrowArgs := filters.narrowing(5)
	//nolint:gosec // narrow is built from this file's own constants and a numeric placeholder index; every value rides args as a bind parameter
	from := `FROM clients c` + portalinvite.HerAccessJoin + dueSortJoin + `
		 WHERE c.practice_id = $1 AND c.standing IN ('live', 'erased') AND (NOT $2 OR ` + hasWorkExpr + `)
		   AND (NOT $3 OR ` + owesExpr + `) AND (NOT $4 OR ` + noContractExpr + `)
		   AND ` + narrow
	return from, append([]any{practiceID, filters.withWorkOnly, filters.owes, filters.noContract}, narrowArgs...)
}

// attachedClientsFromWhere is clientsFromWhere narrowed to Clients staffID
// holds an open, granted attachment on, with its bind arguments $1 to $7. It
// joins one row per attachment, so a reader of it takes DISTINCT c.id.
func attachedClientsFromWhere(practiceID, staffID string, filters listFilters) (string, []any) {
	narrow, narrowArgs := filters.narrowing(6)
	//nolint:gosec // narrow is built from this file's own constants and a numeric placeholder index; every value rides args as a bind parameter
	from := `FROM clients c
		     JOIN engagements e ON e.client_id = c.id
		     JOIN engagement_attachments ea ON ea.engagement_id = e.id
		     ` + portalinvite.HerAccessJoin + dueSortJoin + `
		     WHERE c.practice_id = $1 AND c.standing IN ('live', 'erased') AND ea.staff_id = $2
		       AND ea.state = 'granted'
		       AND (NOT $3 OR ` + hasWorkExpr + `)
		       AND (NOT $4 OR ` + owesExpr + `) AND (NOT $5 OR ` + noContractExpr + `)
		       AND ` + narrow
	return from, append([]any{practiceID, staffID, filters.withWorkOnly, filters.owes, filters.noContract}, narrowArgs...)
}

// countListedClients is how many Clients the list holds under filters,
// counted from the very FROM and WHERE the page reads: attached when the
// reader's list is narrowed to her own attachments (an ambient contractor, or
// ?mine=true), so a contractor's total is her own and never the Practice's.
func countListedClients(ctx context.Context, tx *sql.Tx, practiceID, staffID string, attached bool, filters listFilters) (int, error) {
	var query string
	var args []any
	if attached {
		from, a := attachedClientsFromWhere(practiceID, staffID, filters)
		query, args = `SELECT count(DISTINCT c.id) `+from, a //nolint:gosec // from is built from this file's own constants; every value rides args
	} else {
		from, a := clientsFromWhere(practiceID, filters)
		query, args = `SELECT count(*) `+from, a //nolint:gosec // from is built from this file's own constants; every value rides args
	}
	var n int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&n); err != nil { //nolint:gosec // query is built from this file's own constants; every value rides args
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, fmt.Errorf("client: count listed clients: %w", err)
	}
	return n, nil
}

// listAttachedClients is listClients narrowed to Clients staffID holds an
// open, granted (state = 'granted') engagement_attachments row on,
// via any of their Engagements -- ADR-0008's contractor column. DISTINCT
// because a contractor may be attached to more than one Engagement for
// the same Client.
func listAttachedClients(ctx context.Context, tx *sql.Tx, practiceID, staffID string, filters listFilters, after *pagecursor.Cursor) ([]ListItem, error) {
	// DISTINCT is wrapped in a subquery rather than applied at the top
	// level: Postgres requires a plain SELECT DISTINCT's ORDER BY
	// expressions to appear literally in the select list, and sorting by
	// (sort_at, id) needs the outer query's own comparison, not the
	// inner DISTINCT's column list.
	if err := disableJIT(ctx, tx); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return nil, err
	}
	from, args := attachedClientsFromWhere(practiceID, staffID, filters)
	//nolint:gosec // from is built from this file's own constants and a numeric placeholder index; every value rides args as a bind parameter
	query := `SELECT * FROM (
		     SELECT DISTINCT c.id, c.given_name, c.preferred_name, c.family_name, COALESCE(c.email, '') AS email, ` + hasWorkExpr + ` AS has_work,
		            ` + portalinvite.HerAccessColumns + `,
		            ` + emailSuppressedExpr + ` AS email_suppressed,
		            ` + pendingRequestKindsExpr + ` AS pending_request_kinds, due.sort_at
		     ` + from + `
		 ) attached`
	if after != nil {
		query += ` WHERE (sort_at, id) > ($8, $9) ORDER BY sort_at, id LIMIT $10`
		args = append(args, after.At, after.ID, pageSize+1)
	} else {
		query += ` ORDER BY sort_at, id LIMIT $8`
		args = append(args, pageSize+1)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: list attached clients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanListItems(ctx, rows)
}

// rawOpenEngagement is one row of fetchOpenEngagements' unshaped read --
// every field SQL can produce, before shapeOpenEngagement decides which
// of them reader may actually see.
type rawOpenEngagement struct {
	clientID         string
	engagementID     string
	kind             string
	engagementStatus string
	dueDate          sql.NullString
	birthOutcome     sql.NullString
	pregnancyEndedOn sql.NullString
	contractStatus   sql.NullString
	// doulaNames is the names joined by doulaNameSeparator, which no name
	// a person types holds.
	doulaNames         sql.NullString
	invoiceStatus      sql.NullString
	invoiceAmountCents sql.NullInt64
	invoiceDueAt       sql.NullTime
	paymentProcessing  sql.NullBool
	refundedCents      sql.NullInt64
	feeCents           sql.NullInt64
}

// invoiceDays is the zone and the instant one read of the list counts
// every rollup line's lateness against (#2130): practicetimezone.Load
// and clock.Now(ctx), read once per request, as payments' bookDays is.
type invoiceDays struct {
	zone *time.Location
	now  time.Time
}

// attachOpenEngagements fills ListItem.OpenEngagements for every row of
// list, in one extra query keyed on the whole page's Client ids -- not
// one query per Engagement per Client -- so the rollup stays a bounded
// number of round trips against a 14-doula Practice's book, per #264's
// AC. A no-op when list is empty (the "See everyone" first page, or a
// Practice with no Clients yet).
func attachOpenEngagements(ctx context.Context, tx *sql.Tx, practiceID string, list []ListItem, reader staffauth.Reader, staffID string) error {
	if len(list) == 0 {
		return nil
	}
	clientIDs := make([]string, len(list))
	for i, item := range list {
		clientIDs[i] = item.ClientID
	}
	raws, err := fetchOpenEngagements(ctx, tx, clientIDs, staffID, reader.IsAmbientContractor())
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	// The zone is read only for a reader who is shown Invoices at all: a
	// contractor's lines carry no lateness, so her list makes no extra
	// round trip for it.
	days := invoiceDays{now: clock.Now(ctx)}
	if !reader.IsAmbientContractor() {
		zone, err := practicetimezone.Load(ctx, tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure or a stored zone that will not load, not exercised by unit tests
			return fmt.Errorf("client: load practice timezone: %w", err)
		}
		days.zone = zone
	}
	byClient := map[string][]rawOpenEngagement{}
	for _, raw := range raws {
		byClient[raw.clientID] = append(byClient[raw.clientID], raw)
	}
	for i := range list {
		for _, raw := range byClient[list[i].ClientID] {
			list[i].OpenEngagements = append(list[i].OpenEngagements, shapeOpenEngagement(reader, raw, days))
		}
	}
	return nil
}

// doulaNameSeparator is the ASCII unit separator the rollup query joins
// Doula names with (its E'\x1f'), split again in shapeOpenEngagement.
const doulaNameSeparator = "\x1f"

// openEngagementRollupQueryTemplate is fetchOpenEngagements' query, with
// two %-verbs standing in for placeholder indices that shift depending on
// how many clientIDs the caller passes -- staffIdx and attachedOnlyIdx
// below always come after every clientID placeholder, never before.
//
// #2130 added inv.due_at, so each line's lateness is counted in Go by
// overdue.Days. It is a column of the one Invoice row the inv LATERAL
// already reads off invoices_contract_created_idx, so it adds no join, no
// index and no row. Measured with EXPLAIN (ANALYZE, BUFFERS) as
// app_runtime under the Practice's RLS (2026-10-26), for one page of
// thirty Clients at 5,000 Clients, each with an active Engagement, a
// signed Contract and two Invoices (TestOpenEngagementRollupQueryPlanAtScale,
// list_perf_test.go): Sort over a Hash Left Join, the Engagements by a
// Bitmap Index Scan on engagements_client_idx, the Contract and the
// Invoice each by its own index once per line (Memoize on the Invoice);
// total cost 17,603, under a fifth of jit_above_cost, so no JIT section;
// 13.6 ms planning, 3.0 ms execution, 421 shared buffers.
// On the demo Practice (dev:full, Rooted Birth Collective), the first
// page's thirty Clients and their 27 open Engagements: Sort over a Nested
// Loop Left Join with the same index scans, total cost 1,860, no JIT
// section; 10.9 ms planning, 3.0 ms execution, 504 shared buffers.
// e.due_date, added for the list's Due column, is a column of the Engagement
// row the plan already reads, so the plan is unchanged (re-run 2026-11-08:
// same nodes, total cost 17,322, no JIT section).
// #2849 added inv.refunded_cents, a scalar subquery over the Invoice's
// Refunds: one Bitmap Heap Scan on payments per line, by
// payments_refunds_by_target and payments_invoice_id_idx (re-run
// 2026-11-13 in WSL: total cost 18,028, no JIT section; 8.8 ms execution,
// 452 shared buffers).
// The zone the count needs is one primary-key read of practices
// (practicetimezone.Load) per request, made only for a reader shown
// Invoices.
const openEngagementRollupQueryTemplate = `
	SELECT e.client_id, e.id, e.kind, e.status, to_char(e.due_date, 'YYYY-MM-DD'),
	       e.birth_outcome::text, to_char(e.pregnancy_ended_on, 'YYYY-MM-DD'), ct.status,
	       (SELECT string_agg(s.name, E'\x1f' ORDER BY s.last_name, s.first_name)
	        FROM engagement_attachments ea
	        JOIN staff s ON s.id = ea.staff_id
	        WHERE ea.engagement_id = e.id AND ea.state = 'granted'),
	       inv.status, inv.amount_cents, inv.due_at, inv.payment_processing, inv.refunded_cents, att.fee_amount_cents
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
	    -- #741: a Contract can hold several Invoices (a deposit, then a
	    -- balance), and this used to be plain "ORDER BY i.created_at DESC
	    -- LIMIT 1" -- whichever Invoice was raised most recently, so a
	    -- paid balance could hide an unpaid deposit raised earlier. The
	    -- rollup exists to answer "is anything outstanding" (RA-G7), so
	    -- this instead reads the oldest still-open (billed, unpaid)
	    -- Invoice when the Contract has one, and only falls back to the
	    -- newest Invoice of any other status (paid, void, uncollectible,
	    -- or draft) once nothing is outstanding.
	    SELECT i.status, i.amount_cents, i.due_at,
	           ` + invoicestatus.PaymentOnItsWayExpr + ` AS payment_processing,
	           (SELECT ` + paymentstanding.RefundedSumExpr + ` FROM payments pt
	             WHERE pt.invoice_id = i.id AND ` + paymentstanding.IsRefundExpr + `) AS refunded_cents
	    FROM invoices i
	    WHERE i.contract_id = ct.id
	    ORDER BY (i.status <> 'open'), (CASE WHEN i.status = 'open' THEN i.created_at END), i.created_at DESC
	    LIMIT 1
	) inv ON true
	LEFT JOIN engagement_attachments att
	    ON att.engagement_id = e.id AND att.staff_id = $%d
	   AND att.state = 'granted'
	WHERE e.client_id IN (%s) AND e.status <> 'completed'
	  AND (NOT $%d OR att.id IS NOT NULL)
	ORDER BY e.client_id, e.created_at`

// fetchOpenEngagements reads every open (non-`completed`) Engagement
// belonging to any of clientIDs, with its Contract's status, its
// attached Doula(s)' names, its outstanding (or, failing that, latest)
// Invoice's status/amount (#741), and staffID's own fee on it if she is
// attached -- one query regardless of how many Clients or Engagements
// the page holds.
//
// staffID is always the caller's own id, whatever her role: passing it
// unconditionally (rather than only when she is a contractor) is what
// lets an owner-contractor's (ADR-0017's "solo Practice") own fee join
// through the same LEFT JOIN an ambient contractor's does, with no second
// code path to keep in sync. attachedOnly narrows the result set itself
// to Engagements staffID is attached to (true only for an ambient
// contractor -- staffauth.Reader.IsAmbientContractor, the same gate
// CanAccessEngagement uses -- never for bare IsContractor, which an
// owner-contractor also satisfies) -- false lets every open Engagement on
// the Client through regardless of attachment, for an Owner, an Admin, an
// employee Doula, and an owner-contractor alike. An owner-contractor
// therefore reads the whole Practice like any other Owner, while her own
// fee still joins through wherever she personally holds a granted
// attachment. Which of the fetched fields actually reach the caller is
// shapeOpenEngagement's job, not this query's: fetching
// InvoiceStatus/FeeCents unconditionally is its own "fetch full, shape by
// role" split.
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
			&raw.clientID, &raw.engagementID, &raw.kind, &raw.engagementStatus, &raw.dueDate,
			&raw.birthOutcome, &raw.pregnancyEndedOn, &raw.contractStatus,
			&raw.doulaNames, &raw.invoiceStatus, &raw.invoiceAmountCents, &raw.invoiceDueAt, &raw.paymentProcessing, &raw.refundedCents, &raw.feeCents,
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
// into what reader is actually entitled to see, per ADR-0008 as amended
// by #282: Invoice status/money reaches an Owner, an Admin, and an
// employed Doula -- only a plain contractor is refused. Before #282, an
// employed Doula fell through both branches below and got neither an
// Invoice status nor a fee, a silent gap #282's own mapping found rather
// than a rule. A contractor's own fee reaches a contractor only
// (raw.feeCents is already staffID's own row, never another Doula's, by
// construction of fetchOpenEngagements' join). Contract status, Doula
// name, and Engagement status carry no gate -- every role on ADR-0006's
// table may read them.
func shapeOpenEngagement(reader staffauth.Reader, raw rawOpenEngagement, days invoiceDays) OpenEngagement {
	oe := OpenEngagement{
		EngagementID:     raw.engagementID,
		Kind:             raw.kind,
		EngagementStatus: raw.engagementStatus,
	}
	if raw.dueDate.Valid {
		due := raw.dueDate.String
		oe.DueDate = &due
	}
	if raw.birthOutcome.Valid {
		outcome := raw.birthOutcome.String
		oe.BirthOutcome = &outcome
	}
	if raw.pregnancyEndedOn.Valid {
		ended := raw.pregnancyEndedOn.String
		oe.PregnancyEndedOn = &ended
	}
	if raw.contractStatus.Valid {
		status := raw.contractStatus.String
		oe.ContractStatus = &status
	}
	if raw.doulaNames.Valid {
		oe.DoulaNames = strings.Split(raw.doulaNames.String, doulaNameSeparator)
	}
	if !reader.IsAmbientContractor() {
		if raw.invoiceStatus.Valid {
			status := raw.invoiceStatus.String
			oe.InvoiceStatus = &status
			// invoices.due_at is NOT NULL (00102), so a line with an
			// Invoice status always carries its due instant.
			processing := raw.paymentProcessing.Bool
			late := overdue.Days(status, processing, raw.invoiceDueAt.Time, days.now, days.zone)
			oe.DaysOverdue = &late
			oe.PaymentProcessing = &processing
			refunded := raw.refundedCents.Int64
			oe.RefundedCents = &refunded
		}
		if raw.invoiceAmountCents.Valid {
			amount := raw.invoiceAmountCents.Int64
			oe.InvoiceAmountCents = &amount
		}
	}
	// Bare IsContractor on purpose (#1635). This decides whose fee is
	// shown, and it decides no reach: an Owner or an Admin who is a
	// contractor has an agreed fee of her own on an Engagement she holds a
	// granted attachment on, and she reads it (fetchOpenEngagements'
	// comment, #742). IsAmbientContractor here would hide her own fee.
	if reader.IsContractor() && raw.feeCents.Valid { //nolint:forbidigo // her own fee, not reach: an Owner or an Admin who is a contractor reads it too
		fee := raw.feeCents.Int64
		oe.FeeCents = &fee
	}
	return oe
}

// scanListItems reads a page's rows, deriving each Client's portal
// access through portalinvite.Derive -- the same derivation
// FetchPortalInviteState gives a single read -- at one instant for the
// whole page.
func scanListItems(ctx context.Context, rows *sql.Rows) ([]ListItem, error) {
	now := clock.Now(ctx)
	list := []ListItem{}
	for rows.Next() {
		var item ListItem
		var givenName string
		var preferredName, familyName sql.NullString
		var accessState, delivery sql.NullString
		var expiresAt sql.NullTime
		var pendingKinds sql.NullString
		var sortAt time.Time
		if err := rows.Scan(
			&item.ClientID, &givenName, &preferredName, &familyName, &item.Email, &item.HasWork,
			&accessState, &expiresAt, &delivery, &item.EmailSuppressed, &pendingKinds, &sortAt,
		); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan list item: %w", err)
		}
		item.Name = personname.Full(givenName, preferredName.String, familyName.String)
		item.PortalInviteStatus, item.PortalInviteDelivery = portalinvite.Derive(accessState, expiresAt, delivery, now).Fields()
		if pendingKinds.Valid {
			item.PendingRequestKinds = strings.Split(pendingKinds.String, ",")
		}
		item.sortAt = sortAt
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate list items: %w", err)
	}
	return list, nil
}
