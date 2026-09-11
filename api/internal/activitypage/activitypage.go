// Package activitypage is the one reader of one subject's rows in
// ADR-0022's activity table.
//
// Four readers used to answer "what happened to this thing?" and three
// of them hand-wrote it -- an Engagement's ledger, a Client's history, a
// Membership's roster history -- because the fourth, the generic one in
// activityfeed, could not carry a diff and all three of them are
// diff-aware. #1150 found what the three copies cost: one departed Staff
// actor had three different names and a blank across four screens of one
// Practice, one of the three filtered the Practice in SQL alone where
// the others also filtered it in the app layer, and one was not
// paginated or ordered like the other two at all. Every one of those is
// a disagreement about a fact rather than about a caller, which is
// exactly what a second copy of a query is for producing.
//
// So this package owns the whole of the shape: Practice and subject
// scoping, the optional action exclusion, the newest-first
// (created_at, id) cursor docs/api-design.md section 4 asks for, the
// one-row-past-the-page sentinel that answers "is there more?" without a
// second query, the page trim, the cursor mint, and the actor's resolved
// name. A caller supplies only what is genuinely its own -- the extra
// columns it selects, the joins those columns need, and the DTO it
// builds -- through a Projection.
//
// A diff is one of those extra columns and never a field of Row. That is
// the whole of how one reader carries a diff to Staff and withholds it
// from a Client (#1150's AC2): a caller that wants a diff names a.diff
// in its own Projection, and the Client-facing reader's SQL has no diff
// in it at all rather than a flag saying not to send one.
//
// It applies no access gate. A caller must already know its reader may
// see this subject before calling -- activitygate.CanAccessSubject on
// the Staff side, a Client-portal ownership check on the other -- the
// same division of labor engagement.ListActivityHandler always drew
// between its own 404 gate and its SQL filter. Beneath that, every read
// runs on the caller's own transaction, so activity_practice_visibility
// (00051) is still in force; the practice_id predicate this package
// always writes is the second, app-layer half of that pair, so a bug in
// either alone cannot reach another Practice's rows.
//
// It lives below activityfeed rather than in it because activityfeed
// imports staffauth and client, and both of those are callers here.
package activitypage

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/personname"
)

// Query names one page of one subject's activity.
//
// ExcludedActions is an optional SQL fragment of quoted action literals
// (e.g. "'offer_sent', 'offer_accepted'"), built once by the caller from
// its own internal action constants -- never request input, the same
// shape engagement's own money exclusion and portal's staffing exclusion
// already are -- or the empty string to exclude nothing.
//
// PageSize of zero means every row: no LIMIT, no sentinel, no cursor
// minted, HasMore false. Only the Client's own history reads that way,
// because it always has (it renders one screen of a woman's whole
// record); it is not a default anything else should reach for, and a
// reader that grows a cursor should stop passing zero rather than page
// around an unbounded read.
type Query struct {
	PracticeID      string
	SubjectKind     string
	SubjectID       string
	ExcludedActions string
	After           *pagecursor.Cursor
	PageSize        int
}

// Row is the facts every subject-scoped reader resolves identically,
// handed to a Projection's builder once the row has been scanned.
//
// ID is the activity row's own id. It is here and not in
// activityfeed.Entry because the two answer different questions: Entry
// is a DTO #486 deliberately kept ids out of, and ID is what a caller
// whose own DTO names the event (the Membership history's eventId) needs
// -- and what this package itself mints the cursor from.
//
// ActorName is always populated, never a bare id a reader has to resolve
// itself: the acting Staff member's name, activity.DepartedStaffName
// once her Membership has ended and staff_practice_visibility (00002)
// stops admitting her staff row, the acting Client's preferred name, or
// activity.SystemActorName ("Doula Cloud", never "System" -- ADR-0022).
type Row struct {
	ID          string
	SubjectKind string
	SubjectID   string
	Action      string
	ActorKind   string
	ActorName   string
	CreatedAt   time.Time
}

// Projection is a caller's own half of a row: what it selects beyond the
// shared columns, the joins those columns need, and how it builds its
// own DTO from the two halves.
//
// Row returns the scan targets for Columns, in the same order, and a
// builder closing over them. It is called once per row.
//
// A builder must not touch the transaction. It runs while the result set
// is still open, and a second query on the same connection there
// deadlocks -- which is why the Client's per-key diff unsealing happens
// after this package returns, not inside its own builder.
//
// Joins are appended after the two actor joins this package always
// writes, and may refer to a (the activity row), s (the actor's staff
// row) and c (the actor's clients row). Columns are appended after the
// shared SELECT list.
//
// Neither is request input: both are built from the caller's own
// compile-time constants, so the query text this package assembles from
// them carries no injection risk. That is the same reasoning the
// exclusion fragments already carried, stated once here instead of at
// each caller.
type Projection[T any] struct {
	Joins   []string
	Columns []string
	Row     func() (targets []any, build func(Row) T)
}

// Page is docs/api-design.md section 4's envelope over whatever row type
// a Projection builds.
type Page[T any] struct {
	Items      []T
	NextCursor *string
	HasMore    bool
}

// sharedColumns is what every subject-scoped reader selects, in scan
// order: the row's identity and timing, and the three name columns the
// two actor joins reach.
const sharedColumns = `a.id, a.subject_kind, a.subject_id, a.action, a.actor_kind::text,
	       s.name, c.given_name, c.preferred_name, a.created_at`

// sharedJoins reach the actor's name whichever of ADR-0022's two named
// actor kinds wrote the row. Both are LEFT JOINs: a system-written row
// matches neither, and a Staff actor who has since left matches the
// first one no longer (staff_practice_visibility, 00002), which is the
// case Row.ActorName names rather than blanks.
const sharedJoins = `LEFT JOIN staff s ON s.id = a.actor_staff_id
	LEFT JOIN clients c ON c.id = a.actor_client_id`

