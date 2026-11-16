package contracts

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/personname"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/sentperiod"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/textmatch"
)

// listPageSize is the number of Contracts per page.
const listPageSize = 30

const (
	dayLayout            = "2006-01-02"
	grantedAttachmentSQL = `EXISTS (SELECT 1 FROM engagement_attachments ea WHERE ea.engagement_id = e.id AND ea.staff_id = %s AND ea.state = 'granted')`
)

// ContractListItem is one row of the Practice's Contracts list: the
// Contract, the Engagement it hangs off and the Client it is for, and where
// it stands. The amount and the Signed PDF are the Practice's money and its
// rendered document, so a plain contractor Doula's row carries neither.
type ContractListItem struct {
	ContractID     string `json:"contractId"`
	EngagementID   string `json:"engagementId"`
	ClientID       string `json:"clientId"`
	ClientName     string `json:"clientName"`
	EngagementKind string `json:"engagementKind"`
	Status         string `json:"status"`
	// SteppedOn is the Practice-zone day of the Contract's current step:
	// made for a Draft, sent for a Sent one, signed, or voided.
	SteppedOn string `json:"steppedOn"`
	// DaysWaiting is set on a Sent Contract only: whole Practice days since
	// it was sent, 0 for today.
	DaysWaiting *int `json:"daysWaiting,omitempty"`
	// OpenedOn is the Practice-zone day the Client first opened it.
	OpenedOn string `json:"openedOn,omitempty"`
	// RemindedOn is the Practice-zone day a Staff member last reminded the
	// Client of a Sent Contract (#2951), empty until one has. A reminder
	// whose mail dead-lettered never reached her and is not counted.
	RemindedOn string `json:"remindedOn,omitempty"`
	// CanRemind is set on a Sent Contract whose Client the reminder
	// endpoint would email now (#2978). On a Sent Contract that is false,
	// CanRemindOn is the Practice-zone day it will.
	CanRemind   bool   `json:"canRemind,omitempty"`
	CanRemindOn string `json:"canRemindOn,omitempty"`
	// RemindWithheld says why a Sent Contract cannot be reminded (#2981).
	RemindWithheld string `json:"remindWithheld,omitempty"`
	SentByName     string `json:"sentByName,omitempty"`
	MadeByName     string `json:"madeByName,omitempty"`
	VoidedByName   string `json:"voidedByName,omitempty"`
	// VoidedByErasure is true for a Contract voided by the erasure of its
	// Client, which no Staff member voided, so VoidedByName is empty.
	VoidedByErasure  bool     `json:"voidedByErasure,omitempty"`
	DoulaIDs         []string `json:"doulaIds,omitempty"`
	DueDate          string   `json:"dueDate,omitempty"`
	PregnancyEndedOn string   `json:"pregnancyEndedOn,omitempty"`
	BirthOutcome     string   `json:"birthOutcome,omitempty"`
	DoulaNames       []string `json:"doulaNames,omitempty"`
	AmountCents      *int64   `json:"amountCents,omitempty"`
	HasSignedPDF     bool     `json:"hasSignedPdf"`
	VoidRequested    bool     `json:"voidRequested"`

	steppedAt time.Time
	sentOn    string
	sortName  string
}

// ListResponse is the cursor-pagination envelope from
// docs/api-design.md section 4, plus the count of every Contract the same
// narrowing matches, sent on every page.
type ListResponse struct {
	Items      []ContractListItem `json:"items"`
	NextCursor *string            `json:"nextCursor,omitempty"`
	HasMore    bool               `json:"hasMore"`
	Total      int                `json:"total"`
}

// ListRequest is the body of the Contracts list read sent as a
// POST: the view, order, Doula, Mine, period and cursor the GET reads from its
// query string, plus the one thing it refuses there, a search. The period is
// the two days a Contract was sent between, both included, in the Practice's
// zone (#2954).
type ListRequest struct {
	View     string `json:"view"`
	Order    string `json:"order"`
	Doula    string `json:"doula"`
	Mine     bool   `json:"mine"`
	Search   string `json:"search"`
	SentFrom string `json:"sentFrom"`
	SentTo   string `json:"sentTo"`
	Cursor   string `json:"cursor"`
}

