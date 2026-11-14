package client

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/overdue"
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
		 INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, reference, due_at, paid_at)
		 SELECT $1, k.id, gen_random_uuid()::text, s.status::invoice_status, 75000, gen_random_uuid()::text,
		        now() + (CASE WHEN random() < 0.5 THEN interval '-12 days' ELSE interval '14 days' END),
		        CASE WHEN s.status = 'paid' THEN now() END
		   FROM k CROSS JOIN (VALUES ('paid'), ('open')) AS s(status)`,
		practiceID, rowCount); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tx := db.BeginAs(t, testdb.Practice(practiceID))

	args := firstPageClientIDs(t, tx, practiceID)
	placeholders := make([]string, len(args))
	for i := range args {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	staffIdx, attachedOnlyIdx := len(args)+1, len(args)+2
	args = append(args, "00000000-0000-0000-0000-000000000000", false)
	query := fmt.Sprintf(openEngagementRollupQueryTemplate, staffIdx, strings.Join(placeholders, ", "), attachedOnlyIdx)

	requireNoJIT(t, tx, query, "the rollup", args...)
}

// TestListQueryPlanAtScale prices the Clients list's own query
// (listClientsQuery: the due-date lateral, the Clients-with-work test and the
// ?doula and ?q narrowing) for the first page and a later page at a Practice
// of 5,000 Clients, each with one active Engagement due on one of 300 days,
// one in fourteen attached to the narrowed Doula, and fails past a generous
// execution ceiling, after disableJIT as the handler does (without it the plan
// JITs: listClientsQuery's comment has the measurement). Gated like the
// rollup's:
//
//	RUN_PERF_TEST=1 go test ./internal/client/ -run TestListQueryPlanAtScale -v
//
// The plan each case found is recorded on listClientsQuery's callers in
// list.go, beside the rollup's.
const maxListExecutionMillis float64 = 500

func TestListQueryPlanAtScale(t *testing.T) {
	if os.Getenv("RUN_PERF_TEST") == "" {
		t.Skip("set RUN_PERF_TEST=1 to run (seeds thousands of rows)")
	}

	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "List Perf Practice")
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "list-perf-doula", []string{"doula"}, "employee")
	const rowCount = 5000
	if _, err := db.Admin.ExecContext(t.Context(),
		`WITH c AS (
		     INSERT INTO clients (practice_id, given_name, family_name)
		     SELECT $1, 'Perf', 'Client ' || n FROM generate_series(1, $2) AS n
		     RETURNING id
		 ), e AS (
		     INSERT INTO engagements (client_id, practice_id, status, kind, due_date)
		     SELECT id, $1, 'active', 'birth', current_date + (row_number() OVER ())::int % 300 FROM c
		     RETURNING id
		 ), k AS (
		     INSERT INTO contracts (engagement_id, status, prose, amount_cents)
		     SELECT id, 'signed', '', 150000 FROM e
		     RETURNING id
		 ), i AS (
		     INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, reference, due_at)
		     SELECT $1, k.id, gen_random_uuid()::text, 'open'::invoice_status, 75000, gen_random_uuid()::text, now() - (row_number() OVER ())::int % 60 * interval '1 day'
		       FROM k
		     RETURNING id
		 )
		 INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by)
		 SELECT id, $3, 'granted'::attachment_origin, $3 FROM (SELECT id, row_number() OVER () AS n FROM e) numbered WHERE n % 14 = 0`,
		practiceID, rowCount, doulaID); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(), `ANALYZE clients, engagements, engagement_attachments, contracts, invoices`); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	tx := db.BeginAs(t, testdb.Practice(practiceID))

	if err := disableJIT(t.Context(), tx); err != nil {
		t.Fatalf("disable jit: %v", err)
	}

	const zeroID = "00000000-0000-0000-0000-000000000000"
	middle := listAfter{at: time.Now().AddDate(0, 0, 150).UTC().Truncate(24 * time.Hour), id: zeroID}
	middleName := listAfter{name: "perf client 2500", id: zeroID}
	middleOwed := listAfter{owed: 75000, at: middle.at, id: zeroID}
	cutoff := overdue.From(time.Now(), time.UTC)
	middleLate := listAfter{late: cutoff.AddDate(0, 0, -30), at: middle.at, id: zeroID}
	for name, c := range map[string]struct {
		filters listFilters
		order   string
		after   *listAfter
	}{
		"first page, default filter":     {listFilters{withWorkOnly: true}, orderDue, nil},
		"later page":                     {listFilters{withWorkOnly: true}, orderDue, &middle},
		"doula":                          {listFilters{withWorkOnly: true, doula: doulaID}, orderDue, nil},
		"name":                           {listFilters{withWorkOnly: true, name: "client 49"}, orderDue, nil},
		"doula, name, owes, no contract": {listFilters{withWorkOnly: true, doula: doulaID, name: "client 4", owes: true, noContract: true}, orderDue, nil},
		"order by name, first page":      {listFilters{withWorkOnly: true}, orderName, nil},
		"order by name, later page":      {listFilters{withWorkOnly: true}, orderNameDesc, &middleName},
		"order by owed, first page":      {listFilters{withWorkOnly: true}, orderOwed, nil},
		"order by owed, later page":      {listFilters{withWorkOnly: true}, orderOwed, &middleOwed},
		"order by owed, doula":           {listFilters{withWorkOnly: true, doula: doulaID}, orderOwed, nil},
		"order by overdue, first page":   {listFilters{withWorkOnly: true, overdueFrom: cutoff}, orderOverdue, nil},
		"order by overdue, later page":   {listFilters{withWorkOnly: true, overdueFrom: cutoff}, orderOverdue, &middleLate},
		"order by overdue, doula":        {listFilters{withWorkOnly: true, overdueFrom: cutoff, doula: doulaID}, orderOverdue, nil},
	} {
		query, args := listClientsQuery(practiceID, c.filters, c.order, c.after)
		explainWithinCeiling(t, tx, name, query, args)
	}
}

// explainWithinCeiling logs the plan of query and fails past
// maxListExecutionMillis.
func explainWithinCeiling(t *testing.T, tx *sql.Tx, name, query string, args []any) {
	t.Helper()
	rows, err := tx.QueryContext(t.Context(), "EXPLAIN (ANALYZE, BUFFERS) "+query, args...) //nolint:gosec // query is the handler's own statement; every value rides args
	if err != nil {
		t.Fatalf("%s: explain: %v", name, err)
	}
	defer func() { _ = rows.Close() }()
	t.Logf("--- %s", name)
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("%s: scan explain line: %v", name, err)
		}
		t.Log(line)
		if ms, ok := strings.CutPrefix(line, "Execution Time: "); ok {
			elapsed, err := strconv.ParseFloat(strings.TrimSuffix(ms, " ms"), 64)
			if err != nil || elapsed > maxListExecutionMillis {
				t.Fatalf("%s: execution took %q, want at most %.0f ms", name, ms, maxListExecutionMillis)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s: iterate explain lines: %v", name, err)
	}
}

// firstPageClientIDs is the list's first page: the Practice's thirty
// newest Clients, as bind arguments.
func firstPageClientIDs(t *testing.T, tx *sql.Tx, practiceID string) []any {
	t.Helper()
	rows, err := tx.QueryContext(t.Context(),
		`SELECT id FROM clients WHERE practice_id = $1 ORDER BY created_at DESC, id DESC LIMIT 30`, practiceID)
	if err != nil {
		t.Fatalf("page ids: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []any
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan id: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate ids: %v", err)
	}
	return ids
}