// Statement returns the SQL and arguments List issues for q and p.
//
// It is exported for one reason, named so the next reader does not have
// to guess: a plan-cost guard has to EXPLAIN the statement that actually
// ships, not a hand-copied literal beside it, or the guard drifts from
// the query the moment either changes. engagement's own JIT canary
// (#1077) is the caller.
func Statement[T any](q Query, p Projection[T]) (string, []any) {
	args := []any{q.PracticeID, q.SubjectKind, q.SubjectID}
	placeholder := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(sharedColumns)
	for _, col := range p.Columns {
		sb.WriteString(",\n	       ")
		sb.WriteString(col)
	}
	sb.WriteString("\n	FROM activity a\n	")
	sb.WriteString(sharedJoins)
	for _, join := range p.Joins {
		sb.WriteString("\n	")
		sb.WriteString(join)
	}
	sb.WriteString("\n	WHERE a.practice_id = $1 AND a.subject_kind = $2 AND a.subject_id = $3")
	if q.ExcludedActions != "" {
		sb.WriteString("\n	  AND a.action NOT IN (")
		sb.WriteString(q.ExcludedActions)
		sb.WriteString(")")
	}
	if q.After != nil {
		sb.WriteString("\n	  AND (a.created_at, a.id) < (")
		sb.WriteString(placeholder(q.After.At))
		sb.WriteString(", ")
		sb.WriteString(placeholder(q.After.ID))
		sb.WriteString(")")
	}
	sb.WriteString("\n	ORDER BY a.created_at DESC, a.id DESC")
	if q.PageSize > 0 {
		sb.WriteString("\n	LIMIT ")
		sb.WriteString(placeholder(q.PageSize + 1))
	}
	return sb.String(), args
}

// List reads one page of q's subject, newest first, and reports whether
// another follows -- asking for one row more than the page holds and
// dropping it, so "is there more?" costs no second query.
func List[T any](ctx context.Context, tx *sql.Tx, q Query, p Projection[T]) (Page[T], error) {
	query, args := Statement(q, p)
	rows, err := tx.QueryContext(ctx, query, args...) //nolint:gosec // every interpolated fragment is a caller's own compile-time constant, never request input -- see Projection's own doc comment
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return Page[T]{}, fmt.Errorf("activitypage: query %s activity: %w", q.SubjectKind, err)
	}
	defer func() { _ = rows.Close() }()

	// keys shadows items one for one. A builder has already turned each
	// row into the caller's own DTO by the time the page is trimmed, and
	// that DTO need not carry the row's id or timestamp at all -- the
	// Engagement ledger's does not -- so the cursor is minted from what
	// was read rather than from what was built.
	items := []T{}
	keys := []rowKey{}
	for rows.Next() {
		var row Row
		var staffName, clientGivenName, clientPreferredName sql.NullString
		targets, build := p.Row()
		dest := append([]any{
			&row.ID, &row.SubjectKind, &row.SubjectID, &row.Action, &row.ActorKind,
			&staffName, &clientGivenName, &clientPreferredName, &row.CreatedAt,
		}, targets...)
		if err := rows.Scan(dest...); err != nil {
			// coverage:ignore reason: row scan failure on a well-typed query, not exercised by unit tests
			return Page[T]{}, fmt.Errorf("activitypage: scan %s activity row: %w", q.SubjectKind, err)
		}
		row.ActorName = ActorName(row.ActorKind, staffName, clientGivenName, clientPreferredName)
		keys = append(keys, rowKey{id: row.ID, at: row.CreatedAt})
		items = append(items, build(row))
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return Page[T]{}, fmt.Errorf("activitypage: iterate %s activity rows: %w", q.SubjectKind, err)
	}

	page := Page[T]{Items: items}
	if q.PageSize <= 0 {
		return page, nil
	}
	if len(items) > q.PageSize {
		page.Items = items[:q.PageSize]
		keys = keys[:q.PageSize]
		page.HasMore = true
		last := keys[len(keys)-1]
		next := pagecursor.Encode(last.at, last.id)
		page.NextCursor = &next
	}
	return page, nil
}

// rowKey is the (created_at, id) pair one row is resumable from.
type rowKey struct {
	id string
	at time.Time
}

// ActorName says who did it, once, for every subject-scoped reader.
//
// The three branches are ADR-0022's three actor kinds, and the fourth
// case -- a Staff actor whose staff row the join could not reach -- is
// the one #1150 found spelled four ways. staff_practice_visibility
// (00002) reaches a staff row only through a live practice_memberships
// row, so the moment a person leaves, every row she wrote loses its
// name. It reads activity.DepartedStaffName, the word #887 settled on,
// so a Practice never meets two different words, or an empty cell, for
// one absence.
//
// A subject kind with no Client writer (a Membership: nothing a Client
// does touches one) simply never reaches the client branch. That costs
// it a clients LEFT JOIN that never matches, which the Membership
// history's own reader used to avoid by hand; #1150 traded that join for
// one spelling of the actor rule, because the hand-written avoidance is
// what let the departed-actor word drift in the first place.
func ActorName(actorKind string, staffName, clientGivenName, clientPreferredName sql.NullString) string {
	switch activity.ActorKind(actorKind) {
	case activity.ActorStaff:
		if !staffName.Valid {
			return activity.DepartedStaffName
		}
		return staffName.String
	case activity.ActorClient:
		return personname.Preferred(clientGivenName.String, clientPreferredName.String)
	default:
		return activity.SystemActorName
	}
}
