package message_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policies added by
// 00009_messaging_client_portal_read.sql directly via db.App and
// set_config, bypassing any Go handler -- mirroring rls_test.go's style.
// They exist because listMessages' sender-name JOINs (00008_messaging.sql)
// need a Client-portal transaction to read a Staff sender's staff row and
// a Client's own clients row, which no pre-#59 policy allowed; see
// resolveSenderName in create.go and the migration's own comments.

// TestRLS_ClientsSelfVisibilityScopedToOwnRow proves clients_self_visibility
// lets a Client see their own clients row and no other Client's.
func TestRLS_ClientsSelfVisibilityScopedToOwnRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	clientA, _ := seedClientEngagement(t, db, practiceID, "Client A", "a@example.com")
	clientB, _ := seedClientEngagement(t, db, practiceID, "Client B", "b@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var name string
	if err := tx.QueryRowContext(t.Context(), `SELECT given_name FROM clients WHERE id = $1`, clientA).Scan(&name); err != nil {
		t.Fatalf("expected Client A to see their own clients row, got error: %v", err)
	}
	if name != "Client A" {
		t.Fatalf("name = %q, want %q", name, "Client A")
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM clients WHERE id = $1`, clientB).Scan(&count); err != nil {
		t.Fatalf("query clients as Client A: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected Client A to see zero rows for Client B, got count = %d", count)
	}
}

// TestRLS_StaffVisibleToOwnClientPortalEngagements proves
// staff_visible_to_own_client_portal_engagements: a Client can see the
// Staff members of their own Engagement's Practice, but not Staff at an
// unrelated Practice.
func TestRLS_StaffVisibleToOwnClientPortalEngagements(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	staffAtA := seedStaffAtPracticeNamed(t, db, practiceA, "staff-at-a", "Staff At A")
	clientA, _ := seedClientEngagement(t, db, practiceA, "Client A", "a@example.com")

	practiceB := seedPractice(t, db, "Practice B")
	seedStaffAtPracticeNamed(t, db, practiceB, "staff-at-b", "Staff At B")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var name string
	if err := tx.QueryRowContext(t.Context(), `SELECT name FROM staff WHERE id = $1`, staffAtA).Scan(&name); err != nil {
		t.Fatalf("expected Client A to see Staff at their own Practice, got error: %v", err)
	}
	if name != "Staff At A" {
		t.Fatalf("name = %q, want %q", name, "Staff At A")
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM staff WHERE identity_uid = 'staff-at-b'`).Scan(&count); err != nil {
		t.Fatalf("query staff as Client A: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected Client A to see zero rows for Staff at an unrelated Practice, got count = %d", count)
	}
}
