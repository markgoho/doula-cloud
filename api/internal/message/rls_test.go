package message_test

import (
	"testing"

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
	practiceID := seedPractice(t, db, "Fail Closed Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, "fail-closed-staff")
	_, engagementID := seedClientEngagement(t, db, practiceID, "Some Client", "some@example.com")
	seedMessage(t, db, engagementID, "staff", staffID, "hello")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM messages`).Scan(&count); err != nil {
		t.Fatalf("query messages with no session vars set: %v", err)
	}
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
	practiceA := seedPractice(t, db, "Practice A")
	staffA := seedStaffAtPractice(t, db, practiceA, "staff-a")
	_, engagementA := seedClientEngagement(t, db, practiceA, "Client A", "a@example.com")
	seedMessage(t, db, engagementA, "staff", staffA, "message at A")

	practiceB := seedPractice(t, db, "Practice B")
	seedStaffAtPractice(t, db, practiceB, "staff-b")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM messages`).Scan(&count); err != nil {
		t.Fatalf("query messages as Practice A: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected Practice A to see its own message, got count = %d", count)
	}

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceB); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM messages`).Scan(&count); err != nil {
		t.Fatalf("query messages as Practice B: %v", err)
	}
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
	practiceID := seedPractice(t, db, "Practice")
	clientA, engagementA := seedClientEngagement(t, db, practiceID, "Client A", "a@example.com")
	seedMessage(t, db, engagementA, "client", clientA, "message from client A")

	clientB, _ := seedClientEngagement(t, db, practiceID, "Client B", "b@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientA); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM messages`).Scan(&count); err != nil {
		t.Fatalf("query messages as Client A: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected Client A to see its own Engagement's message, got count = %d", count)
	}

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientB); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM messages`).Scan(&count); err != nil {
		t.Fatalf("query messages as Client B: %v", err)
	}
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
	practiceID := seedPractice(t, db, "Insert Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, "insert-staff")
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, body) VALUES ($1, 'staff', $2, 'hello from staff')`,
		engagementID, staffID,
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
	practiceID := seedPractice(t, db, "Insert Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, body) VALUES ($1, 'client', $2, 'hello from client')`,
		engagementID, clientID,
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
	practiceID := seedPractice(t, db, "Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, "staff-empty-msg")
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	_, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id) VALUES ($1, 'staff', $2)`,
		engagementID, staffID,
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
	practiceID := seedPractice(t, db, "Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, "staff-partial-attachment")
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	_, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, attachment_object_path)
		 VALUES ($1, 'staff', $2, 'objects/some-file.png')`,
		engagementID, staffID,
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
	practiceID := seedPractice(t, db, "Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, "fail-closed-push-staff")
	seedPushSubscription(t, db, "staff", staffID, "https://push.example.com/fail-closed")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM push_subscriptions`).Scan(&count); err != nil {
		t.Fatalf("query push_subscriptions with no session vars set: %v", err)
	}
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
	practiceA := seedPractice(t, db, "Practice A")
	staffA := seedStaffAtPractice(t, db, practiceA, "push-staff-a")
	subA := seedPushSubscription(t, db, "staff", staffA, "https://push.example.com/staff-a")

	practiceB := seedPractice(t, db, "Practice B")
	staffB := seedStaffAtPractice(t, db, practiceB, "push-staff-b")
	seedPushSubscription(t, db, "staff", staffB, "https://push.example.com/staff-b")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, "push-staff-a"); err != nil {
		t.Fatalf("set_config identity: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config practice: %v", err)
	}

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
	practiceID := seedPractice(t, db, "Practice")
	clientA, _ := seedClientEngagement(t, db, practiceID, "Client A", "a@example.com")
	subA := seedPushSubscription(t, db, "client", clientA, "https://push.example.com/client-a")

	clientB, _ := seedClientEngagement(t, db, practiceID, "Client B", "b@example.com")
	seedPushSubscription(t, db, "client", clientB, "https://push.example.com/client-b")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

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
	practiceID := seedPractice(t, db, "Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, "insert-push-staff")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, "insert-push-staff"); err != nil {
		t.Fatalf("set_config identity: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config practice: %v", err)
	}

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO push_subscriptions (owner_type, owner_id, endpoint, p256dh_key, auth_key)
		 VALUES ('staff', $1, 'https://push.example.com/insert-staff', 'p256dh-key', 'auth-key')`,
		staffID,
	); err != nil {
		t.Fatalf("expected Staff INSERT to be allowed for their own identity, got error: %v", err)
	}
}

// TestRLS_PushSubscriptionsClientTierAllowsInsert mirrors
// TestRLS_PushSubscriptionsStaffTierAllowsInsert for the Client-portal
// population.
func TestRLS_PushSubscriptionsClientTierAllowsInsert(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	clientID, _ := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO push_subscriptions (owner_type, owner_id, endpoint, p256dh_key, auth_key)
		 VALUES ('client', $1, 'https://push.example.com/insert-client', 'p256dh-key', 'auth-key')`,
		clientID,
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

	practiceID := seedPractice(t, db, "Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, sharedUID)
	seedPushSubscription(t, db, "staff", staffID, "https://push.example.com/shared-staff")

	clientID, _ := seedClientEngagement(t, db, practiceID, "Shared Client", "shared@example.com")
	seedPortalUser(t, db, sharedUID, clientID)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, sharedUID); err != nil {
		t.Fatalf("set_config identity: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config client: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM push_subscriptions WHERE owner_type = 'staff'`).Scan(&count); err != nil {
		t.Fatalf("query push_subscriptions during client-portal context: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected the shared identity's Staff push_subscriptions row to stay hidden during a Client-portal context, got count = %d", count)
	}
}
