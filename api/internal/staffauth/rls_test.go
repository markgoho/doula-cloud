package staffauth_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policies from
// 00002_practice_staff_tenancy.sql directly via db.App and set_config,
// bypassing the Go middleware. The middleware tests above prove the HTTP
// contract (401/403/200); these prove the SQL policies themselves scope
// visibility correctly rather than merely agreeing with the middleware by
// coincidence (e.g. a query returning zero rows because no row exists, not
// because RLS hid it).

func seedPractice(t *testing.T, db *testdb.DB, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(), `INSERT INTO practices (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed practice %q: %v", name, err)
	}
	return id
}

func seedStaff(t *testing.T, db *testdb.DB, identityUID string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Test Staff', 'staff@example.com') RETURNING id`,
		identityUID,
	).Scan(&id); err != nil {
		t.Fatalf("seed staff %q: %v", identityUID, err)
	}
	return id
}

func seedMembership(t *testing.T, db *testdb.DB, practiceID, staffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}', 'employee')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

// TestRLS_StaffFailsClosedWithNoSessionVarsSet proves staff denies all
// rows when neither app.current_practice_id nor app.current_identity_uid
// is set -- the fail-closed case for the table with two OR'd policies,
// where a mistake in either policy could otherwise leak rows.
func TestRLS_StaffFailsClosedWithNoSessionVarsSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Some Practice")
	staffID := seedStaff(t, db, "fail-closed-staff-uid")
	seedMembership(t, db, practiceID, staffID)

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM staff`).Scan(&count); err != nil {
		t.Fatalf("query staff with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_StaffPracticeVisibilityIsScopedToCurrentPractice proves the
// staff_practice_visibility EXISTS-subquery policy narrows the staff
// table to people who hold a membership at app.current_practice_id, not
// to every staff row globally.
func TestRLS_StaffPracticeVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	staffAtA := seedStaff(t, db, "staff-at-a")
	staffAtB := seedStaff(t, db, "staff-at-b")
	seedMembership(t, db, practiceA, staffAtA)
	seedMembership(t, db, practiceB, staffAtB)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM staff`)
	if err != nil {
		t.Fatalf("query staff: %v", err)
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

	if len(visibleIDs) != 1 || visibleIDs[0] != staffAtA {
		t.Fatalf("visible staff = %v, want only %q", visibleIDs, staffAtA)
	}
}

// TestRLS_StaffSelfVisibilityOnlyAppliesBeforePracticeIsChosen proves the
// staff_self_visibility policy stops applying once
// app.current_practice_id has been set, so a caller's own staff row does
// not leak into a request scoped to a Practice they don't belong to.
func TestRLS_StaffSelfVisibilityOnlyAppliesBeforePracticeIsChosen(t *testing.T) {
	db := testdb.New(t)
	unrelatedPractice := seedPractice(t, db, "Unrelated Practice")
	staffID := seedStaff(t, db, "self-visibility-uid")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, "self-visibility-uid"); err != nil {
		t.Fatalf("set_config identity: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM staff WHERE id = $1`, staffID).Scan(&count); err != nil {
		t.Fatalf("query staff before practice set: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected caller's own staff row visible before a Practice is chosen, got count = %d", count)
	}

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, unrelatedPractice); err != nil {
		t.Fatalf("set_config practice: %v", err)
	}

	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM staff WHERE id = $1`, staffID).Scan(&count); err != nil {
		t.Fatalf("query staff after practice set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected caller's own staff row hidden once scoped to a Practice they don't belong to, got count = %d", count)
	}
}

// TestRLS_PracticeMembershipsVisibilityIsScopedToCurrentPractice proves
// the practice_memberships policy narrows to rows for
// app.current_practice_id, not every membership globally.
func TestRLS_PracticeMembershipsVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	staffA := seedStaff(t, db, "member-of-a")
	staffB := seedStaff(t, db, "member-of-b")
	seedMembership(t, db, practiceA, staffA)
	seedMembership(t, db, practiceB, staffB)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM practice_memberships`).Scan(&count); err != nil {
		t.Fatalf("query practice_memberships: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected only Practice A's membership visible, got count = %d", count)
	}
}
