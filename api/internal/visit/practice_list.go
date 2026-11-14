package visit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/personname"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/textmatch"
)

// schedulePageSize is the fixed number of scheduled Visits returned per
// page, matching pageSize's reasoning: a fixed size keeps the query
// parameter surface small, and the date window below is what actually
// bounds the read.
const schedulePageSize = 30

// defaultScheduleWindow is how far ahead the schedule reaches when the
// caller names no range of her own (#263). "Every Visit ever" is not a
// schedule -- it is an archive, and it grows without limit while the
// question "who is covering whom this month" does not. Thirty days is
// the horizon a fourteen-doula agency books against: long enough to hold
// a full month's work, short enough that the index range scan below
// touches a bounded slice of the table.
const defaultScheduleWindow = 30 * 24 * time.Hour

// The refusals this endpoint owns; the screen renders them and the tests
// assert them.
const (
	MsgInvalidFrom   = "from must be an RFC3339 timestamp"
	MsgInvalidTo     = "to must be an RFC3339 timestamp"
	MsgEmptyRange    = "to must be after from"
	MsgInvalidCursor = "invalid cursor"
	MsgInvalidOrder  = "The order must be empty or latest."
	// MsgInvalidOffEngagement refuses an offEngagement narrowing that is not true (#2863).
	MsgInvalidOffEngagement = "The offEngagement narrowing must be true or absent."
	// MsgInvalidType refuses a type that is not one DeriveType returns (#2888).
	MsgInvalidType = "The type must be prenatal, birth or postpartum."
	// MsgSearchInBody refuses a search in the address: it can be a Client's
	// name, and a URL is logged (#1829).
	MsgSearchInBody = "Send the search in a POST to /visits/list, not in the address."
	MsgSearchLength = "Use 100 characters or fewer for the search."
)

// maxScheduleSearchRunes bounds the search, as the Invoices list's is.
const maxScheduleSearchRunes = 100

// The orders the schedule reads in (#2869). The empty name is the view's
// own, soonest first.
const (
	orderSoonest = ""
	orderLatest  = "latest"
)

// ScheduledVisit is one row of the Practice-wide schedule: when the Visit
// happens, whose it is, and who is covering it. The Engagement identity is
// what the row links through to -- every Visit screen is addressed by
// Engagement id.
//
// ScheduledAt is a plain time.Time rather than the pointer Visit carries:
// this list's own predicate is `scheduled_at IS NOT NULL`, so a row that
// reached here always has one. ClientName arrives already resolved -- the
// schedule names her, it does not print her record -- and no other Client
// fact, no Contract state and no Invoice state is carried.
type ScheduledVisit struct {
	VisitID      string    `json:"visitId"`
	EngagementID string    `json:"engagementId"`
	ClientName   string    `json:"clientName"`
	StaffID      string    `json:"staffId"`
	StaffName    string    `json:"staffName"`
	ScheduledAt  time.Time `json:"scheduledAt"`
	// IsOffEngagement: see Visit.IsOffEngagement (#2596).
	IsOffEngagement bool `json:"isOffEngagement"`
	// Type is DeriveType's own output (type.go), derived in the Practice's
	// zone as ListHandler's rows are (ADR-0036): prenatal, birth or
	// postpartum.
	Type string `json:"type"`
}

// PracticeScheduleResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4.
type PracticeScheduleResponse struct {
	Items      []ScheduledVisit `json:"items"`
	NextCursor *string          `json:"nextCursor,omitempty"`
	HasMore    bool             `json:"hasMore"`
	// VisitCount (#2867) is every Visit the same narrowing matches -- the
	// days, the Doula and, for a contractor, her attachments -- not the
	// page's. Only the first page carries it; a request with a cursor
	// omits it.
	VisitCount *int `json:"visitCount,omitempty"`
	// OffEngagementCount (#2863) is how many Visits under the same days, Doula
	// and search wait for a new Doula. Only the first page of a read not already
	// narrowed to them carries it, and only for an Owner or Admin.
	OffEngagementCount *int `json:"offEngagementCount,omitempty"`
}

