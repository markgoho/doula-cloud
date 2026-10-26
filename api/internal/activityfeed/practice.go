package activityfeed

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activitygate"
	"doula-cloud/api/internal/activitypage"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/personname"
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
// Re-measured after 00116 (#1256) added a fourth policy on staff so the
// subj join's own subject could resolve to a real name once she has
// left: 67 ms on this same all-live-actor fixture (TestPracticeQueryPlan
// AtScale), against the 13.1 ms recorded above -- every one of the
// 5,000 actor-staff reads now costs one SECURITY DEFINER function call.
// TestPracticeQueryPlanWithDepartedMembershipSubjects is the fixture the
// caveat above says this file never had: 5,000 rows where the subj join
// really is driven, every one of them a departed Membership subject.
// 140 ms there, still no JIT section in either case -- the number that
// keeps mattering, per #1077 and per 00116's own doc comment, which
// records the version of this policy that priced the Engagement ledger's
// own JIT canary almost to its ceiling and why the shipped shape does
// not.
//
// Re-measured after #1281 composed activitypage.SharedActorJoin into this
// query instead of hand-writing its columns and joins a second time:
// TestListPracticeActivityQuery_ComposesTheSharedActorJoin
// (perf_test.go) pins listPracticeActivityQuery byte-for-byte against the
// text this section describes, so the change could not have moved the
// plan -- and the two fixtures above were re-run to confirm it did not
// (2026-09-21): TestPracticeQueryPlanAtScale, 78.8 ms (against the 67 ms
// recorded above -- machine variance, not a regression, same shape);
// TestPracticeQueryPlanWithDepartedMembershipSubjects, 147.2 ms (against
// 140 ms above). Both still Limit -> Sort (top-N heapsort) -> an
// activity_subject-prefixed scan, still no JIT section in either case.
//
// Re-measured after #2006 added the se and sc joins below (2026-10-26):
// TestPracticeQueryPlanAtScale, 202 ms; TestPracticeQueryPlanWith
// DepartedMembershipSubjects, 419 ms. Both higher than the numbers
// above, and not from the new joins: in the plan, the actor-staff join
// alone took 171 ms of the 194 ms under the Sort, the se join (an
// Index Scan on engagements_pkey, 5,000 loops) about 11 ms, and the sc
// join (a Materialize over the Practice's clients) about 3 ms. The
// Podman VM ran slower this day than the day above. Same shape: Limit ->
// Sort (top-N heapsort) -> an activity_subject-prefixed scan, no JIT
// section.
//
// Re-measured after #2095 read sc.family_name as well (2026-10-26): one
// more column from a row the sc join already fetches, no new join.
// TestPracticeQueryPlanAtScale, 98 ms; TestPracticeQueryPlanWith
// DepartedMembershipSubjects, 176 ms. The same plan shape, no JIT
// section; lower than the numbers just above because the Podman VM was
// quieter, not because anything got cheaper.
//
// Re-measured after #2111 read c.family_name in SharedActorJoin.Columns
// (2026-10-26): one more column from a row the c join already fetches, no
// new join. TestPracticeQueryPlanAtScale, 107 ms; TestPracticeQueryPlan
// WithDepartedMembershipSubjects, 263 ms. The same plan shape, Limit ->
// Sort (top-N heapsort), no JIT section; the spread against the numbers
// above is the shared Podman VM, which three sessions were using at once.
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
// spells a person, free to drift from the staff row -- and, #1256 found,
// unable to honor ADR-0033's redaction, since activity is INSERT-only
// and a diff-carried name would survive a later login deletion forever.
// 00116's RLS policy on staff reads the row's current state instead, so
// a redaction this join's own subject already sits behind takes effect
// the moment it is written, with nothing here to update.
//
// %[2]s is personSubjectKind, interpolated for the same reason
// %[1]d is: a package-internal constant, never request input, and naming
// the constant rather than repeating the literal is what keeps the write
// side and this join from drifting apart.
//
// %[3]s and %[4]s are activitypage.SharedActorJoin's own Columns and
// Joins (#1281): this feed has no single subject to scope by, so it
// cannot call activitypage.Statement itself, but the actor half of its
// row is the same fact activitypage.List resolves for every
// subject-scoped reader, reached the same way. Composing the one
// exported value rather than hand-writing the three columns and two
// joins again is what keeps this query and activitypage's from drifting
// on what those columns are or how they are reached -- #1150 already did
// that for what a name resolves to (ActorName below); this is the SQL
// that reaches it.
//
// %[5]s and %[6]s are activity.SubjectEngagement and SubjectClient: the
// se and sc joins are #2006's, and name the Client an Engagement's row or
// a Client record's own row is about. Like subj, each is guarded by an
// equality on a.subject_kind, so a Membership or Practice row drives
// neither, and each is a primary-key lookup on a table this query
// already reads.
const listPracticeActivityQueryTemplate = `SELECT a.id, a.subject_kind, a.subject_id, a.action, a.actor_kind::text,
	       %[3]s, subj.name, sc.given_name, sc.preferred_name, sc.family_name, a.created_at
	FROM activity a
	%[4]s
	LEFT JOIN staff subj ON a.subject_kind = '%[2]s' AND subj.id = a.subject_id
	LEFT JOIN engagements se ON a.subject_kind = '%[5]s' AND se.id = a.subject_id
	LEFT JOIN clients sc ON sc.id = CASE WHEN a.subject_kind = '%[6]s' THEN a.subject_id ELSE se.client_id END
	WHERE a.practice_id = $1
	ORDER BY a.created_at DESC, a.id DESC LIMIT %[1]d`

