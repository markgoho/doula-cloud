package activityfeed

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activitygate"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/staffauth"
)

// practicePageSize is the fixed number of rows one page of the feed
// carries, matching engagement.activityPageSize's own reasoning.
const practicePageSize = 30

// practiceBatchSize is how many raw rows one query fetches before
// #485's gate is applied per row. It is a multiple of practicePageSize,
// not practicePageSize+1 the way a single-subject reader's own "+1
// sentinel" is: a row can be filtered out here (an unregistered subject
// kind, ADR-0008's money tier, a contractor with no attachment), so a
// batch this feed reads may yield fewer than practicePageSize allowed
// rows even though the table holds more. 4x is a deliberate margin
// against that, not a correctness requirement -- see fetchPage's own
// comment for why a short page is still a correct page, and
// practice_test.go's own page-boundary case for the proof.
const practiceBatchSize = practicePageSize * 4

// PracticeHandler lists a Practice's activity across every subject kind
// #485's registry knows, newest first, cursor-paginated -- #486's AC1.
// Every row is gated individually through activitygate.CanAccessSubject
// and activitygate.CanSeeAction (AC2): an unregistered subject kind is
// refused by the gate itself (activitygate's own doc comment), and
// ADR-0008's money tier is applied the same per-row way the gate already
// states it for a cross-subject reader (activitygate.CanSeeAction's own
// doc comment) rather than engagement.ListActivityHandler's single-subject
// SQL exclusion. Must be mounted behind staffauth.Middleware.
func PracticeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		resp, err := fetchPage(r.Context(), tx, reader, practiceID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// personSubjectKind is the one subject kind whose subject_id names a
// person the feed can put a name to (#1148): a membership row's is a
// staff_id. Named once because both halves of the answer need it -- the
// query's own join guard, and resolveSubjectName's decision about which
// rows get a name -- and two spellings of "which kind is a person" is
// exactly the drift a second person-kind would land in. A second one
// turns this into a set and both readers follow it; activitygate's
// registry is the third place that would change, and its own guard test
// is what makes sure that one is not forgotten.
const personSubjectKind = activity.SubjectMembership

// listPracticeActivityQueryTemplate and its "after" counterpart carry no
// subject_kind clause at all -- the one thing that makes this a
// practice-wide feed rather than engagement.listEngagementActivity's own
// single-subject query. %[1]d is practiceBatchSize+1 (the "+1 sentinel"
// pattern engagement.go already uses, generalized to a batch): a compile-
// time constant this package itself wrote, never request input, so
// gosec's G201 does not apply.
//
// AC8's own note, no new index: activity_subject
// (00058_activity_subject_id_index.sql) is a (practice_id, subject_kind,
// subject_id, created_at DESC, id DESC) prefix, built for a single
// subject's own range scan -- it cannot serve this query's bare
// `WHERE practice_id = $1 ORDER BY created_at DESC, id DESC`, which asks
// for every subject_kind interleaved by time. Adding an index the read
// side alone would use is a schema change, and #486 rules that out of
// scope alongside every other Activity-table change. Measured instead
// (perf_test.go's TestPracticeQueryPlanAtScale, 2026-09-04, seeded via
// this repo's own Podman Postgres): at 5,000 activity rows for one
// Practice -- generous headroom over a pilot-scale 14-doula agency's
// first couple of years, going by the design brief's own "eleven events
// on a page" estimate for a single Engagement -- EXPLAIN (ANALYZE,
// BUFFERS) shows `Limit -> Sort (top-N heapsort) -> Bitmap Heap Scan on
// activity via activity_subject's own practice_id prefix`, executing in
// 15-25ms across repeated runs (machine variance, not a regression -- the
// plan shape is what stays fixed). The Sort node is unavoidable without
// the index this ticket isn't adding: every one of the practice_id's own
// matching rows has to be joined and ordered before LIMIT can trim it,
// since no index leaves the rows already in (created_at, id) order across
// every subject_kind. Most of that time is the actor-name JOIN to staff
// re-evaluating its own RLS policy per row (a pre-existing cost
// engagement.listEngagementActivity and client.detail.go's own staff
// joins already pay, not something new here) rather than the activity
// scan itself, which alone runs in under a millisecond. The perf test now
// EXPLAINs listPracticeActivityQuery itself (moved into this package,
// perf_test.go's own doc comment explains why) rather than a hand-copied
// literal, so this comment's numbers can't silently drift from the query
// that actually ships.
//
// Re-measured after #1148 added the subject join below, on the same
// fixture (2026-09-10): Limit -> Sort (top-N heapsort, 41kB) over three
// nested-loop left joins, with the activity scan itself an Index Scan on
// activity_subject's practice_id prefix in 0.5 ms; 8.3 ms planning, 13.1
// ms execution, and no JIT section at all -- which is the number that
// matters, given #1077 measured what a staff-table plan crossing
// jit_above_cost costs (00111's own doc comment). The scan node is an
// Index Scan here where the paragraph above recorded a Bitmap Heap Scan;
// both are the same activity_subject prefix and the planner's choice
// between them moves with the fixture, so the fixed shape is
// Limit -> Sort -> a practice_id-prefixed scan of activity, not the
// particular scan node.
//
// Caveat this re-measurement shares with the one above: every seeded row
// is subject_kind = 'engagement', so the subject join's equality guard
// excludes all 5,000 of them and it is never driven. That is the shape a
// real Practice's feed mostly has -- Membership events are rare beside
// Engagement ones -- but it does mean this number does not price the
// join's own work on a feed dominated by roster rows.
//
// Caveat this measurement doesn't cover: all 5,000 rows sit on one
// subject_id, which activity_subject's own index serves more cheaply than
// a real multi-subject Practice's mixed rows would. If a Practice's
// activity volume ever grows past what a top-N heapsort over a Bitmap
// Heap Scan keeps fast, that is new evidence for a follow-up ticket to
// add a (practice_id, created_at DESC, id DESC) index -- not a case this
// one needs to solve against a fixture.
//
// The subject join (subj) is #1148's own addition: a feed spanning many
// subject kinds has to say who a roster change happened to, or "Roles
// changed" names the actor and nobody else, and a reader cannot tell one
// roster change from another. It is a third LEFT JOIN on a table this
// query already joins once, guarded by an equality on a.subject_kind so
// it does no work at all for the Engagement and Client rows that make up
// most of the feed, and it resolves nothing a row does not already carry
// -- subject_id is a uuid column (00051), and a membership row's is a
// staff_id.
//
// It is a join and not a read of a.diff on purpose: the shared reader
// carries no diff (#1150 is the ticket for that), and a name copied into
// a diff at write time would be a second place a Practice's roster
// spells a person, free to drift from the staff row.
//
// %[2]s is personSubjectKind, interpolated for the same reason
// %[1]d is: a package-internal constant, never request input, and naming
// the constant rather than repeating the literal is what keeps the write
// side and this join from drifting apart.
const listPracticeActivityQueryTemplate = `SELECT a.id, a.subject_kind, a.subject_id, a.action, a.actor_kind::text,
	       s.name, c.given_name, c.preferred_name, subj.name, a.created_at
	FROM activity a
	LEFT JOIN staff s ON s.id = a.actor_staff_id
	LEFT JOIN clients c ON c.id = a.actor_client_id
	LEFT JOIN staff subj ON a.subject_kind = '%[2]s' AND subj.id = a.subject_id
	WHERE a.practice_id = $1
	ORDER BY a.created_at DESC, a.id DESC LIMIT %[1]d`

const listPracticeActivityAfterQueryTemplate = `SELECT a.id, a.subject_kind, a.subject_id, a.action, a.actor_kind::text,
	       s.name, c.given_name, c.preferred_name, subj.name, a.created_at
	FROM activity a
	LEFT JOIN staff s ON s.id = a.actor_staff_id
	LEFT JOIN clients c ON c.id = a.actor_client_id
	LEFT JOIN staff subj ON a.subject_kind = '%[2]s' AND subj.id = a.subject_id
	WHERE a.practice_id = $1
	  AND (a.created_at, a.id) < ($2, $3)
	ORDER BY a.created_at DESC, a.id DESC LIMIT %[1]d`

var listPracticeActivityQuery = fmt.Sprintf(listPracticeActivityQueryTemplate, practiceBatchSize+1, personSubjectKind) //nolint:gosec // both interpolated values are package-internal constants, not request input

var listPracticeActivityAfterQuery = fmt.Sprintf(listPracticeActivityAfterQueryTemplate, practiceBatchSize+1, personSubjectKind) //nolint:gosec // both interpolated values are package-internal constants, not request input

// rawEntry is Entry plus the row id fetchPage needs to mint a cursor but
// never puts in the response, the same shape engagement.activityRow
// already uses.
type rawEntry struct {
	Entry
	cursorID string
}

// fetchPage reads one practiceBatchSize(+1)-row batch, gates each row in
// order through activitygate, and stops the moment practicePageSize
// allowed rows are collected.
//
// The cursor and hasMore are both read off the last RAW row examined,
// never off the last ALLOWED one: a page boundary that lands mid-batch on
// a row the gate refuses must still be resumable from exactly that point,
// or the next page silently skips whatever came after it. gateCache
// memoizes CanAccessSubject per (subjectKind, subjectID) seen in this
// batch -- activitygate.CanAccessSubject is a pure in-memory check for an
// Owner, an Admin or an employee (staffauth.Reader.CanAccessEngagement/
// CanAccessClient's own fast path), so this only matters for a
// contractor, where it turns practiceBatchSize+1 possible round trips into
// one per distinct subject actually seen.
func fetchPage(ctx context.Context, tx *sql.Tx, reader staffauth.Reader, practiceID string, after *pagecursor.Cursor) (ListResponse, error) {
	rows, err := queryBatch(ctx, tx, practiceID, after)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return ListResponse{}, err
	}

	// The sentinel row (see the query templates' own comment) says more
	// exist beyond this batch regardless of how many of the batch's own
	// rows the gate allows -- drop it from what gets examined/gated.
	moreBeyondBatch := len(rows) > practiceBatchSize
	if moreBeyondBatch {
		rows = rows[:practiceBatchSize]
	}

	gateCache := map[string]bool{}
	items := []Entry{}
	examinedIndex := -1
	for i, row := range rows {
		examinedIndex = i
		key := row.SubjectKind + "|" + row.SubjectID
		can, cached := gateCache[key]
		if !cached {
			can, err = activitygate.CanAccessSubject(ctx, tx, reader, row.SubjectKind, row.SubjectID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				return ListResponse{}, fmt.Errorf("activityfeed: check subject access: %w", err)
			}
			gateCache[key] = can
		}
		if can && activitygate.CanSeeAction(reader, row.SubjectKind, row.Action) {
			items = append(items, row.Entry)
			if len(items) == practicePageSize {
				break
			}
		}
	}

	resp := ListResponse{Items: items}
	if examinedIndex < 0 {
		return resp, nil
	}
	resp.HasMore = examinedIndex < len(rows)-1 || moreBeyondBatch
	if resp.HasMore {
		last := rows[examinedIndex]
		next := pagecursor.Encode(last.CreatedAt, last.cursorID)
		resp.NextCursor = &next
	}
	return resp, nil
}