// scheduleFilter is the narrowed question a caller actually asked: a
// half-open instant range, an optional Doula, and the contractor rule
// that is not hers to choose.
type scheduleFilter struct {
	from time.Time
	// to is exclusive, so a caller asking for one day passes that day's
	// start and the next day's start and gets neither the previous day's
	// last Visit nor the next day's first.
	to time.Time
	// staffID is the empty string when the caller named no Doula.
	staffID string
	// contractorStaffID is set only for an ambient contractor Doula: the
	// staff id her attachment rows are keyed by. Empty for every other
	// caller, which is what turns the EXISTS narrowing off.
	contractorStaffID string
	// order is the name of the order the reader chose; the count ignores it.
	order string
	// namePattern is textmatch.Contains of the Client name a reader typed,
	// "" for none.
	namePattern string
	// offEngagement narrows to the Visits that wait for a new Doula (#2863).
	offEngagement bool
	// visitType narrows to one of the three types DeriveType returns, "" for
	// any (#2888).
	visitType string
	// zone is the Practice's own, which the type is read in; the handler sets
	// it once it has loaded it.
	zone *time.Location
}

// ScheduleListRequest is the body of the Practice-wide schedule read sent
// as a POST (#2870, #1829): the narrowing and cursor PracticeScheduleHandler
// reads from its query string, plus the one thing it refuses there, a
// search. From and To are RFC 3339, "" for the default window.
type ScheduleListRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	StaffID string `json:"staffId"`
	Order   string `json:"order"`
	Search  string `json:"search"`
	Cursor  string `json:"cursor"`
	// OffEngagement narrows to the Visits that wait for a new Doula (#2863).
	OffEngagement bool `json:"offEngagement"`
	// Type narrows to prenatal, birth or postpartum Visits (#2888).
	Type string `json:"type"`
}

// scheduleAfter is the position a keyed cursor decodes to: the scheduled
// instant and the Visit the previous page ended on.
type scheduleAfter struct {
	at time.Time
	id string
}

func decodeScheduleCursor(order string) func(string) (scheduleAfter, error) {
	return func(s string) (scheduleAfter, error) {
		c, err := pagecursor.DecodeKeyed(s, order)
		if err != nil {
			return scheduleAfter{}, fmt.Errorf("visit: schedule cursor: %w", err)
		}
		if len(c.Keys) != 1 {
			return scheduleAfter{}, errors.New("visit: schedule cursor: wrong key count")
		}
		at, err := pagecursor.ParseTimeKey(c.Keys[0])
		if err != nil {
			return scheduleAfter{}, fmt.Errorf("visit: schedule cursor: %w", err)
		}
		return scheduleAfter{at: at, id: c.ID}, nil
	}
}

// PracticeScheduleHandler lists every scheduled Visit at the Practice,
// across all Clients and all Doulas, soonest first (#263). Before it,
// seeing what the Practice was carrying meant opening one Engagement page
// per Client and holding the result in your head. Named for the surface
// rather than the noun because ScheduleHandler is already #250's
// per-Visit write. Must be mounted behind staffauth.Middleware.
//
// Mounted AnyStaff, which is ADR-0006/ADR-0008's Engagements/Visits/
// Messages row verbatim rather than a new rule: an Owner and an Admin
// read every Visit at the Practice, an employee Doula reads every Visit
// at the Practice too (the ambient grant #227 decided), and a contractor
// Doula reads only Visits on Engagements she holds an open, granted
// attachment on. That last narrowing is a row filter, not a refusal --
// she gets 200 and a shorter list, the same shape ListHandler's own
// CanAccessEngagement check produces one Engagement at a time. It is
// enforced here, in the query, and not only hidden in the UI.
//
// Ordered ascending, unlike ListHandler's newest-first feed: a schedule
// is read forwards. The soonest Visit is the one somebody is about to
// have to cover, and it belongs at the top.
//
// A read surface only. Nothing here changes state, so there is nothing
// for the audit trail to record -- the writes that put a scheduled
// instant on a Visit (#250) and assign it a Doula stay where they are, on
// the Engagement page, and record themselves there.
func PracticeScheduleHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		query := r.URL.Query()
		if query.Has("q") || query.Has("search") {
			apierr.WriteError(w, MsgSearchInBody, http.StatusBadRequest)
			return
		}
		if query.Has("offEngagement") && query.Get("offEngagement") != "true" {
			apierr.WriteError(w, MsgInvalidOffEngagement, http.StatusBadRequest)
			return
		}
		filter, ok := parseScheduleFilter(w, r, c, ScheduleListRequest{
			From: query.Get("from"), To: query.Get("to"), StaffID: query.Get("staffId"), Order: query.Get("order"),
			OffEngagement: query.Has("offEngagement"), Type: query.Get("type"),
		})
		if !ok {
			return
		}
		serveSchedule(w, r, c, filter, query.Get("cursor"))
	})
}