// ListContractsHandler lists the Practice's Contracts by view, with the
// ambient-reach rule of ADR-0008: Owner, Admin and an employed Doula read
// every Contract, a plain contractor Doula only those on an Engagement she
// holds a granted attachment on. ?view= is empty (not signed, Draft and
// Sent), draft, sent, signed, voided or all; ?order= is empty (the view's
// own), oldest, newest or client; ?doula= and ?mine=true narrow to granted
// attachments; ?sentFrom= and ?sentTo= together narrow to the Contracts sent
// on those days, which leaves out a Draft, since it was never sent. A GET that carries ?search is refused, since a search can be
// a Client's name and a URL is logged (#1829). Must be mounted behind
// staffauth.Middleware.
func ListContractsHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		query := r.URL.Query()
		if query.Has("search") {
			apierr.WriteError(w, msgSearchInGet, http.StatusBadRequest)
			return
		}
		serveContracts(w, r, c, ListRequest{
			View:     query.Get("view"),
			Order:    query.Get("order"),
			Doula:    query.Get("doula"),
			Mine:     query.Get("mine") == "true",
			SentFrom: query.Get("sentFrom"),
			SentTo:   query.Get("sentTo"),
			Cursor:   query.Get("cursor"),
		})
	})
}

// SearchContractsHandler is ListContractsHandler with its filters, cursor
// and search in a request body, so a Client's name never reaches a URL, a
// request log or a Referer. It writes nothing. Must be mounted behind
// staffauth.Middleware.
func SearchContractsHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		var req ListRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		serveContracts(w, r, c, req)
	})
}

// listQuery is what a request named, read and checked.
type listQuery struct {
	statuses   string
	sort       listSort
	cursorName string
	doula      string
	mine       bool
	search     string
	period     sentperiod.Period
	zone       *time.Location
	ambient    bool
	staffID    string
}

func newListQuery(w http.ResponseWriter, c staffauth.Call, req ListRequest, zone *time.Location) (listQuery, bool) {
	view, ok := resolveView(req.View)
	if !ok {
		apierr.WriteError(w, msgInvalidView, http.StatusBadRequest)
		return listQuery{}, false
	}
	sort, ok := resolveSort(req.Order, view)
	if !ok {
		apierr.WriteError(w, msgInvalidOrder, http.StatusBadRequest)
		return listQuery{}, false
	}
	doula, ok := client.DoulaParam(w, req.Doula)
	if !ok {
		return listQuery{}, false
	}
	search := strings.TrimSpace(req.Search)
	if utf8.RuneCountInString(search) > maxSearchRunes {
		apierr.WriteError(w, msgSearchLength, http.StatusBadRequest)
		return listQuery{}, false
	}
	period, ok := sentperiod.Parse(w, req.SentFrom, req.SentTo)
	if !ok {
		return listQuery{}, false
	}
	return listQuery{
		statuses:   view.statuses,
		sort:       sort,
		cursorName: req.View + ":" + req.Order,
		doula:      doula,
		mine:       req.Mine,
		search:     search,
		period:     period,
		zone:       zone,
		ambient:    c.Reader.IsAmbientContractor(),
		staffID:    c.StaffID,
	}, true
}

// where is the narrowing both the page and the count share, always ANDed:
// the view's statuses, the contractor's reach, Mine, the Doula, the period
// (in the Practice's zone) and the search. Every value is a bound parameter appended to args.
func (q listQuery) where(practiceID string, args *[]any) string {
	bind := func(v any) string {
		*args = append(*args, v)
		return fmt.Sprintf("$%d", len(*args))
	}
	var out strings.Builder
	out.WriteString("e.practice_id = " + bind(practiceID))
	if q.statuses != "" {
		out.WriteString(" AND " + q.statuses)
	}
	if q.ambient || q.mine {
		fmt.Fprintf(&out, " AND "+grantedAttachmentSQL, bind(q.staffID))
	}
	if q.doula != "" {
		fmt.Fprintf(&out, " AND "+grantedAttachmentSQL, bind(q.doula))
	}
	out.WriteString(q.period.Clause("c.sent_at", q.zone, bind))
	if q.search != "" {
		out.WriteString(" AND " + textmatch.ClientNameExpr("cl", len(*args)+1))
		bind(textmatch.Contains(q.search))
	}
	return out.String()
}

