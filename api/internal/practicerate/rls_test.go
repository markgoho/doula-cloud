package practicerate_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the practice_rates_practice_visibility RLS policy
// from 00096_practice_rates.sql directly via db.App and set_config,
// bypassing the Go handlers -- the same shape as contracts' own
// rls_test.go for contract_templates.

// TestRLS_PracticeRatesFailsClosedWithNoSessionVarSet proves
// practice_rates denies all rows when app.current_practice_id is never
// set.
func TestRLS_PracticeRatesFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Some Practice")
	seedRate(t, db, practiceID, 15000)

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM practice_rates`).Scan(&count); err != nil {
		t.Fatalf("query practice_rates with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

// TestRLS_PracticeRatesVisibilityIsScopedToCurrentPractice proves the
// plain-column-comparison policy narrows practice_rates to rows for
// app.current_practice_id, not every row globally.
func TestRLS_PracticeRatesVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	seedRate(t, db, practiceA, 15000)
	seedRate(t, db, practiceB, 20000)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visiblePracticeIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT practice_id FROM practice_rates`)
	if err != nil {
		t.Fatalf("query practice_rates: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visiblePracticeIDs = append(visiblePracticeIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visiblePracticeIDs) != 1 || visiblePracticeIDs[0] != practiceA {
		t.Fatalf("visible practice_ids = %v, want only %q", visiblePracticeIDs, practiceA)
	}
}

// TestRLS_PracticeRatesUpdateRejectedAcrossPractice proves the policy
// scopes UPDATE, not just SELECT: a session acting as Practice A can't
// edit a rate that belongs to Practice B, even naming the correct row,
// because the row isn't visible to update in the first place.
func TestRLS_PracticeRatesUpdateRejectedAcrossPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	seedRate(t, db, practiceB, 15000)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	result, err := tx.ExecContext(t.Context(),
		`UPDATE practice_rates SET amount_cents = 99999 WHERE practice_id = $1`,
		practiceB,
	)
	if err != nil {
		t.Fatalf("update practice_rates: %v", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rowsAffected != 0 {
		t.Fatalf("expected 0 rows affected editing a rate at a different Practice, got %d", rowsAffected)
	}
}
