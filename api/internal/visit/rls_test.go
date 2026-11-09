package visit_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policy from 00007_visit.sql directly via
// db.App and set_config, bypassing the Go handlers -- proving the SQL
// policy itself scopes visibility (and mutation) correctly, not just that
// the handlers happen to agree with it.

func TestRLS_VisitsFailsClosedWithNoPracticeSet(t *testing.T) {
	e := newEnv(t)
	seedVisit(t, e.db, e.Engagement.ID, e.Staff.ID)

	testdb.RequireFailsClosed(t, e.db, "visits")
}

// TestRLS_VisitsSelectIsScopedViaEngagementsExistsSubquery proves the
// visits_practice_visibility policy narrows to Visits whose Engagement
// belongs to app.current_practice_id, not to every Visit row globally.
func TestRLS_VisitsSelectIsScopedViaEngagementsExistsSubquery(t *testing.T) {
	a := newEnv(t)
	practiceB := fixture.Practice(t, a.db)
	visitAtA := seedVisit(t, a.db, a.Engagement.ID, a.Staff.ID)
	seedVisit(t, a.db, fixture.Engagement(t, a.db, practiceB).ID, fixture.Staff(t, a.db, practiceB).ID)

	tx := a.db.BeginAs(t, testdb.Practice(a.Practice.ID))

	if visibleIDs := testdb.Column[string](t, tx, `SELECT id FROM visits`); len(visibleIDs) != 1 || visibleIDs[0] != visitAtA {
		t.Fatalf("visible visits = %v, want only %q", visibleIDs, visitAtA)
	}
}

// TestRLS_VisitsUpdateRejectedAcrossPractice proves the policy scopes
// UPDATE, not just SELECT: a session acting as Practice A can't reassign a
// Visit that belongs to Practice B's Engagement, even with the correct
// visit id, because the row isn't visible to update in the first place.
func TestRLS_VisitsUpdateRejectedAcrossPractice(t *testing.T) {
	a := newEnv(t)
	practiceB := fixture.Practice(t, a.db)
	visitAtB := seedVisit(t, a.db, fixture.Engagement(t, a.db, practiceB).ID, fixture.Staff(t, a.db, practiceB).ID)

	tx := a.db.BeginAs(t, testdb.Practice(a.Practice.ID))

	rows := testdb.RowsAffected(t, tx, `UPDATE visits SET staff_id = $1 WHERE id = $2`, a.Staff.ID, visitAtB)
	if rows != 0 {
		t.Fatalf("expected 0 rows affected reassigning a Visit at a different Practice, got %d", rows)
	}
}