const listFrom = `
	FROM contracts c
	JOIN engagements e ON e.id = c.engagement_id
	JOIN clients cl ON cl.id = e.client_id`

const listColumns = `SELECT c.id, e.id, cl.id, cl.given_name, cl.preferred_name, cl.family_name, e.kind::text, c.status::text,
	       c.stepped_at, c.sent_at, c.opened_at,
	       (SELECT max(o.created_at) FROM contract_notice_outbox o WHERE o.contract_id = c.id AND o.kind = 'reminder' AND o.status IN ('pending', 'sent')),
	       (SELECT s.name FROM staff s WHERE s.id = c.sent_by),
	       (SELECT s.name FROM staff s WHERE s.id = c.created_by),
	       (SELECT s.name FROM staff s WHERE s.id = c.voided_by), c.voided_by_erasure,
	       to_char(e.due_date, 'YYYY-MM-DD'), to_char(e.pregnancy_ended_on, 'YYYY-MM-DD'), e.birth_outcome::text,
	       (SELECT ` + client.DoulaAgg + `
	          FROM engagement_attachments ea
	          JOIN staff s ON s.id = ea.staff_id
	         WHERE ea.engagement_id = e.id AND ea.state = 'granted'),
	       c.amount_cents, c.signed_pdf_object_path IS NOT NULL,
	       EXISTS (SELECT 1 FROM contract_void_requests v WHERE v.contract_id = c.id AND v.status = 'open'),
	       ` + nameKeyExpr

