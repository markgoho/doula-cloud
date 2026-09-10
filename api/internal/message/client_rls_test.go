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
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientA, _ := testdb.SeedNamedEngagement(t, db, practiceID, "Client A", "a@example.com")
	clientB, _ := testdb.SeedNamedEngagement(t, db, practiceID, "Client B", "b@example.com")

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
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	staffAtA := testdb.SeedNamedStaffAtPractice(t, db, practiceA, "staff-at-a", "Staff At A", []string{doulaRole}, "employee")
	clientA, _ := testdb.SeedNamedEngagement(t, db, practiceA, "Client A", "a@example.com")

	practiceB := testdb.SeedPractice(t, db, "Practice B")
	testdb.SeedNamedStaffAtPractice(t, db, practiceB, "staff-at-b", "Staff At B", []string{doulaRole}, "employee")

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

// TestRLS_StaffVisibleToOwnClientPortalHistory pins 00111
// (staff_visible_to_own_client_portal_history, #1077): once a Doula's
// Membership is gone, 00009's reach is gone with it, and what she worked
// on this Client's own Engagement is what keeps her name readable. Both
// halves of the reach are exercised -- a Visit and a Message -- because
// the Client portal reads a Staff name on both screens.
func TestRLS_StaffVisibleToOwnClientPortalHistory(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "History Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Nadia Client", "nadia@example.com")

	visited := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "history-visited", "Maya Okonkwo", []string{doulaRole}, "employee")
	messaged := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "history-messaged", "Priya Raman", []string{doulaRole}, "employee")
	stranger := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "history-stranger", "Never Met", []string{doulaRole}, "employee")

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id, scheduled_at) VALUES ($1, $2, now() - interval '3 days')`,
		engagementID, visited,
	); err != nil {
		t.Fatalf("seed visit: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, body) VALUES ($1, 'staff', $2, 'Checking in.')`,
		engagementID, messaged,
	); err != nil {
		t.Fatalf("seed message: %v", err)
	}
	// Every one of them leaves, so nothing below is answered by 00009.
	if _, err := db.Admin.ExecContext(t.Context(),
		`DELETE FROM practice_memberships WHERE practice_id = $1`, practiceID,
	); err != nil {
		t.Fatalf("remove memberships: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	for _, want := range []struct {
		staffID string
		name    string
	}{{visited, "Maya Okonkwo"}, {messaged, "Priya Raman"}} {
		var name string
		if err := tx.QueryRowContext(t.Context(), `SELECT name FROM staff WHERE id = $1`, want.staffID).Scan(&name); err != nil {
			t.Fatalf("expected the departed Staff member %q to stay readable to her own Client, got error: %v", want.name, err)
		}
		if name != want.name {
			t.Fatalf("name = %q, want %q", name, want.name)
		}
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM staff WHERE id = $1`, stranger).Scan(&count); err != nil {
		t.Fatalf("query staff as the Client: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero rows for a departed Staff member who never worked with this Client, got count = %d", count)
	}
}

// TestRLS_StaffPortalHistoryGrantsAStaffSessionNothing is the other side
// of 00111's guard: app.current_client_id is a Client-portal variable
// that staffauth.Middleware never sets, so a Staff transaction reads the
// staff table exactly as it did before -- through its own Membership and
// nothing else.
func TestRLS_StaffPortalHistoryGrantsAStaffSessionNothing(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Staff Session Practice")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Nadia Client", "nadia2@example.com")
	departed := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "session-departed", "Maya Okonkwo", []string{doulaRole}, "employee")
	reader := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "session-reader", "Renata Owner", []string{"owner"}, "employee")

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id, scheduled_at) VALUES ($1, $2, now() - interval '3 days')`,
		engagementID, departed,
	); err != nil {
		t.Fatalf("seed visit: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`DELETE FROM practice_memberships WHERE staff_id = $1`, departed,
	); err != nil {
		t.Fatalf("remove membership: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM staff WHERE id = $1`, departed).Scan(&count); err != nil {
		t.Fatalf("query staff as Staff: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected a Staff session to see zero rows for a departed Staff member, got count = %d", count)
	}

	var name string
	if err := tx.QueryRowContext(t.Context(), `SELECT name FROM staff WHERE id = $1`, reader).Scan(&name); err != nil {
		t.Fatalf("expected a Staff session to still read a live Membership's row, got error: %v", err)
	}
	if name != "Renata Owner" {
		t.Fatalf("name = %q, want %q", name, "Renata Owner")
	}
}
