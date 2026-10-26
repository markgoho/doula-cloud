package idempotency_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise idempotency_keys_own_scope's Staff branch
// (00129_idempotency_keys_client_scope.sql, first written in 00027)
// directly via db.App and set_config, proving the SQL policy itself
// scopes access to the caller's own Practice and Staff member -- not just
// that the Go helper happens to agree with it. Its Client branch is held
// in rlsguardrail/idempotency_keys_test.go (#1924).

// seedStaffMember inserts a Staff row (no membership needed for these
// tests -- they set session vars directly rather than going through
// staffauth.Middleware) and returns its id.
func seedStaffMember(t *testing.T, db *testdb.DB, identityUID string) (staffID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, first_name, last_name, email, work_state) VALUES ($1, 'RLS', 'Test Staff', 'rls-staff@example.com', 'NY') RETURNING id`,
		identityUID,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	return staffID
}

// TestRLS_IdempotencyKeysFailsClosedWithNoSessionSet proves a row is
// invisible with no app.current_practice_id/app.current_staff_id set.
func TestRLS_IdempotencyKeysFailsClosedWithNoSessionSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "No Session Practice")
	staffID := seedStaffMember(t, db, "no-session-staff")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO idempotency_keys (key, practice_id, staff_id, status_code, response_body) VALUES ('k', $1, $2, 200, '{}')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed idempotency key: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM idempotency_keys`).Scan(&count); err != nil {
		t.Fatalf("query idempotency_keys: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 visible rows with no session vars set, got %d", count)
	}
}

// TestRLS_IdempotencyKeysDeniesOtherPracticeAndStaff proves the policy
// rejects a row belonging to a different Practice/Staff pair even when
// one of the two matches.
func TestRLS_IdempotencyKeysDeniesOtherPracticeAndStaff(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	staffA := seedStaffMember(t, db, "staff-a")
	staffB := seedStaffMember(t, db, "staff-b")

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO idempotency_keys (key, practice_id, staff_id, status_code, response_body) VALUES
		 ('k-a-a', $1, $2, 200, '{}'),
		 ('k-a-b', $1, $3, 200, '{}'),
		 ('k-b-a', $4, $2, 200, '{}')`,
		practiceA, staffA, staffB, practiceB,
	); err != nil {
		t.Fatalf("seed idempotency keys: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config practice: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_staff_id', $1, true)`, staffA); err != nil {
		t.Fatalf("set_config staff: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM idempotency_keys`).Scan(&count); err != nil {
		t.Fatalf("query idempotency_keys: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 visible row (own practice+staff), got %d", count)
	}

	var visibleKey string
	if err := tx.QueryRowContext(t.Context(), `SELECT key FROM idempotency_keys`).Scan(&visibleKey); err != nil {
		t.Fatalf("query key: %v", err)
	}
	if visibleKey != "k-a-a" {
		t.Fatalf("visible key = %q, want %q", visibleKey, "k-a-a")
	}
}

// TestRLS_IdempotencyKeysAllowsOwnPracticeAndStaff proves the door the Go
// helper relies on actually opens for the matching case.
func TestRLS_IdempotencyKeysAllowsOwnPracticeAndStaff(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Own Practice")
	staffID := seedStaffMember(t, db, "own-staff")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO idempotency_keys (key, practice_id, staff_id, status_code, response_body) VALUES ('own-key', $1, $2, 201, '{"ok":true}')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed idempotency key: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config practice: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_staff_id', $1, true)`, staffID); err != nil {
		t.Fatalf("set_config staff: %v", err)
	}

	var statusCode int
	if err := tx.QueryRowContext(t.Context(), `SELECT status_code FROM idempotency_keys WHERE key = 'own-key'`).Scan(&statusCode); err != nil {
		t.Fatalf("expected own-key to be visible: %v", err)
	}
	if statusCode != 201 {
		t.Fatalf("status_code = %d, want 201", statusCode)
	}
}

// TestSchema_IdempotencyKeyNamesExactlyOneCaller proves
// idempotency_keys_one_caller (00129, #1924): a key belongs to a Staff
// member or to a Client, never to both and never to neither -- checked
// through the superuser, so RLS plays no part.
func TestSchema_IdempotencyKeyNamesExactlyOneCaller(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "One Caller Practice")
	staffID := seedStaffMember(t, db, "one-caller-staff")
	clientID, _ := testdb.SeedNamedEngagement(t, db, practiceID, "One Caller Client", "one-caller@example.com")

	for _, tc := range []struct {
		name              string
		staffID, clientID any
	}{
		{"neither", nil, nil},
		{"both", staffID, clientID},
	} {
		_, err := db.Admin.ExecContext(t.Context(),
			`INSERT INTO idempotency_keys (key, practice_id, staff_id, client_id, status_code, response_body) VALUES ('k', $1, $2, $3, 200, '{}')`,
			practiceID, tc.staffID, tc.clientID,
		)
		if err == nil {
			t.Errorf("%s: a key naming %s caller was stored, want it refused", tc.name, tc.name)
		}
	}
}

// TestRLS_IdempotencyKeysInsertRejectedForOtherStaff proves the INSERT
// door (app_runtime has GRANT INSERT, gated by this same USING policy
// since no separate WITH CHECK was defined) rejects writing a row for a
// staff_id other than the caller's own.
func TestRLS_IdempotencyKeysInsertRejectedForOtherStaff(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Insert Practice")
	staffID := seedStaffMember(t, db, "insert-staff")
	otherStaffID := seedStaffMember(t, db, "other-insert-staff")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config practice: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_staff_id', $1, true)`, staffID); err != nil {
		t.Fatalf("set_config staff: %v", err)
	}

	_, err = tx.ExecContext(t.Context(),
		`INSERT INTO idempotency_keys (key, practice_id, staff_id, status_code, response_body) VALUES ('hijack', $1, $2, 200, '{}')`,
		practiceID, otherStaffID,
	)
	if err == nil {
		t.Fatal("expected insert for a different staff_id to be rejected by RLS")
	}
}
