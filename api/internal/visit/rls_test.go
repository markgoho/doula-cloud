package visit_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policy from 00007_visit.sql directly via
// db.App and set_config, bypassing the Go handlers -- proving the SQL
// policy itself scopes visibility (and mutation) correctly, not just that
// the handlers happen to agree with it.

func TestRLS_VisitsFailsClosedWithNoPracticeSet(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := seedDoulaWithMembership(t, db, "fail-closed-doula")
	engagementID := seedEngagement(t, db, practiceID)
	seedVisit(t, db, engagementID, staffID)

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM visits`).Scan(&count); err != nil {
		t.Fatalf("query visits with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_VisitsSelectIsScopedViaEngagementsExistsSubquery proves the
// visits_practice_visibility policy narrows to Visits whose Engagement
// belongs to app.current_practice_id, not to every Visit row globally.
func TestRLS_VisitsSelectIsScopedViaEngagementsExistsSubquery(t *testing.T) {
	db := testdb.New(t)
	practiceA, staffA := seedDoulaWithMembership(t, db, "staff-visits-a")
	practiceB, staffB := seedDoulaWithMembership(t, db, "staff-visits-b")
	engagementA := seedEngagement(t, db, practiceA)
	engagementB := seedEngagement(t, db, practiceB)
	visitAtA := seedVisit(t, db, engagementA, staffA)
	seedVisit(t, db, engagementB, staffB)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM visits`)
	if err != nil {
		t.Fatalf("query visits: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visibleIDs = append(visibleIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visibleIDs) != 1 || visibleIDs[0] != visitAtA {
		t.Fatalf("visible visits = %v, want only %q", visibleIDs, visitAtA)
	}
}

// TestRLS_VisitsUpdateRejectedAcrossPractice proves the policy scopes
// UPDATE, not just SELECT: a session acting as Practice A can't reassign a
// Visit that belongs to Practice B's Engagement, even with the correct
// visit id, because the row isn't visible to update in the first place.
func TestRLS_VisitsUpdateRejectedAcrossPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, staffA := seedDoulaWithMembership(t, db, "staff-update-a")
	practiceB, staffB := seedDoulaWithMembership(t, db, "staff-update-b")
	engagementB := seedEngagement(t, db, practiceB)
	visitAtB := seedVisit(t, db, engagementB, staffB)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	result, err := tx.ExecContext(t.Context(), `UPDATE visits SET staff_id = $1 WHERE id = $2`, staffA, visitAtB)
	if err != nil {
		t.Fatalf("update visits: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 rows affected reassigning a Visit at a different Practice, got %d", rows)
	}
}
