package message_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policies from 00008_messaging.sql
// directly via db.App and set_config, bypassing any Go handler -- this
// ticket adds only the schema and RLS, not the handlers that will run
// against it (a future ticket).

// TestRLS_MessagesFailsClosedWithNoSessionVarsSet proves messages denies
// all rows when no session variable is set, even though a matching row
// genuinely exists.
func TestRLS_MessagesFailsClosedWithNoSessionVarsSet(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Fail Closed Practice"))
	staff := fixture.Staff(t, db, p)
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Some Client"), fixture.ClientEmail("some@example.com")))
	seedMessage(t, db, engagement.ID, "staff", staff.ID, "hello")

	count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM messages`)
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_MessagesPracticeTierScopedToOwnPractice proves
// messages_practice_visibility narrows to messages on Engagements at
// app.current_practice_id -- and that a Staff member at a different
// Practice gets zero rows, per the ticket's acceptance criteria.
func TestRLS_MessagesPracticeTierScopedToOwnPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := fixture.Practice(t, db, fixture.PracticeName("Practice A"))
	staffA := fixture.Staff(t, db, practiceA)
	engagementA := fixture.Engagement(t, db, practiceA, fixture.NewClient(fixture.ClientName("Client A"), fixture.ClientEmail("a@example.com")))
	seedMessage(t, db, engagementA.ID, "staff", staffA.ID, "message at A")

	practiceB := fixture.Practice(t, db, fixture.PracticeName("Practice B"))
	fixture.Staff(t, db, practiceB)

	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))
	count := testdb.Scalar[int](t, tx, `SELECT count(*) FROM messages`)
	if count != 1 {
		t.Fatalf("expected Practice A to see its own message, got count = %d", count)
	}

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceB.ID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	count = testdb.Scalar[int](t, tx, `SELECT count(*) FROM messages`)
	if count != 0 {
		t.Fatalf("expected a Staff member at a different Practice to get zero rows, got count = %d", count)
	}
}

// TestRLS_MessagesClientTierScopedToOwnEngagement proves
// messages_client_visibility narrows to messages on the Client's own
// Engagement -- and that a Client not linked to the Engagement gets zero
// rows, per the ticket's acceptance criteria.
func TestRLS_MessagesClientTierScopedToOwnEngagement(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Practice"))
	engagementA := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client A"), fixture.ClientEmail("a@example.com")))
	seedMessage(t, db, engagementA.ID, "client", engagementA.Client.ID, "message from client A")

	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client B"), fixture.ClientEmail("b@example.com")))

	tx := db.BeginAs(t, testdb.Client(engagementA.Client.ID))
	count := testdb.Scalar[int](t, tx, `SELECT count(*) FROM messages`)
	if count != 1 {
		t.Fatalf("expected Client A to see its own Engagement's message, got count = %d", count)
	}

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, engagement.Client.ID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	count = testdb.Scalar[int](t, tx, `SELECT count(*) FROM messages`)
	if count != 0 {
		t.Fatalf("expected a Client not linked to the Engagement to get zero rows, got count = %d", count)
	}
}

// TestRLS_MessagesPracticeTierAllowsStaffInsert proves
// messages_practice_visibility is an ALL-command policy, not
// SELECT-only: a Staff member with a Practice context set can INSERT a
// Message through the low-privilege db.App connection, the same
// connection RLS actually applies to.
func TestRLS_MessagesPracticeTierAllowsStaffInsert(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Insert Practice"))
	staff := fixture.Staff(t, db, p)
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client"), fixture.ClientEmail("client@example.com")))

	tx := db.BeginAs(t, testdb.Practice(p.ID))

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, body) VALUES ($1, 'staff', $2, 'hello from staff')`,
		engagement.ID, staff.ID,
	); err != nil {
		t.Fatalf("expected Staff INSERT to be allowed with a Practice context set, got error: %v", err)
	}
}

