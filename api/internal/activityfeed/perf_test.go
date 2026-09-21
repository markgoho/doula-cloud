package activityfeed

import (
	"os"
	"strings"
	"testing"

	"doula-cloud/api/internal/activitypage"
	"doula-cloud/api/internal/testdb"
)

// TestListPracticeActivityQuery_ComposesTheSharedActorJoin is #1281's own
// AC1: the practice-wide feed and activitypage reach the actor's three
// name columns through one spelling, activitypage.SharedActorJoin, rather
// than two hand-written copies of the same join and the same columns.
//
// It also pins the query text this package's own EXPLAIN comment measured
// (practice.go's doc comment above listPracticeActivityQueryTemplate):
// composing SharedActorJoin must not change a single byte of what
// PracticeHandler sends Postgres, or the recorded plan and timings stop
// describing the query that actually ships.
func TestListPracticeActivityQuery_ComposesTheSharedActorJoin(t *testing.T) {
	for _, query := range []string{listPracticeActivityQuery, listPracticeActivityAfterQuery} {
		if !strings.Contains(query, activitypage.SharedActorJoin.Columns) {
			t.Errorf("query does not contain SharedActorJoin.Columns:\n%s", query)
		}
		if !strings.Contains(query, activitypage.SharedActorJoin.Joins) {
			t.Errorf("query does not contain SharedActorJoin.Joins:\n%s", query)
		}
	}

	const wantColumns = `s.name, c.given_name, c.preferred_name, subj.name, a.created_at`
	if !strings.Contains(listPracticeActivityQuery, wantColumns) {
		t.Errorf("listPracticeActivityQuery select list = %q, want it to contain %q byte-for-byte", listPracticeActivityQuery, wantColumns)
	}

	const wantJoins = "LEFT JOIN staff s ON s.id = a.actor_staff_id\n\tLEFT JOIN clients c ON c.id = a.actor_client_id\n\tLEFT JOIN staff subj ON"
	if !strings.Contains(listPracticeActivityQuery, wantJoins) {
		t.Errorf("listPracticeActivityQuery joins = %q, want it to contain %q byte-for-byte", listPracticeActivityQuery, wantJoins)
	}
}