const listPracticeActivityAfterQueryTemplate = `SELECT a.id, a.subject_kind, a.subject_id, a.action, a.actor_kind::text,
	       %[3]s, subj.name, sc.given_name, sc.preferred_name, sc.family_name, a.created_at
	FROM activity a
	%[4]s
	LEFT JOIN staff subj ON a.subject_kind = '%[2]s' AND subj.id = a.subject_id
	LEFT JOIN engagements se ON a.subject_kind = '%[5]s' AND se.id = a.subject_id
	LEFT JOIN clients sc ON sc.id = CASE WHEN a.subject_kind = '%[6]s' THEN a.subject_id ELSE se.client_id END
	WHERE a.practice_id = $1
	  AND (a.created_at, a.id) < ($2, $3)
	ORDER BY a.created_at DESC, a.id DESC LIMIT %[1]d`

var listPracticeActivityQuery = fmt.Sprintf(listPracticeActivityQueryTemplate, //nolint:gosec // all six interpolated values are package-internal constants (two activitypage's own, two activity's), not request input
	practiceBatchSize+1, personSubjectKind, activitypage.SharedActorJoin.Columns, activitypage.SharedActorJoin.Joins,
	activity.SubjectEngagement, activity.SubjectClient)

var listPracticeActivityAfterQuery = fmt.Sprintf(listPracticeActivityAfterQueryTemplate, //nolint:gosec // all six interpolated values are package-internal constants (two activitypage's own, two activity's), not request input
	practiceBatchSize+1, personSubjectKind, activitypage.SharedActorJoin.Columns, activitypage.SharedActorJoin.Joins,
	activity.SubjectEngagement, activity.SubjectClient)

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
		var staffName, clientGivenName, clientPreferredName, clientFamilyName, subjectName sql.NullString
		var subjectClientGivenName, subjectClientPreferredName, subjectClientFamilyName sql.NullString
		if err := rows.Scan(&row.cursorID, &row.SubjectKind, &row.SubjectID, &row.Action, &actorKind,
			&staffName, &clientGivenName, &clientPreferredName, &clientFamilyName, &subjectName,
			&subjectClientGivenName, &subjectClientPreferredName, &subjectClientFamilyName, &row.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("activityfeed: scan practice activity row: %w", err)
		}
		row.ActorKind = actorKind
		// #2111: this feed sets every Client at the Practice side by side,
		// so a Client actor reads her full name, as the subject name beside
		// it does.
		row.ActorName = activitypage.ActorName(actorKind, staffName, clientGivenName, clientPreferredName, clientFamilyName, activitypage.ActorFullName)
		row.SubjectName = resolveSubjectName(row.SubjectKind, subjectName, subjectClientGivenName, subjectClientPreferredName, subjectClientFamilyName)
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
// Client's own rows name the Client (#2006): this feed spans every Client
// the Practice holds, and "Contract sent" without a name left the reader
// unable to say whose. The name is read through the Client's row as it
// stands, never a copy in the diff, so an Erasure's redaction (ADR-0027)
// reaches every row at once -- the same reason #1256 gave for staff. A
// Practice's own rows (#1255) stay empty: subject_id is the reader's own
// practice_id, so a name would only print the Practice's own name on
// every row of its own feed -- decided empty, deliberately, not merely
// left off.
//
// #1256 closed the gap this comment used to describe: staff_practice_
// visibility (00002) reaches a staff row only through a live
// practice_memberships row, so a departed person's row was unreachable
// here -- the 'removed' event itself, whose subject is by construction
// somebody whose Membership has just gone, and equally the 'joined' and
// 'roles_changed' rows still sitting further down the same feed from
// when she was here. 00116's staff_visible_to_own_practice_membership_
// history policy is the second door: SECURITY DEFINER-shaped like 00111,
// so the planner cannot inline it into this query's plan, it admits a
// staff row this session's Practice has ever recorded a Membership
// event about, whether or not her own membership is still live. subj.name
// (the query's own LEFT JOIN, unchanged by this function) is populated
// whenever either policy admits the row, so subjectName.Valid alone still
// decides real name vs. activity.DepartedStaffName -- this function did
// not need to change shape, only what the column beneath it can see.
//
// activity.DepartedStaffName still covers the one case 00116 leaves
// alone on purpose: a staff row ADR-0033 has redacted. That route was
// chosen over writing the name into the event's diff at write time
// specifically because the diff route cannot honor ADR-0033 -- activity
// is INSERT-only (ADR-0022), so a name baked into a diff would outlive a
// later login deletion and hand back her real name forever, which is
// exactly what ADR-0033's redaction promises stops happening. 00116's own
// doc comment carries the full reasoning and the cost measurement #1077
// asked any fourth staff policy to repeat.
func resolveSubjectName(subjectKind string, subjectName, clientGivenName, clientPreferredName, clientFamilyName sql.NullString) string {
	if subjectKind != personSubjectKind {
		// #2006: an Engagement's row and a Client record's own row name
		// the Client. #2095 made it the full name (personname.Full): this
		// feed spans every Client at the Practice, and two of them can
		// share a first name. Not
		// Valid for a Practice row, which joins no Client, and for a row
		// whose Client this reader's RLS does not reach -- a row the gate
		// refuses anyway.
		if !clientGivenName.Valid {
			return ""
		}
		return personname.Full(clientGivenName.String, clientPreferredName.String, clientFamilyName.String)
	}
	if !subjectName.Valid {
		return activity.DepartedStaffName
	}
	return subjectName.String
}