// TestRLS_MessagesClientTierAllowsClientInsert mirrors
// TestRLS_MessagesPracticeTierAllowsStaffInsert for the Client-portal
// population: a Client with their own Engagement's client context set
// can INSERT a Message through db.App.
func TestRLS_MessagesClientTierAllowsClientInsert(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Insert Practice"))
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client"), fixture.ClientEmail("client@example.com")))

	tx := db.BeginAs(t, testdb.Client(engagement.Client.ID))

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, body) VALUES ($1, 'client', $2, 'hello from client')`,
		engagement.ID, engagement.Client.ID,
	); err != nil {
		t.Fatalf("expected Client INSERT to be allowed with a Client context set, got error: %v", err)
	}
}

// TestMessages_HasContentConstraintRejectsEmptyMessage proves
// messages_has_content: a row with neither a body nor an attachment is
// rejected at the schema level. Not an RLS test -- it runs through
// db.Admin (which bypasses RLS) to isolate the CHECK constraint from the
// policies under test elsewhere in this file.
func TestMessages_HasContentConstraintRejectsEmptyMessage(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Practice"))
	staff := fixture.Staff(t, db, p)
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client"), fixture.ClientEmail("client@example.com")))

	_, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id) VALUES ($1, 'staff', $2)`,
		engagement.ID, staff.ID,
	)
	if err == nil {
		t.Fatal("expected INSERT with neither body nor attachment to be rejected, got no error")
	}
}

// TestMessages_AttachmentColumnsMustAllBeSetTogether proves
// messages_attachment_all_or_nothing: a row with only some attachment
// columns set is rejected at the schema level. Not an RLS test -- see
// TestMessages_HasContentConstraintRejectsEmptyMessage above.
func TestMessages_AttachmentColumnsMustAllBeSetTogether(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Practice"))
	staff := fixture.Staff(t, db, p)
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client"), fixture.ClientEmail("client@example.com")))

	_, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, attachment_object_path)
		 VALUES ($1, 'staff', $2, 'objects/some-file.png')`,
		engagement.ID, staff.ID,
	)
	if err == nil {
		t.Fatal("expected INSERT with only attachment_object_path set to be rejected, got no error")
	}
}

// TestRLS_PushSubscriptionsFailsClosedWithNoSessionVarsSet proves
// push_subscriptions denies all rows when no session variable is set,
// even though a matching row genuinely exists.
func TestRLS_PushSubscriptionsFailsClosedWithNoSessionVarsSet(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Practice"))
	staff := fixture.Staff(t, db, p)
	testdb.SeedPushSubscription(t, db, "staff", staff.ID, "https://push.example.com/fail-closed")

	count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM push_subscriptions`)
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_PushSubscriptionsStaffScopedToOwnIdentity proves
// push_subscriptions_staff_visibility narrows to the calling Staff
// member's own subscription row specifically (not just a matching row
// count), across Practices -- staffB is seeded at a different Practice
// than staffA, since push_subscriptions is identity-scoped, not
// Practice-scoped, and staffA's own current_practice_id shouldn't matter
// to which subscription is visible.
func TestRLS_PushSubscriptionsStaffScopedToOwnIdentity(t *testing.T) {
	db := testdb.New(t)
	practiceA := fixture.Practice(t, db, fixture.PracticeName("Practice A"))
	staffA := fixture.Staff(t, db, practiceA)
	subA := testdb.SeedPushSubscription(t, db, "staff", staffA.ID, "https://push.example.com/staff-a")

	practiceB := fixture.Practice(t, db, fixture.PracticeName("Practice B"))
	staffB := fixture.Staff(t, db, practiceB)
	testdb.SeedPushSubscription(t, db, "staff", staffB.ID, "https://push.example.com/staff-b")

	tx := db.BeginAs(t, testdb.Identity(staffA.UID), testdb.Practice(practiceA.ID))

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM push_subscriptions`)
	if err != nil {
		t.Fatalf("query push_subscriptions as staff A: %v", err)
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

	if len(visibleIDs) != 1 || visibleIDs[0] != subA {
		t.Fatalf("visible push_subscriptions = %v, want only %q", visibleIDs, subA)
	}
}

// TestRLS_PushSubscriptionsClientScopedToOwnIdentity proves
// push_subscriptions_client_visibility narrows to the calling Client's
// own subscription row specifically, never another Client's.
func TestRLS_PushSubscriptionsClientScopedToOwnIdentity(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Practice"))
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client A"), fixture.ClientEmail("a@example.com")))
	subA := testdb.SeedPushSubscription(t, db, "client", engagement.Client.ID, "https://push.example.com/client-a")

	engagement2 := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client B"), fixture.ClientEmail("b@example.com")))
	testdb.SeedPushSubscription(t, db, "client", engagement2.Client.ID, "https://push.example.com/client-b")

	tx := db.BeginAs(t, testdb.Client(engagement.Client.ID))

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM push_subscriptions`)
	if err != nil {
		t.Fatalf("query push_subscriptions as client A: %v", err)
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

	if len(visibleIDs) != 1 || visibleIDs[0] != subA {
		t.Fatalf("visible push_subscriptions = %v, want only %q", visibleIDs, subA)
	}
}