// ScheduleListHandler is PracticeScheduleHandler with its narrowing, cursor
// and search in a request body, so a Client's name never reaches a URL, a
// request log or a Referer (#1829, #2870). The same rows, count, order,
// cursor and contractor rule, narrowed to the Visits whose Client's name
// matches the search. It writes nothing. Must be mounted behind
// staffauth.Middleware.
func ScheduleListHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		var req ScheduleListRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		filter, ok := parseScheduleFilter(w, r, c, req)
		if !ok {
			return
		}
		serveSchedule(w, r, c, filter, req.Cursor)
	})
}

// serveSchedule answers the schedule for a filter either entry point has
// already read and checked.
func serveSchedule(w http.ResponseWriter, r *http.Request, c staffauth.Call, filter scheduleFilter, cursor string) {
	ctx := r.Context()
	page, ok := pagecursor.FetchKeyedFrom(w, cursor, MsgInvalidCursor, filter.order, schedulePageSize,
		decodeScheduleCursor(filter.order),
		func(after *scheduleAfter) ([]ScheduledVisit, error) {
			zone, err := practicetimezone.Load(ctx, c.Tx, c.PracticeID)
			if err != nil {
				return nil, err //nolint:wrapcheck // Fetch turns any error into the 500
			}
			filter.zone = zone
			return listScheduledVisits(ctx, c.Tx, c.PracticeID, filter, after)
		},
		func(v ScheduledVisit) ([]string, string) {
			return []string{pagecursor.TimeKey(v.ScheduledAt)}, v.VisitID
		})
	if !ok {
		return
	}

	resp := PracticeScheduleResponse{Items: page.Items, HasMore: page.HasMore, NextCursor: page.Next}
	if cursor == "" {
		count, err := countScheduledVisits(ctx, c.Tx, c.PracticeID, filter)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		resp.VisitCount = &count
		if c.Reader.IsOwnerOrAdmin() && !filter.offEngagement {
			waiting := filter
			waiting.offEngagement = true
			offCount, err := countScheduledVisits(ctx, c.Tx, c.PracticeID, waiting)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			resp.OffEngagementCount = &offCount
		}
	}
	apierr.WriteJSON(w, http.StatusOK, resp)
}

// parseScheduleFilter reads the narrowing inputs off the query string,
// writing the refusal itself and reporting false when one is malformed.
// An absent range is the default window rather than "everything": the
// read stays bounded whether or not the caller thought about it.
//
// The contractor narrowing is decided here too, from the request's own
// Reader rather than from anything the caller sent -- a query parameter
// could be edited, a Reader cannot.
func parseScheduleFilter(w http.ResponseWriter, r *http.Request, c staffauth.Call, req ScheduleListRequest) (scheduleFilter, bool) {
	filter := scheduleFilter{from: clock.Now(r.Context()).UTC()}

	if raw := req.From; raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			apierr.WriteError(w, MsgInvalidFrom, http.StatusBadRequest)
			return scheduleFilter{}, false
		}
		filter.from = parsed
	}
	filter.to = filter.from.Add(defaultScheduleWindow)
	if raw := req.To; raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			apierr.WriteError(w, MsgInvalidTo, http.StatusBadRequest)
			return scheduleFilter{}, false
		}
		filter.to = parsed
	}
	if !filter.to.After(filter.from) {
		apierr.WriteError(w, MsgEmptyRange, http.StatusBadRequest)
		return scheduleFilter{}, false
	}

	filter.order = req.Order
	if filter.order != orderSoonest && filter.order != orderLatest {
		apierr.WriteError(w, MsgInvalidOrder, http.StatusBadRequest)
		return scheduleFilter{}, false
	}

	if raw := req.StaffID; raw != "" {
		if !staffauth.ParseUUID(w, "staff", raw) {
			return scheduleFilter{}, false
		}
		filter.staffID = raw
	}

	search := strings.TrimSpace(req.Search)
	if utf8.RuneCountInString(search) > maxScheduleSearchRunes {
		apierr.WriteError(w, MsgSearchLength, http.StatusBadRequest)
		return scheduleFilter{}, false
	}
	filter.namePattern = textmatch.Contains(search)
	filter.offEngagement = req.OffEngagement
	switch req.Type {
	case "", TypePrenatal, TypeBirth, TypePostpartum:
		filter.visitType = req.Type
	default:
		apierr.WriteError(w, MsgInvalidType, http.StatusBadRequest)
		return scheduleFilter{}, false
	}

	if c.Reader.IsAmbientContractor() {
		filter.contractorStaffID = c.StaffID
	}
	return filter, true
}

