package client

import (
	"database/sql"
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

	tx := db.BeginAs(t, testdb.Practice(practiceID))

	requireNoJIT(t, tx, countClientsQuery, "the count", practiceID)
}

// requireNoJIT logs the plan EXPLAIN ANALYZE gives query and fails if it
// JITs, which is what the handler's queries must not do at this scale.
func requireNoJIT(t *testing.T, tx *sql.Tx, query, what string, args ...any) {
	t.Helper()
	for _, line := range testdb.Column[string](t, tx, "EXPLAIN (ANALYZE, BUFFERS) "+query, args...) {
		t.Log(line)
		if strings.Contains(line, "JIT:") {
			t.Fatalf("%s JITs at this scale: %s", what, line)
		}
	}
}
