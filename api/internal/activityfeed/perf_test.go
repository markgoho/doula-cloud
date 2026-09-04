package activityfeed

import (
	"os"
	"testing"

	"doula-cloud/api/internal/testdb"
)

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
	t.Logf("EXPLAIN (ANALYZE, BUFFERS) for practice-wide feed query at %d rows:", rowCount)
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain line: %v", err)
		}
		t.Log(line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate explain lines: %v", err)
	}
}
