package engagement

import (
	"encoding/json"
	"strconv"
	"testing"

	"doula-cloud/api/internal/activitypage"
	"doula-cloud/api/internal/testdb"
)

// TestListEngagementActivityQuery_StaysOffTheJITCliff is #1077's guard,
// and it is here rather than in a comment because a comment does not
// fail a build.
//
// This query joins staff three times, which makes it the most expensive
// reader of that table's RLS policies in the codebase and therefore the
// canary for anything added to them. The RLS rewriter inlines a policy's
// subquery into every query that reads the table, and an inlined EXISTS
// over another RLS-bearing table drags that table's policies in behind
// it. #1077 measured what that costs: one such policy took this query's
// plan from 171 lines to 301, pushed its estimated cost past
// jit_above_cost, and Postgres then spent 6675 ms compiling 1828
// functions -- per statement, against an actual row cost under 40 ms.
// Nothing below the HTTP layer showed it, which is why it reached CI as
// "POST .../offers never returns".
//
// So the invariant is the estimated cost, not a millisecond budget: as
// long as the plan stays under jit_above_cost, Postgres never reaches
// for LLVM and the cliff cannot be fallen off. The threshold is read
// from the server rather than written down here, so this asserts what
// the database will actually decide.
//
// This runs against a freshly migrated database with no ANALYZE, which
// is deliberately the worst case and exactly what CI and a new Practice
// both have: the planner works from default estimates, which is when
// this cost is highest.
func TestListEngagementActivityQuery_StaysOffTheJITCliff(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "JIT Canary Practice")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Nadia Client", "jit-canary@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var threshold string
	if err := tx.QueryRowContext(t.Context(), `SHOW jit_above_cost`).Scan(&threshold); err != nil {
		t.Fatalf("read jit_above_cost: %v", err)
	}
	limit, err := strconv.ParseFloat(threshold, 64)
	if err != nil {
		t.Fatalf("parse jit_above_cost %q: %v", threshold, err)
	}
	// -1 disables JIT entirely, which no configuration this repo ships
	// sets, and which would make this assertion vacuous rather than
	// wrong.
	if limit < 0 {
		t.Skipf("jit_above_cost = %s, so this server never JITs and there is no cliff to guard", threshold)
	}

	// The statement is asked of activitypage rather than written out
	// again here (#1150): a guard that EXPLAINs a hand-copied literal
	// stops guarding the shipped query the moment either one moves.
	// moneyGate false is deliberate -- that is the contractor's read,
	// the only one that carries the action-exclusion predicate, and so
	// the heavier of the two plans this reader issues.
	query, args := activitypage.Statement(activityQuery(practiceID, engagementID, false, nil), activityProjection)

	var raw []byte
	if err := tx.QueryRowContext(t.Context(), "EXPLAIN (FORMAT JSON) "+query, args...).Scan(&raw); err != nil {
		t.Fatalf("explain activity query: %v", err)
	}

	var explained []struct {
		Plan struct {
			TotalCost float64 `json:"Total Cost"`
		} `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &explained); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	if len(explained) != 1 {
		t.Fatalf("plans = %d, want 1", len(explained))
	}

	if cost := explained[0].Plan.TotalCost; cost >= limit {
		t.Fatalf("the Engagement Activity query's estimated cost is %.0f, at or above jit_above_cost (%.0f), "+
			"so Postgres will LLVM-compile it on every call. Something has been added to the RLS policies on a table "+
			"this query reads -- see 00111_staff_name_survives_a_departure.sql for what that costs and how to reach "+
			"a Staff row from a policy without paying it.", cost, limit)
	}
}