func queryBatch(ctx context.Context, tx *sql.Tx, practiceID string, after *pagecursor.Cursor) ([]rawEntry, error) {
	var rows *sql.Rows
	var err error
	if after == nil {
		rows, err = tx.QueryContext(ctx, listPracticeActivityQuery, practiceID)
	} else {
		rows, err = tx.QueryContext(ctx, listPracticeActivityAfterQuery, practiceID, after.At, after.ID)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("activityfeed: query practice batch: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []rawEntry{}
	for rows.Next() {
		var row rawEntry
		var actorKind string
		var staffName, clientGivenName, clientPreferredName, subjectName sql.NullString
		if err := rows.Scan(&row.cursorID, &row.SubjectKind, &row.SubjectID, &row.Action, &actorKind,
			&staffName, &clientGivenName, &clientPreferredName, &subjectName, &row.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("activityfeed: scan practice activity row: %w", err)
		}
		row.ActorKind = actorKind
		row.ActorName = resolveActorName(actorKind, staffName, clientGivenName, clientPreferredName)
		row.SubjectName = resolveSubjectName(row.SubjectKind, subjectName)
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("activityfeed: iterate practice activity rows: %w", err)
	}
	return items, nil
}

// resolveSubjectName says who a row happened to, for the subject kinds
// whose subject_id is a person (#1148). Only membership is one today:
// its subject_id is a staff_id, and the roster change the row records is
// unreadable without the name -- "Roles changed" alone names the actor
// and leaves the reader to guess whose roles moved. An Engagement's or a
// Client's own rows name a record the reader is looking at or can reach
// by id, and are left empty rather than given a second, differently-
// shaped label.
//
// Where this degrades, said plainly rather than left to be discovered:
// staff_practice_visibility (00002) reaches a staff row only through a
// live practice_memberships row, so once a person leaves, EVERY row about
// her loses its name at once -- the 'removed' event itself, whose subject
// is by construction somebody whose Membership has just gone, and equally
// the 'joined' and 'roles_changed' rows still sitting further down the
// same feed from when she was here. The feed says
// activity.DepartedStaffName -- the same word #887
// settled on for an actor who has left, so a Practice never meets two
// different words for the same absence. Recovering the name would mean
// either a fourth policy on staff, which #1077 measured the cost of, or
// copying the name into the event's diff at write time; neither is this
// ticket's to spend.
func resolveSubjectName(subjectKind string, subjectName sql.NullString) string {
	if subjectKind != personSubjectKind {
		return ""
	}
	if !subjectName.Valid {
		return activity.DepartedStaffName
	}
	return subjectName.String
}

// resolveActorName mirrors engagement.listEngagementActivity's own
// switch: a staff actor's name, a client actor's PreferredName, or
// activity.SystemActorName for a system actor -- every row this package
// returns already carries a name a reader never has to look up itself.
func resolveActorName(actorKind string, staffName, clientGivenName, clientPreferredName sql.NullString) string {
	switch actorKind {
	case "staff":
		// A Staff actor whose Membership has ended is unreachable through
		// staff_practice_visibility (00002), so the join finds nothing and
		// the Who column would render blank. #1148 makes that a certainty
		// rather than an edge: a 'removed' row written by a person
		// deleting her own login has actor and subject as the same
		// departed person, so both halves of the sentence resolve to
		// nothing at once. Say the same word the subject side and #872's
		// own membership history already say -- a Practice must not meet
		// two different words, or an empty cell, for one absence.
		if !staffName.Valid {
			return activity.DepartedStaffName
		}
		return staffName.String
	case "client":
		return client.PreferredName(clientGivenName.String, clientPreferredName.String)
	default:
		return activity.SystemActorName
	}
}
