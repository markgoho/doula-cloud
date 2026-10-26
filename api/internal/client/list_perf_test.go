package client

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestOpenEngagementRollupQueryPlanAtScale prices the Clients list's
// rollup read (openEngagementRollupQueryTemplate, #2130) for one page of
// thirty Clients at a Practice of 5,000 Clients, each with an open
// Engagement, a signed Contract and two Invoices, well past a
// fourteen-doula agency's book, and fails if the plan JITs. It seeds
// thousands of rows, so it runs only when asked:
//
//	RUN_PERF_TEST=1 go test ./internal/client/ -run TestOpenEngagementRollupQueryPlanAtScale -v
//
// It lives in package client so it EXPLAINs the template the handler
// fills, not a copy of it. The template's doc comment records what it
// found.
func TestOpenEngagementRollupQueryPlanAtScale(t *testing.T) {
	if os.Getenv("RUN_PERF_TEST") == "" {
		t.Skip("set RUN_PERF_TEST=1 to run (seeds thousands of rows)")
	}

	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Rollup Perf Practice")
	const rowCount = 5000
	// Each Client: one active Engagement, one signed Contract, and two
	// Invoices -- a paid deposit and an open balance, half of them late.
	if _, err := db.Admin.ExecContext(t.Context(),
		`WITH c AS (
		     INSERT INTO clients (practice_id, given_name, family_name)
		     SELECT $1, 'Perf', 'Client ' || n FROM generate_series(1, $2) AS n
		     RETURNING id
		 ), e AS (
		     INSERT INTO engagements (client_id, practice_id, status, kind)
		     SELECT id, $1, 'active', 'birth' FROM c
		     RETURNING id
		 ), k AS (
		     INSERT INTO contracts (engagement_id, status, prose, amount_cents)
		     SELECT id, 'signed', '', 150000 FROM e
		     RETURNING id
		 )
		 INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, reference, due_at)
		 SELECT $1, k.id, gen_random_uuid()::text, s.status::invoice_status, 75000, gen_random_uuid()::text,
		        now() + (CASE WHEN random() < 0.5 THEN interval '-12 days' ELSE interval '14 days' END)
		   FROM k CROSS JOIN (VALUES ('paid'), ('open')) AS s(status)`,
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

	// The list's first page: its thirty newest Clients.
	idRows, err := tx.QueryContext(t.Context(),
		`SELECT id FROM clients WHERE practice_id = $1 ORDER BY created_at DESC, id DESC LIMIT 30`, practiceID)
	if err != nil {
		t.Fatalf("page ids: %v", err)
	}
	var args []any
	var placeholders []string
	for idRows.Next() {
		var id string
		if err := idRows.Scan(&id); err != nil {
			t.Fatalf("scan id: %v", err)
		}
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	_ = idRows.Close()
	staffIdx, attachedOnlyIdx := len(args)+1, len(args)+2
	args = append(args, "00000000-0000-0000-0000-000000000000", false)
	query := fmt.Sprintf(openEngagementRollupQueryTemplate, staffIdx, strings.Join(placeholders, ", "), attachedOnlyIdx)

	rows, err := tx.QueryContext(t.Context(), "EXPLAIN (ANALYZE, BUFFERS) "+query, args...)
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
			t.Fatalf("the rollup JITs at this scale: %s", line)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate explain lines: %v", err)
	}
}
