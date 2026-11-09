package clientfieldtemplate_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise 00050's client_field_templates_select/_insert/
// _update policies directly via db.App and set_config, bypassing the Go
// handlers -- proving AC1's "at both seams" holds at the database seam,
// not only PutHandler's own RequireOwnerOrAdmin check.

// TestRLS_SelectFailsClosedWithNoSessionVarSet proves client_field_templates
// denies all rows when app.current_practice_id is never set.
func TestRLS_SelectFailsClosedWithNoSessionVarSet(t *testing.T) {
	e := newEnv(t, fixture.Owner())
	e.seedIntakeNote()

	testdb.RequireFailsClosed(t, e.DB, "client_field_templates")
}

// TestRLS_SelectScopedToCurrentPracticeAndAnyRole proves a Doula's
// session (any Staff role, per the sibling GET handler) reads its own
// Practice's row and not another Practice's.
func TestRLS_SelectScopedToCurrentPracticeAndAnyRole(t *testing.T) {
	e := newEnv(t)
	e.seedIntakeNote()
	other := fixture.Practice(t, e.DB, fixture.PracticeName("Practice B"))
	e.Exec(`INSERT INTO client_field_templates (practice_id, fields) VALUES ($1, $2)`, other.ID, intakeNoteField)

	tx := e.DB.BeginAs(t, testdb.Practice(e.Practice.ID), testdb.Staff(e.Staff.ID))

	if visible := testdb.Column[string](t, tx, `SELECT practice_id FROM client_field_templates`); !slices.Equal(visible, []string{e.Practice.ID}) {
		t.Fatalf("visible practice_ids = %v, want only %q", visible, e.Practice.ID)
	}
}

// TestRLS_InsertIsOwnerOrAdminOnly proves client_field_templates_insert
// refuses a Doula's INSERT independent of PutHandler's own Go-level check,
// admits an Owner (so the refusal is the role gate working, not a blanket
// one), and admits an Admin (RequireOwnerOrAdmin's widened half, held in
// RLS too, not only in Go).
func TestRLS_InsertIsOwnerOrAdminOnly(t *testing.T) {
	cases := []struct {
		name    string
		staff   fixture.StaffOption
		allowed bool
	}{
		{"a Doula is refused", fixture.Doula(), false},
		{"an Owner is admitted", fixture.Owner(), true},
		{"an Admin is admitted", fixture.Admin(), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, tc.staff)
			tx := e.DB.BeginAs(t, testdb.Practice(e.Practice.ID), testdb.Staff(e.Staff.ID))

			_, err := tx.ExecContext(t.Context(),
				`INSERT INTO client_field_templates (practice_id, fields) VALUES ($1, '[]'::jsonb)`, e.Practice.ID)

			if tc.allowed && err != nil {
				t.Fatalf("expected the INSERT to be admitted, got: %v", err)
			}
			if !tc.allowed && err == nil {
				t.Fatal("expected client_field_templates_insert to refuse the INSERT, got no error")
			}
		})
	}
}

// TestRLS_DoulaUpdateAffectsZeroRows proves client_field_templates_update
// refuses a Doula's UPDATE -- Gap 1 from review: 00042's original policy
// was a bare ALL-commands, any-role grant, and this is the regression
// test pinning the fix.
func TestRLS_DoulaUpdateAffectsZeroRows(t *testing.T) {
	e := newEnv(t)
	e.seedIntakeNote()
	tx := e.DB.BeginAs(t, testdb.Practice(e.Practice.ID), testdb.Staff(e.Staff.ID))

	if rows := testdb.RowsAffected(t, tx, `UPDATE client_field_templates SET fields = '[]'::jsonb WHERE practice_id = $1`, e.Practice.ID); rows != 0 {
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
// staffauth.RequireOwnerOrAdmin (TestPutHandler_RoleAndBodyBoundaries),
// the same seam every other Staff-write endpoint in this repo relies on.
func TestRLS_ActivityIsScopedToCurrentPractice(t *testing.T) {
	e := newEnv(t, fixture.Owner())
	theirs := fixture.Practice(t, e.DB, fixture.PracticeName("Practice B"))
	e.Exec(`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id)
		 VALUES ($1, 'client_field_template', $1, 'updated', '{}'::jsonb, 'staff', $2)`,
		e.Practice.ID, e.Staff.ID)

	for _, tc := range []struct {
		name       string
		practiceID string
		want       int
	}{
		{"own practice", e.Practice.ID, 1},
		{"another practice", theirs.ID, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := e.DB.BeginAs(t, testdb.Practice(tc.practiceID), testdb.Staff(e.Staff.ID))

			if count := testdb.Scalar[int](t, tx, `SELECT count(*) FROM activity`); count != tc.want {
				t.Fatalf("visible activity rows = %d, want %d", count, tc.want)
			}
		})
	}
}
