package practicerate_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/fixture"
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
	e := newEnv(t)
	e.seedRate(fixture.KindBirth, 15000)

	testdb.RequireFailsClosed(t, e.DB, "practice_rates")
}

// TestRLS_PracticeRatesVisibilityIsScopedToCurrentPractice proves the
// plain-column-comparison policy narrows practice_rates to rows for
// app.current_practice_id, not every row globally.
func TestRLS_PracticeRatesVisibilityIsScopedToCurrentPractice(t *testing.T) {
	e := newEnv(t)
	e.seedRate(fixture.KindBirth, 15000)
	other := fixture.Practice(t, e.DB, fixture.PracticeName("Practice B"))
	testdb.SeedPracticeRate(t, e.DB, other.ID, fixture.KindBirth, 20000)

	tx := e.DB.BeginAs(t, testdb.Practice(e.Practice.ID))

	if visible := testdb.Column[string](t, tx, `SELECT practice_id FROM practice_rates`); !slices.Equal(visible, []string{e.Practice.ID}) {
		t.Fatalf("visible practice_ids = %v, want only %q", visible, e.Practice.ID)
	}
}

// TestRLS_PracticeRatesUpdateRejectedAcrossPractice proves the policy
// scopes UPDATE, not just SELECT: a session acting as one Practice can't
// edit a rate that belongs to another, even naming the correct row,
// because the row isn't visible to update in the first place.
func TestRLS_PracticeRatesUpdateRejectedAcrossPractice(t *testing.T) {
	e := newEnv(t)
	other := fixture.Practice(t, e.DB, fixture.PracticeName("Practice B"))
	testdb.SeedPracticeRate(t, e.DB, other.ID, fixture.KindBirth, 15000)

	tx := e.DB.BeginAs(t, testdb.Practice(e.Practice.ID))

	if rows := testdb.RowsAffected(t, tx, `UPDATE practice_rates SET amount_cents = 99999 WHERE practice_id = $1`, other.ID); rows != 0 {
		t.Fatalf("expected 0 rows affected editing a rate at a different Practice, got %d", rows)
	}
}
