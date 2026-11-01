package client

import (
	"os"
	"strings"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestCountClientsQueryPlanAtScale prices countClientsQuery (#2095) at
// 5,000 Clients for one Practice, well past a
// fourteen-doula agency's records, and fails if the plan JITs. It seeds
// thousands of rows, so it runs only when asked:
//
//	RUN_PERF_TEST=1 go test ./internal/client/ -run TestCountClientsQueryPlanAtScale -v
//
// It lives in package client so it EXPLAINs the constant the handler
// sends, not a copy of it.
func TestCountClientsQueryPlanAtScale(t *testing.T) {
	if os.Getenv("RUN_PERF_TEST") == "" {
		t.Skip("set RUN_PERF_TEST=1 to run (seeds thousands of rows)")
	}

	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Count Perf Practice")
	const rowCount = 5000
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, family_name)
		 SELECT $1, 'Perf', 'Client ' || n FROM generate_series(1, $2) AS n`,
		practiceID, rowCount); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}

	rows, err := tx.QueryContext(t.Context(), "EXPLAIN (ANALYZE, BUFFERS) "+countClientsQuery, practiceID)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain line: %v", err)
		}
		t.Log(line)
		if strings.Contains(line, "JIT:") {
			t.Fatalf("the count JITs at this scale: %s", line)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate explain lines: %v", err)
	}
}