// listScheduledVisits reads one page of the Practice's schedule, soonest
// first. The cursor comparison is `>` because this list ascends;
// pagecursor carries a position, not a direction.
//
// Performance, per #263's own criterion, stated here because this is
// where the mechanism lives. `visits` has no practice_id column, so the
// Practice filter is the join to engagements -- applied explicitly on top
// of the RLS scoping staffauth.Middleware already set on tx, the same
// belt-and-braces the contracts awaiting-signature roll-up uses. What
// keeps the read quick at a fourteen-doula agency's size is the half-open
// `scheduled_at` range plus 00091's partial index over (scheduled_at, id)
// WHERE scheduled_at IS NOT NULL: the index's predicate is this query's
// predicate, so the planner range-scans the window instead of reading
// every Visit ever logged, and the index's column order is this query's
// ORDER BY, so a page comes out of that same scan already sorted. The
// window is bounded whether or not the caller passed one
// (parseScheduleFilter's default), and the page is bounded by the cursor
// rather than by an unbounded LIMIT.
//
// The contractor narrowing is the same EXISTS
// staffauth.Reader.CanAccessEngagement runs one Engagement at a time --
// an open, granted attachment, never an accrued one (#228: a record of
// work, never a key) -- pushed into the list query so a contractor's page
// is short rather than filtered after the fact.
// scheduleSelect is the whole predicate except the cursor: the Practice,
// the half-open window, and the two optional narrowings expressed as
// "this parameter is NULL, or it matches". `$4` is the Doula a reader
// picked; `$5` is the contractor rule she did not pick, and is NULL for
// every caller ADR-0008 grants ambient reach.
var scheduleSelect = `SELECT v.id, e.id, cl.given_name, cl.preferred_name, cl.family_name, s.id, s.name, v.scheduled_at,
	       ` + offEngagementBefore + `$7` + offEngagementAfter + `, e.pregnancy_ended_on::text` + scheduleFromWhere

// scheduleFromWhere is the rows the schedule holds, shared by the page and
// by the count (#2867), so the total can never be of a different set than
// the rows. It reads $1 to $10; $6 is the Client name pattern, "" for none
// (#2870), $7 is the instant a Visit is measured against, and $8 narrows to
// the Visits waiting for a new Doula (#2863). $9 is the Visit type to keep,
// "" for any, and $10 is the Practice's IANA zone (#2888).
//
// The CASE is DeriveType written a second time, in SQL, so the type is a
// predicate the rows and the count both read; practice_list_type_test.go
// holds the two to one table of cases.
var scheduleFromWhere = `
	  FROM visits v
	  JOIN engagements e ON e.id = v.engagement_id
	  JOIN clients cl ON cl.id = e.client_id
	  JOIN staff s ON s.id = v.staff_id
	 WHERE e.practice_id = $1
	   AND v.scheduled_at IS NOT NULL
	   AND v.scheduled_at >= $2
	   AND v.scheduled_at < $3
	   AND ($4::uuid IS NULL OR v.staff_id = $4::uuid)
	   AND ($5::uuid IS NULL OR EXISTS (
	         SELECT 1 FROM engagement_attachments ea
	          WHERE ea.engagement_id = v.engagement_id AND ea.staff_id = $5::uuid
	            AND ea.state = 'granted'))
	   AND ($6::text = '' OR ` + textmatch.ClientNameExpr("cl", 6) + `)
	   AND (NOT $8::boolean OR ` + offEngagementBefore + `$7` + offEngagementAfter + `)
	   AND ($9::text = '' OR (CASE
	         WHEN e.pregnancy_ended_on IS NULL THEN '` + TypePrenatal + `'
	         WHEN (v.scheduled_at AT TIME ZONE $10::text)::date < e.pregnancy_ended_on THEN '` + TypePrenatal + `'
	         WHEN (v.scheduled_at AT TIME ZONE $10::text)::date = e.pregnancy_ended_on THEN '` + TypeBirth + `'
	         ELSE '` + TypePostpartum + `' END) = $9::text)`

