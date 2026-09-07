package visit

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
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
}

// PracticeScheduleResponse is the standard cursor-pagination envelope from
// docs/api-design.md section 4.
type PracticeScheduleResponse struct {
	Items      []ScheduledVisit `json:"items"`
	NextCursor *string          `json:"nextCursor,omitempty"`
	HasMore    bool             `json:"hasMore"`
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		filter, ok := parseScheduleFilter(w, r)
		if !ok {
			return
		}

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, MsgInvalidCursor, http.StatusBadRequest)
				return
			}
			after = &c
		}

		list, err := listScheduledVisits(r.Context(), tx, practiceID, filter, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		hasMore := len(list) > schedulePageSize
		if hasMore {
			list = list[:schedulePageSize]
		}
		resp := PracticeScheduleResponse{Items: list, HasMore: hasMore}
		if hasMore {
			last := list[len(list)-1]
			next := pagecursor.Encode(last.ScheduledAt, last.VisitID)
			resp.NextCursor = &next
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// parseScheduleFilter reads the narrowing inputs off the query string,
// writing the refusal itself and reporting false when one is malformed.
// An absent range is the default window rather than "everything": the
// read stays bounded whether or not the caller thought about it.
//
// The contractor narrowing is decided here too, from the request's own
// Reader rather than from anything the caller sent -- a query parameter
// could be edited, a Reader cannot.
func parseScheduleFilter(w http.ResponseWriter, r *http.Request) (scheduleFilter, bool) {
	query := r.URL.Query()
	filter := scheduleFilter{from: time.Now().UTC()}

	if raw := query.Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			apierr.WriteError(w, MsgInvalidFrom, http.StatusBadRequest)
			return scheduleFilter{}, false
		}
		filter.from = parsed
	}
	filter.to = filter.from.Add(defaultScheduleWindow)
	if raw := query.Get("to"); raw != "" {
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

	if raw := query.Get("staffId"); raw != "" {
		if !staffauth.ParseUUID(w, "staff", raw) {
			return scheduleFilter{}, false
		}
		filter.staffID = raw
	}

	reader, has := staffauth.ReaderFrom(r.Context())
	staffID, hasStaffID := staffauth.StaffID(r.Context())
	if !has || !hasStaffID {
		// coverage:ignore reason: staffauth.Middleware always places a Reader and a staff id on context before this handler runs
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return scheduleFilter{}, false
	}
	if reader.IsAmbientContractor() {
		filter.contractorStaffID = staffID
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
const scheduleSelect = `SELECT v.id, e.id, cl.given_name, cl.preferred_name, s.id, s.name, v.scheduled_at
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
	            AND ea.origin = 'granted' AND ea.ended_at IS NULL))`

// nullableID turns "the caller named none" into a real SQL NULL, which is
// what the `$n::uuid IS NULL` halves above test. An empty string would be
// a malformed uuid, not an absent one.
func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func listScheduledVisits(
	ctx context.Context,
	tx *sql.Tx,
	practiceID string,
	filter scheduleFilter,
	after *pagecursor.Cursor,
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
		nullableID(filter.staffID), nullableID(filter.contractorStaffID),
	}
	if after != nil {
		query += ` AND (v.scheduled_at, v.id) > ($6, $7) ORDER BY v.scheduled_at, v.id LIMIT $8`
		args = append(args, after.At, after.ID, schedulePageSize+1)
	} else {
		query += ` ORDER BY v.scheduled_at, v.id LIMIT $6`
		args = append(args, schedulePageSize+1)
	}

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
		var preferredName sql.NullString
		if err := rows.Scan(&item.VisitID, &item.EngagementID, &givenName, &preferredName,
			&item.StaffID, &item.StaffName, &item.ScheduledAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("visit: scan practice schedule row: %w", err)
		}
		item.ClientName = client.PreferredName(givenName, preferredName.String)
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: iterate practice schedule rows: %w", err)
	}
	return list, nil
}
