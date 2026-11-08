package message_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
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
	p := fixture.Practice(t, db, fixture.PracticeName("Practice"))
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client A"), fixture.ClientEmail("a@example.com")))
	engagement2 := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client B"), fixture.ClientEmail("b@example.com")))

	tx := db.BeginAs(t, testdb.Client(engagement.Client.ID))

	name := testdb.Scalar[string](t, tx, `SELECT given_name FROM clients WHERE id = $1`, engagement.Client.ID)
	if name != "Client A" {
		t.Fatalf("name = %q, want %q", name, "Client A")
	}

	count := testdb.Scalar[int](t, tx, `SELECT count(*) FROM clients WHERE id = $1`, engagement2.Client.ID)
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
	practiceA := fixture.Practice(t, db, fixture.PracticeName("Practice A"))
	staffAtA := fixture.Staff(t, db, practiceA, fixture.StaffName("Staff At A"))
	engagement := fixture.Engagement(t, db, practiceA, fixture.NewClient(fixture.ClientName("Client A"), fixture.ClientEmail("a@example.com")))

	practiceB := fixture.Practice(t, db, fixture.PracticeName("Practice B"))
	fixture.Staff(t, db, practiceB, fixture.StaffName("Staff At B"))

	tx := db.BeginAs(t, testdb.Client(engagement.Client.ID))

	name := testdb.Scalar[string](t, tx, `SELECT name FROM staff WHERE id = $1`, staffAtA.ID)
	if name != "Staff At A" {
		t.Fatalf("name = %q, want %q", name, "Staff At A")
	}

	count := testdb.Scalar[int](t, tx, `SELECT count(*) FROM staff WHERE identity_uid = 'staff-at-b'`)
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
	p := fixture.Practice(t, db, fixture.PracticeName("History Practice"))
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Nadia Client"), fixture.ClientEmail("nadia@example.com")))

	visited := fixture.Staff(t, db, p, fixture.StaffName("Maya Okonkwo"))
	messaged := fixture.Staff(t, db, p, fixture.StaffName("Priya Raman"))
	stranger := fixture.Staff(t, db, p, fixture.StaffName("Never Met"))

	// The case the reach has to refuse and a Practice-wide one would not:
	// a Doula who did the same work, at the same Practice, for somebody
	// else. Both halves of the reach get one, so neither EXISTS can be
	// widened to the Practice without this failing.
	othersEngagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Another Client"), fixture.ClientEmail("another@example.com")))
	visitedOther := fixture.Staff(t, db, p, fixture.StaffName("Somebody Else's Doula"))
	messagedOther := fixture.Staff(t, db, p, fixture.StaffName("Somebody Else's Admin"))

	seedVisit := func(engagement, staffID string) {
		t.Helper()
		if _, err := db.Admin.ExecContext(t.Context(),
			`INSERT INTO visits (engagement_id, staff_id, scheduled_at) VALUES ($1, $2, now() - interval '3 days')`,
			engagement, staffID,
		); err != nil {
			t.Fatalf("seed visit: %v", err)
		}
	}
	seedStaffMessage := func(engagement, staffID string) {
		t.Helper()
		if _, err := db.Admin.ExecContext(t.Context(),
			`INSERT INTO messages (engagement_id, sender_type, sender_id, body) VALUES ($1, 'staff', $2, 'Checking in.')`,
			engagement, staffID,
		); err != nil {
			t.Fatalf("seed message: %v", err)
		}
	}
	seedVisit(engagement.ID, visited.ID)
	seedStaffMessage(engagement.ID, messaged.ID)
	seedVisit(othersEngagement.ID, visitedOther.ID)
	seedStaffMessage(othersEngagement.ID, messagedOther.ID)
	// Every one of them leaves, so nothing below is answered by 00009.
	if _, err := db.Admin.ExecContext(t.Context(),
		`DELETE FROM practice_memberships WHERE practice_id = $1`, p.ID,
	); err != nil {
		t.Fatalf("remove memberships: %v", err)
	}

	tx := db.BeginAs(t, testdb.Client(engagement.Client.ID))

	for _, want := range []struct {
		staffID string
		name    string
	}{{visited.ID, "Maya Okonkwo"}, {messaged.ID, "Priya Raman"}} {
		name := testdb.Scalar[string](t, tx, `SELECT name FROM staff WHERE id = $1`, want.staffID)
		if name != want.name {
			t.Fatalf("name = %q, want %q", name, want.name)
		}
	}

	for _, refused := range []struct {
		staffID string
		why     string
	}{
		{stranger.ID, "never worked with anyone"},
		{visitedOther.ID, "worked another Client's Visit at the same Practice"},
		{messagedOther.ID, "sent a Message on another Client's Engagement at the same Practice"},
	} {
		count := testdb.Scalar[int](t, tx, `SELECT count(*) FROM staff WHERE id = $1`, refused.staffID)
		if count != 0 {
			t.Fatalf("expected zero rows for a departed Staff member who %s, got count = %d", refused.why, count)
		}
	}
}

// TestRLS_StaffPortalHistoryGrantsAStaffSessionNothing is the other side
// of 00111's guard: app.current_client_id is a Client-portal variable
// that staffauth.Middleware never sets, so a Staff transaction reads the
// staff table exactly as it did before -- through its own Membership and
// nothing else.
func TestRLS_StaffPortalHistoryGrantsAStaffSessionNothing(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Staff Session Practice"))
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Nadia Client"), fixture.ClientEmail("nadia2@example.com")))
	departed := fixture.Staff(t, db, p, fixture.StaffName("Maya Okonkwo"))
	reader := fixture.Staff(t, db, p, fixture.StaffName("Renata Owner"), fixture.Owner())

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id, scheduled_at) VALUES ($1, $2, now() - interval '3 days')`,
		engagement.ID, departed.ID,
	); err != nil {
		t.Fatalf("seed visit: %v", err)
	}
	testdb.RemoveMembership(t, db, departed.ID)

	tx := db.BeginAs(t, testdb.Practice(p.ID))

	count := testdb.Scalar[int](t, tx, `SELECT count(*) FROM staff WHERE id = $1`, departed.ID)
	if count != 0 {
		t.Fatalf("expected a Staff session to see zero rows for a departed.ID Staff member, got count = %d", count)
	}

	name := testdb.Scalar[string](t, tx, `SELECT name FROM staff WHERE id = $1`, reader.ID)
	if name != "Renata Owner" {
		t.Fatalf("name = %q, want %q", name, "Renata Owner")
	}
}