// pageQuery builds one page's query and its arguments: at most limit rows
// after the cursor's position, in q's order, or every row when limit is 0.
func (q listQuery) pageQuery(practiceID string, after *listAfter, limit int) (string, []any) {
	args := []any{}
	query := listColumns + listFrom + "\n\tWHERE " + q.where(practiceID, &args)
	if after != nil {
		query += " AND " + q.sort.after(len(args)+1)
		args = append(args, after.keys...)
		args = append(args, after.id)
	}
	query += " ORDER BY " + q.sort.orderBy()
	if limit > 0 {
		args = append(args, limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	return query, args
}

func practiceZoneAndNow(w http.ResponseWriter, r *http.Request, c staffauth.Call) (*time.Location, time.Time, bool) {
	ctx := r.Context()
	zone, err := practicetimezone.Load(ctx, c.Tx, c.PracticeID)
	if err != nil {
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, time.Time{}, false
	}
	return zone, clock.Now(ctx), true
}

func serveContracts(w http.ResponseWriter, r *http.Request, c staffauth.Call, req ListRequest) {
	ctx := r.Context()
	zone, now, ok := practiceZoneAndNow(w, r, c)
	if !ok {
		return
	}
	q, ok := newListQuery(w, c, req, zone)
	if !ok {
		return
	}

	page, ok := pagecursor.FetchKeyedFrom(w, req.Cursor, pagecursor.MsgInvalidCursor, q.cursorName, listPageSize,
		q.sort.decode(q.cursorName),
		func(after *listAfter) ([]ContractListItem, error) {
			return q.read(ctx, c.Tx, c.PracticeID, after, zone, now)
		},
		func(item ContractListItem) ([]string, string) { return q.sort.cursorKeys(item), item.ContractID })
	if !ok {
		return
	}

	total, err := q.count(ctx, c.Tx, c.PracticeID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	apierr.WriteJSON(w, http.StatusOK, ListResponse{Items: page.Items, NextCursor: page.Next, HasMore: page.HasMore, Total: total})
}

// read fetches one page past the cursor, one row more than a page so the
// caller can tell whether another follows.
func (q listQuery) read(ctx context.Context, tx *sql.Tx, practiceID string, after *listAfter, zone *time.Location, now time.Time) ([]ContractListItem, error) {
	return q.readRows(ctx, tx, practiceID, after, listPageSize+1, zone, now)
}

// readRows is the rows past the cursor in q's order: at most limit of them,
// or every one when limit is 0, which the export reads.
func (q listQuery) readRows(ctx context.Context, tx *sql.Tx, practiceID string, after *listAfter, limit int, zone *time.Location, now time.Time) ([]ContractListItem, error) {
	query, args := q.pageQuery(practiceID, after, limit)
	rows, err := tx.QueryContext(ctx, query, args...) //nolint:gosec // G701: the only text spliced in is a column and a direction from listSort's literals; every value is bound
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []ContractListItem{}
	for rows.Next() {
		var item ContractListItem
		var givenName string
		var preferredName, familyName, sentByName, madeByName, voidedByName, dueDate, endedOn, outcome, doulas sql.NullString
		var sentAt, openedAt, remindedAt sql.NullTime
		var amount int64
		if err := rows.Scan(&item.ContractID, &item.EngagementID, &item.ClientID, &givenName, &preferredName, &familyName,
			&item.EngagementKind, &item.Status, &item.steppedAt, &sentAt, &openedAt, &remindedAt, &sentByName, &madeByName, &voidedByName, &item.VoidedByErasure, &dueDate, &endedOn, &outcome, &doulas,
			&amount, &item.HasSignedPDF, &item.VoidRequested, &item.sortName); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("contracts: scan list row: %w", err)
		}
		item.ClientName = personname.Full(givenName, preferredName.String, familyName.String)
		item.SteppedOn = item.steppedAt.In(zone).Format(dayLayout)
		if sentAt.Valid {
			item.sentOn = sentAt.Time.In(zone).Format(dayLayout)
			if item.Status == string(StatusSent) {
				days := practiceDaysBetween(sentAt.Time, now, zone)
				item.DaysWaiting = &days
				if openedAt.Valid {
					item.OpenedOn = openedAt.Time.In(zone).Format(dayLayout)
				}
				if remindedAt.Valid {
					item.RemindedOn = remindedAt.Time.In(zone).Format(dayLayout)
				}
			}
		}
		item.SentByName = sentByName.String
		item.MadeByName = madeByName.String
		item.VoidedByName = voidedByName.String
		item.DueDate = dueDate.String
		item.PregnancyEndedOn = endedOn.String
		item.BirthOutcome = outcome.String
		if doulas.Valid {
			item.DoulaNames, item.DoulaIDs = client.SplitDoulas(doulas.String)
		}
		if q.ambient {
			item.HasSignedPDF = false
		} else {
			item.AmountCents = &amount
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: iterate list rows: %w", err)
	}
	if err := setRemindability(ctx, tx, items, zone); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	return items, nil
}

// setRemindability fills the reminder fields on the Sent rows from
// remindRules, the rule the reminder endpoint enforces, for every Contract
// on the page in one read.
func setRemindability(ctx context.Context, tx *sql.Tx, items []ContractListItem, zone *time.Location) error {
	var targets []remindTarget
	for _, item := range items {
		if item.Status == string(StatusSent) {
			targets = append(targets, remindTarget{ContractID: item.ContractID, ClientID: item.ClientID})
		}
	}
	rules, err := remindRules(ctx, tx, targets, zone)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	for i := range items {
		if items[i].Status == string(StatusSent) {
			rule := rules[items[i].ContractID]
			items[i].CanRemind, items[i].CanRemindOn, items[i].RemindWithheld = rule.Can, rule.On, rule.Why
		}
	}
	return nil
}

// count is the number of Contracts the narrowing matches, with no cursor.
func (q listQuery) count(ctx context.Context, tx *sql.Tx, practiceID string) (int, error) {
	args := []any{}
	query := `SELECT count(*)` + listFrom + "\n\tWHERE " + q.where(practiceID, &args)
	var n int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&n); err != nil { //nolint:gosec // G701: the only text spliced in is the period's column from a literal; every value is bound
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, fmt.Errorf("contracts: count list: %w", err)
	}
	return n, nil
}

// practiceDaysBetween is how many whole calendar days of loc lie between
// from and to: 0 when both fall on one day.
func practiceDaysBetween(from, to time.Time, loc *time.Location) int {
	return int(civilDay(to, loc).Sub(civilDay(from, loc)) / (24 * time.Hour))
}

// civilDay is at's calendar day in loc as UTC midnight of that day, so two of
// them subtract to a whole number of 24-hour days.
func civilDay(at time.Time, loc *time.Location) time.Time {
	year, month, day := at.In(loc).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
