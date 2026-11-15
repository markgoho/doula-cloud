package contracts

import (
	"database/sql"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"doula-cloud/api/internal/testdb"
)

const maxListExecutionMillis = 500

var executionTime = regexp.MustCompile(`Execution Time: ([0-9.]+) ms`)

// TestListQueryPlanAtScale prices the Contracts list's page query for the
// first page in each view and order at a Practice of 5,000 Contracts spread
// over the four states, well past a fourteen-doula agency's book. As the app
// role, under row security, every view in every order has to run under a
// generous ceiling after JIT is off, as the handler's transaction leaves it.
// Row security keeps the planner off any index on status, so the plan sorts
// the Practice's Contracts and the ceiling is the whole assertion. It
// seeds thousands of rows, so it runs only when asked:
//
//	RUN_PERF_TEST=1 go test ./internal/contracts/ -run TestListQueryPlanAtScale -v
//
// It lives in package contracts so it EXPLAINs the query the handler builds,
// not a copy of it.
func TestListQueryPlanAtScale(t *testing.T) {
	if os.Getenv("RUN_PERF_TEST") == "" {
		t.Skip("set RUN_PERF_TEST=1 to run (seeds thousands of rows)")
	}

	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Contracts Perf Practice")
	const rowCount = 5000
	if _, err := db.Admin.ExecContext(t.Context(),
		`WITH c AS (
		     INSERT INTO clients (practice_id, given_name, family_name)
		     SELECT $1, 'Perf', 'Client ' || n FROM generate_series(1, $2) AS n
		     RETURNING id
		 ), e AS (
		     INSERT INTO engagements (client_id, practice_id, status, kind)
		     SELECT id, $1, 'active', 'birth' FROM c
		     RETURNING id
		 ), numbered AS (
		     SELECT id, row_number() OVER () AS n FROM e
		 )
		 INSERT INTO contracts (engagement_id, status, prose, amount_cents, created_at, sent_at, signed_at, voided_at)
		 SELECT id, s.status::contract_status, '', 150000,
		        now() - (n % 400 + 4) * interval '1 hour',
		        CASE WHEN s.status <> 'draft' THEN now() - (n % 400 + 3) * interval '1 hour' END,
		        CASE WHEN s.status IN ('signed', 'voided') THEN now() - (n % 400 + 2) * interval '1 hour' END,
		        CASE WHEN s.status = 'voided' THEN now() - (n % 400 + 1) * interval '1 hour' END
		   FROM numbered
		   JOIN (VALUES (0, 'draft'), (1, 'sent'), (2, 'signed'), (3, 'voided')) AS s(slot, status) ON s.slot = n % 4`,
		practiceID, rowCount); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(), `ANALYZE clients, engagements, contracts`); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	tx := db.BeginAs(t, testdb.Practice(practiceID))
	if _, err := tx.ExecContext(t.Context(), `SET LOCAL jit = off`); err != nil {
		t.Fatalf("disable jit: %v", err)
	}

	for _, viewName := range []string{viewNotSigned, viewDraft, viewSent, viewSigned, viewVoided, viewAll} {
		for _, order := range []string{orderViewOwn, orderOldest, orderNewest, orderClient} {
			view, _ := resolveView(viewName)
			sort, _ := resolveSort(order, view)
			q := listQuery{statuses: view.statuses, sort: sort}
			query, args := q.pageQuery(practiceID, nil, listPageSize+1)

			plan := explain(t, tx, query, args)
			m := executionTime.FindStringSubmatch(plan)
			if m == nil {
				t.Fatalf("no execution time in plan:\n%s", plan)
			}
			millis, _ := strconv.ParseFloat(m[1], 64)
			t.Logf("view %q order %q: %.1f ms", viewName, order, millis)
			if millis > maxListExecutionMillis {
				t.Errorf("view %q order %q ran %.1f ms, ceiling %d ms:\n%s", viewName, order, millis, maxListExecutionMillis, plan)
			}
		}
	}
}

func explain(t *testing.T, tx *sql.Tx, query string, args []any) string {
	t.Helper()
	rows, err := tx.QueryContext(t.Context(), `EXPLAIN (ANALYZE, BUFFERS) `+query, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		plan.WriteString(line + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	return plan.String()
}
