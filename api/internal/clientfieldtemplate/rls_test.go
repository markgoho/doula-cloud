package clientfieldtemplate_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise 00050's client_field_templates_select/_insert/
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

// TestRLS_ActivityIsScopedToCurrentPractice proves the activity table
// (00051, ADR-0022) fences a Client Field Template's audit rows by
// Practice the same way every practice_id-carrying table is fenced.
//
// 00050's own client_field_template_events_insert additionally refused
// a Doula's INSERT and an Owner naming someone else as the actor -- a
// per-table INSERT-time role check that activity's single, shared
// practice-tier policy does not carry forward (ADR-0022 is one policy
// for every subject_kind, not a carve-out per predecessor table; see
// 00051_activity_log.sql's comment). That role check still runs where
// it can actually be enforced: PutHandler behind
// staffauth.RequireOwnerOrAdmin (TestPutHandler_DoulaForbidden), the
// same seam every other Staff-write endpoint in this repo relies on.
func TestRLS_ActivityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	mine := seedOwner(t, db, "rls-activity-mine")
	mineID := seedStaffID(t, db, "rls-activity-mine")
	theirs := seedOwner(t, db, "rls-activity-theirs")

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id)
		 VALUES ($1, 'client_field_template', $1, 'updated', '{}'::jsonb, 'staff', $2)`,
		mine, mineID,
	); err != nil {
		t.Fatalf("seed activity row: %v", err)
	}

	for _, tc := range []struct {
		name       string
		practiceID string
		want       int
	}{
		{"own practice", mine, 1},
		{"another practice", theirs, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := beginAs(t, db, tc.practiceID, mineID)
			var count int
			if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM activity`).Scan(&count); err != nil {
				t.Fatalf("count activity: %v", err)
			}
			if count != tc.want {
				t.Fatalf("visible activity rows = %d, want %d", count, tc.want)
			}
		})
	}
}
