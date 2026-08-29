package clientfieldtemplate_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise 00048's client_field_templates_select/_insert/
// _update policies directly via db.App and set_config, bypassing the Go
// handlers -- proving AC1's "at both seams" holds at the database seam,
// not only PutHandler's own RequireOwnerOrAdmin check.

// beginAs opens a tx on db.App and sets app.current_practice_id and
// app.current_staff_id on it -- the pattern engagementrequest/rls_test.go
// and plans/rls_test.go's sibling tests already use.
func beginAs(t *testing.T, db *testdb.DB, practiceID, staffID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set current_practice_id: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_staff_id', $1, true)`, staffID); err != nil {
		t.Fatalf("set current_staff_id: %v", err)
	}
	return tx
}

func seedStaffID(t *testing.T, db *testdb.DB, identityUID string) string {
	t.Helper()
	var staffID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT id FROM staff WHERE identity_uid = $1`, identityUID).Scan(&staffID); err != nil {
		t.Fatalf("read seeded staff id: %v", err)
	}
	return staffID
}

// TestRLS_SelectFailsClosedWithNoSessionVarSet proves client_field_templates
// denies all rows when app.current_practice_id is never set.
func TestRLS_SelectFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "rls-fail-closed")
	seedTemplate(t, db, practiceID, `[{"id":"f1","type":"short_text","label":"Note","order":0,"archived":false}]`)

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM client_field_templates`).Scan(&count); err != nil {
		t.Fatalf("query client_field_templates with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

// TestRLS_SelectScopedToCurrentPracticeAndAnyRole proves a Doula's
// session (any Staff role, per the sibling GET handler) reads its own
// Practice's row and not another Practice's.
func TestRLS_SelectScopedToCurrentPracticeAndAnyRole(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedDoula(t, db, "rls-select-a")
	practiceB := seedOwner(t, db, "rls-select-b")
	seedTemplate(t, db, practiceA, `[{"id":"a","type":"short_text","label":"A","order":0,"archived":false}]`)
	seedTemplate(t, db, practiceB, `[{"id":"b","type":"short_text","label":"B","order":0,"archived":false}]`)

	tx := beginAs(t, db, practiceA, seedStaffID(t, db, "rls-select-a"))

	var visible []string
	rows, err := tx.QueryContext(t.Context(), `SELECT practice_id FROM client_field_templates`)
	if err != nil {
		t.Fatalf("query client_field_templates: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visible = append(visible, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}
	if len(visible) != 1 || visible[0] != practiceA {
		t.Fatalf("visible practice_ids = %v, want only %q", visible, practiceA)
	}
}

// TestRLS_DoulaInsertRefused proves client_field_templates_insert refuses
// a Doula's INSERT independent of PutHandler's own Go-level check.
func TestRLS_DoulaInsertRefused(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedDoula(t, db, "rls-insert-doula")
	tx := beginAs(t, db, practiceID, seedStaffID(t, db, "rls-insert-doula"))

	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO client_field_templates (practice_id, fields) VALUES ($1, '[]'::jsonb)`, practiceID,
	)
	if err == nil {
		t.Fatal("expected client_field_templates_insert to refuse a Doula's INSERT, got no error")
	}
}

// TestRLS_OwnerInsertAllowed proves the same policy admits an Owner, so
// the Doula refusal above is the role gate working, not a blanket
// refusal.
func TestRLS_OwnerInsertAllowed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "rls-insert-owner")
	tx := beginAs(t, db, practiceID, seedStaffID(t, db, "rls-insert-owner"))

	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO client_field_templates (practice_id, fields) VALUES ($1, '[]'::jsonb)`, practiceID,
	)
	if err != nil {
		t.Fatalf("expected an Owner's INSERT to be admitted, got: %v", err)
	}
}

// TestRLS_AdminInsertAllowed proves RequireOwnerOrAdmin's widened half
// holds in RLS too, not only in Go.
func TestRLS_AdminInsertAllowed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedAdmin(t, db, "rls-insert-admin")
	tx := beginAs(t, db, practiceID, seedStaffID(t, db, "rls-insert-admin"))

	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO client_field_templates (practice_id, fields) VALUES ($1, '[]'::jsonb)`, practiceID,
	)
	if err != nil {
		t.Fatalf("expected an Admin's INSERT to be admitted, got: %v", err)
	}
}

// TestRLS_DoulaUpdateAffectsZeroRows proves client_field_templates_update
// refuses a Doula's UPDATE -- Gap 1 from review: 00042's original policy
// was a bare ALL-commands, any-role grant, and this is the regression
// test pinning the fix.
func TestRLS_DoulaUpdateAffectsZeroRows(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedDoula(t, db, "rls-update-doula")
	seedTemplate(t, db, practiceID, `[{"id":"f1","type":"short_text","label":"Note","order":0,"archived":false}]`)
	tx := beginAs(t, db, practiceID, seedStaffID(t, db, "rls-update-doula"))

	result, err := tx.ExecContext(t.Context(),
		`UPDATE client_field_templates SET fields = '[]'::jsonb WHERE practice_id = $1`, practiceID,
	)
	if err != nil {
		t.Fatalf("update client_field_templates: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 rows affected by a Doula's UPDATE, got %d", rows)
	}
}

// TestRLS_AuditEventInsertRequiresOwnerOrAdminAndOwnActorID proves
// client_field_template_events_insert refuses a Doula's INSERT, and
// refuses an Owner naming someone else as the actor.
func TestRLS_AuditEventInsertRequiresOwnerOrAdminAndOwnActorID(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "rls-audit-owner")
	ownerID := seedStaffID(t, db, "rls-audit-owner")

	doulaPracticeID := seedDoula(t, db, "rls-audit-doula")
	doulaID := seedStaffID(t, db, "rls-audit-doula")

	txDoula := beginAs(t, db, doulaPracticeID, doulaID)
	if _, err := txDoula.ExecContext(t.Context(),
		`INSERT INTO client_field_template_events (practice_id, diff, actor_staff_id) VALUES ($1, '{}'::jsonb, $2)`,
		doulaPracticeID, doulaID,
	); err == nil {
		t.Fatal("expected a Doula's audit-event INSERT to be refused, got no error")
	}

	// A separate tx per assertion: Postgres aborts the whole transaction
	// on a failed INSERT, so the "names herself" success case below can't
	// share a tx with the "names someone else" failure above.
	txOwnerRefused := beginAs(t, db, practiceID, ownerID)
	if _, err := txOwnerRefused.ExecContext(t.Context(),
		`INSERT INTO client_field_template_events (practice_id, diff, actor_staff_id) VALUES ($1, '{}'::jsonb, $2)`,
		practiceID, doulaID,
	); err == nil {
		t.Fatal("expected an Owner naming a different staff member as actor to be refused, got no error")
	}

	txOwnerAdmitted := beginAs(t, db, practiceID, ownerID)
	if _, err := txOwnerAdmitted.ExecContext(t.Context(),
		`INSERT INTO client_field_template_events (practice_id, diff, actor_staff_id) VALUES ($1, '{}'::jsonb, $2)`,
		practiceID, ownerID,
	); err != nil {
		t.Fatalf("expected an Owner naming herself as actor to be admitted, got: %v", err)
	}
}