// TestPracticeQueryPlanAtScale is not part of the coverage gate's normal
// run -- it seeds several thousand rows, which is disproportionate to run
// on every `go test ./...` (docs/testing.md's own Podman infra is shared
// with other worktrees). Run it deliberately:
//
//	RUN_PERF_TEST=1 go test ./internal/activityfeed/... -run TestPracticeQueryPlanAtScale -v
//
// It exists to answer AC8 -- "pagination stays fast at a 14-doula
// agency's volume, not a fixture's" -- without a new index
// (activity_subject, 00058_activity_subject_id_index.sql, is a
// (practice_id, subject_kind, subject_id, created_at, id) prefix that
// cannot serve a practice-wide ORDER BY created_at; adding one is a
// schema change #486 rules out of scope). See practice.go's own query
// doc comment for the captured plan this test produced.
//
// It lives in package activityfeed itself, not activityfeed_test alongside
// this package's other tests, so it can EXPLAIN listPracticeActivityQuery
// directly -- the exact text PracticeHandler runs -- rather than a
// hand-copied literal free to drift from it (a bare `LIMIT 121` here once
// stopped matching practiceBatchSize+1 unnoticed).
func TestPracticeQueryPlanAtScale(t *testing.T) {
	if os.Getenv("RUN_PERF_TEST") == "" {
		t.Skip("set RUN_PERF_TEST=1 to run (seeds thousands of rows)")
	}

	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Perf Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "perf-owner", []string{"owner"}, "employee")

	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Perf Client', 'perf@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	var engagementID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, 'active', 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}

	// A 14-doula agency's realistic volume, per the design brief's own
	// eleven-events-on-a-page estimate for one Engagement's ledger: 5,000
	// rows is generous headroom over what even a busy pilot Practice
	// accumulates in its first couple of years, seeded directly (bulk
	// INSERT, not 5,000 round trips through activity.Record) since this
	// test is about the read query's plan, not the write path.
	const rowCount = 5000
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id, created_at)
		 SELECT $1, 'engagement', $2, 'visit_logged', '{}'::jsonb, 'staff', $3,
		        now() - (n || ' minutes')::interval
		 FROM generate_series(1, $4) AS n`,
		practiceID, engagementID, ownerID, rowCount,
	); err != nil {
		t.Fatalf("seed activity rows: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}

	rows, err := tx.QueryContext(t.Context(), "EXPLAIN (ANALYZE, BUFFERS) "+listPracticeActivityQuery, practiceID)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var plan []string
	t.Logf("EXPLAIN (ANALYZE, BUFFERS) for practice-wide feed query at %d rows:", rowCount)
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain line: %v", err)
		}
		t.Log(line)
		plan = append(plan, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate explain lines: %v", err)
	}

	assertNoJIT(t, plan)
}

// assertNoJIT fails the test if plan carries a JIT section -- the one
// number 00111's own history (00111_staff_name_survives_a_departure.sql)
// says actually matters: a wall-clock number moves with the machine, a
// JIT section means the plan crossed jit_above_cost and a policy on
// staff just repeated 00105's own failure.
func assertNoJIT(t *testing.T, plan []string) {
	t.Helper()
	for _, line := range plan {
		if strings.Contains(line, "JIT:") {
			t.Fatalf("query JITs at this scale -- exactly the failure mode 00111 measured for a plain EXISTS policy: %s", line)
		}
	}
}

// TestPracticeQueryPlanWithDepartedMembershipSubjects is #1256's AC3:
// 00116 added a fourth policy on staff, so its plan cost is measured the
// way 00111 measured its own -- plan shape, and whether the query JITs.
//
// TestPracticeQueryPlanAtScale's own caveat says why this needs a
// separate fixture: its 5,000 rows are all subject_kind = 'engagement'
// with one live actor, so the subj join -- and 00116's own policy -- are
// never driven at all. This fixture seeds departed Membership subjects
// instead, so both the join and the policy actually run the way a real
// Practice's roster history would drive them.
//
// Run it deliberately, the same way:
//
//	RUN_PERF_TEST=1 go test ./internal/activityfeed/... -run TestPracticeQueryPlanWithDepartedMembershipSubjects -v
func TestPracticeQueryPlanWithDepartedMembershipSubjects(t *testing.T) {
	if os.Getenv("RUN_PERF_TEST") == "" {
		t.Skip("set RUN_PERF_TEST=1 to run (seeds thousands of rows)")
	}

	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Perf Departed Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "perf-departed-owner", []string{"owner"}, "employee")

	// 5,000 departed Doulas: each held a Membership, each was removed by
	// the Owner, and each 'removed' row is what this migration's policy
	// has to admit a name for -- the worst case for the policy's own
	// added cost, not the best one.
	const rowCount = 5000
	if _, err := db.Admin.ExecContext(t.Context(),
		`WITH departed AS (
		     INSERT INTO staff (identity_uid, name, email, work_state)
		     SELECT 'perf-departed-' || n, 'Perf Doula ' || n, 'perf-departed-' || n || '@example.com', 'NY'
		     FROM generate_series(1, $2) AS n
		     RETURNING id
		 )
		 INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id, created_at)
		 SELECT $1, 'membership', id, 'removed', '{}'::jsonb, 'staff', $3,
		        now() - (row_number() OVER () || ' minutes')::interval
		 FROM departed`,
		practiceID, rowCount, ownerID,
	); err != nil {
		t.Fatalf("seed departed membership rows: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}

	rows, err := tx.QueryContext(t.Context(), "EXPLAIN (ANALYZE, BUFFERS) "+listPracticeActivityQuery, practiceID)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var plan []string
	t.Logf("EXPLAIN (ANALYZE, BUFFERS) for practice-wide feed query at %d departed Membership subjects:", rowCount)
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain line: %v", err)
		}
		t.Log(line)
		plan = append(plan, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate explain lines: %v", err)
	}

	assertNoJIT(t, plan)
}