// nullableID turns "the caller named none" into a real SQL NULL, which is
// what the `$n::uuid IS NULL` halves above test. An empty string would be
// a malformed uuid, not an absent one.
func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

// countScheduledVisits is how many Visits scheduleFromWhere holds for
// filter, whoever asks: a contractor's count is narrowed by her attachments
// exactly as her rows are.
func countScheduledVisits(ctx context.Context, tx *sql.Tx, practiceID string, filter scheduleFilter) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT count(*)`+scheduleFromWhere,
		practiceID, filter.from, filter.to, nullableID(filter.staffID), nullableID(filter.contractorStaffID), filter.namePattern,
		clock.Now(ctx), filter.offEngagement, filter.visitType, filter.zone.String()).Scan(&n)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, fmt.Errorf("visit: count practice schedule: %w", err)
	}
	return n, nil
}

// withSchedulePage closes the query with the cursor comparison, the order
// and the page limit. The four shapes stay string literals: `>` ascends and
// `<` descends, and each seeks along 00091's index in its own direction.
func withSchedulePage(query string, args []any, order string, after *scheduleAfter) (string, []any) {
	latest := order == orderLatest
	switch {
	case after != nil && latest:
		query += ` AND (v.scheduled_at, v.id) < ($11, $12) ORDER BY v.scheduled_at DESC, v.id DESC LIMIT $13`
	case after != nil:
		query += ` AND (v.scheduled_at, v.id) > ($11, $12) ORDER BY v.scheduled_at, v.id LIMIT $13`
	case latest:
		query += ` ORDER BY v.scheduled_at DESC, v.id DESC LIMIT $11`
	default:
		query += ` ORDER BY v.scheduled_at, v.id LIMIT $11`
	}
	if after != nil {
		args = append(args, after.at, after.id)
	}
	return query, append(args, schedulePageSize+1)
}

func listScheduledVisits(
	ctx context.Context,
	tx *sql.Tx,
	practiceID string,
	filter scheduleFilter,
	after *scheduleAfter,
) ([]ScheduledVisit, error) {
	// Every clause below is a string literal, and the two optional
	// narrowings ride on NULL-able parameters rather than on placeholders
	// counted at run time: a query assembled with fmt.Sprintf is what
	// gosec's G202/G701 refuse, and rightly, since the next person to add
	// a filter is one interpolation away from a real injection. The
	// cursor is the one branch left, because `(scheduled_at, id) > (…)`
	// has to stay a plain comparison to seek along 00091's index instead
	// of being buried in an OR the planner cannot use.
	query := scheduleSelect
	args := []any{
		practiceID, filter.from, filter.to,
		nullableID(filter.staffID), nullableID(filter.contractorStaffID), filter.namePattern, clock.Now(ctx), filter.offEngagement,
		filter.visitType, filter.zone.String(),
	}
	query, args = withSchedulePage(query, args, filter.order, after)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: list practice schedule: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []ScheduledVisit{}
	for rows.Next() {
		var item ScheduledVisit
		var givenName string
		var preferredName, familyName sql.NullString
		var endedOn *string
		if err := rows.Scan(&item.VisitID, &item.EngagementID, &givenName, &preferredName, &familyName,
			&item.StaffID, &item.StaffName, &item.ScheduledAt, &item.IsOffEngagement, &endedOn); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("visit: scan practice schedule row: %w", err)
		}
		item.ClientName = personname.Full(givenName, preferredName.String, familyName.String)
		item.Type = DeriveType(item.ScheduledAt, endedOn, filter.zone)
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: iterate practice schedule rows: %w", err)
	}
	return list, nil
}