// TestRLS_PushSubscriptionsStaffTierAllowsInsert proves
// push_subscriptions_staff_visibility is an ALL-command policy: a Staff
// member can register their own subscription (owner_id = their own
// current_staff_id()) through db.App.
func TestRLS_PushSubscriptionsStaffTierAllowsInsert(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Practice"))
	staff := fixture.Staff(t, db, p)

	tx := db.BeginAs(t, testdb.Identity(staff.UID), testdb.Practice(p.ID))

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO push_subscriptions (owner_type, owner_id, endpoint, p256dh_key, auth_key)
		 VALUES ('staff', $1, 'https://push.example.com/insert-staff', 'p256dh-key', 'auth-key')`,
		staff.ID,
	); err != nil {
		t.Fatalf("expected Staff INSERT to be allowed for their own identity, got error: %v", err)
	}
}

// TestRLS_PushSubscriptionsClientTierAllowsInsert mirrors
// TestRLS_PushSubscriptionsStaffTierAllowsInsert for the Client-portal
// population.
func TestRLS_PushSubscriptionsClientTierAllowsInsert(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Practice"))
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client"), fixture.ClientEmail("client@example.com")))

	tx := db.BeginAs(t, testdb.Client(engagement.Client.ID))

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO push_subscriptions (owner_type, owner_id, endpoint, p256dh_key, auth_key)
		 VALUES ('client', $1, 'https://push.example.com/insert-client', 'p256dh-key', 'auth-key')`,
		engagement.Client.ID,
	); err != nil {
		t.Fatalf("expected Client INSERT to be allowed for their own identity, got error: %v", err)
	}
}

// TestRLS_PushSubscriptionsStaffRowHiddenDuringClientPortalContext proves
// the app.current_client_id guard on push_subscriptions_staff_visibility:
// a person whose identity_uid happens to match both a staff row and a
// client_portal_users row must not have their Staff push_subscriptions
// row leak into a Client-portal-scoped transaction, mirroring
// clientauth's TestRLS_StaffSelfVisibilityHiddenDuringClientPortalContext
// for the same shared-identity scenario.
func TestRLS_PushSubscriptionsStaffRowHiddenDuringClientPortalContext(t *testing.T) {
	db := testdb.New(t)
	const sharedUID = "shared-push-identity"

	p := fixture.Practice(t, db, fixture.PracticeName("Practice"))
	staff := fixture.Staff(t, db, p, fixture.StaffUID(sharedUID))
	testdb.SeedPushSubscription(t, db, "staff", staff.ID, "https://push.example.com/shared-staff")

	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Shared Client"), fixture.ClientEmail("shared@example.com")))
	testdb.SeedPortalUser(t, db, sharedUID, engagement.Client.ID)

	tx := db.BeginAs(t, testdb.Identity(sharedUID), testdb.Client(engagement.Client.ID))

	count := testdb.Scalar[int](t, tx, `SELECT count(*) FROM push_subscriptions WHERE owner_type = 'staff'`)
	if count != 0 {
		t.Fatalf("expected the shared identity's Staff push_subscriptions row to stay hidden during a Client-portal context, got count = %d", count)
	}
}
