package portalinvite_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policies from
// 00026_client_portal_provisioning.sql directly via db.App and set_config,
// bypassing the Go handlers -- proving the SQL policies themselves scope
// access correctly, not just that the handlers happen to agree with them.

// TestRLS_ClientPortalUsersInviteInsertFailsClosedWithNoPracticeSet proves
// a pending-row INSERT is rejected with no app.current_practice_id set.
func TestRLS_ClientPortalUsersInviteInsertFailsClosedWithNoPracticeSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "No Session Practice")
	clientID, _ := seedClientEngagement(t, db, practiceID, "No Session Client", "nosession@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid())`,
		clientID,
	)
	if err == nil {
		t.Fatal("expected insert to be rejected by RLS with no practice context set")
	}
}

// TestRLS_ClientPortalUsersInviteInsertFailsForClientAtOtherPractice
// proves the EXISTS check actually scopes to the caller's own Practice,
// not any Practice.
func TestRLS_ClientPortalUsersInviteInsertFailsForClientAtOtherPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	clientB, _ := seedClientEngagement(t, db, practiceB, "Client B", "clientb@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	_, err = tx.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid())`,
		clientB,
	)
	if err == nil {
		t.Fatal("expected insert to be rejected -- clientB has no Engagement at practiceA")
	}
}

// TestRLS_ClientPortalUsersInviteInsertSucceedsForClientAtOwnPractice
// proves the door invite handler needs actually opens for the matching
// case.
func TestRLS_ClientPortalUsersInviteInsertSucceedsForClientAtOwnPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Own Practice")
	clientID, _ := seedClientEngagement(t, db, practiceID, "Own Client", "own@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid())`,
		clientID,
	); err != nil {
		t.Fatalf("expected insert to succeed for a Client with an Engagement at the current Practice: %v", err)
	}
}

// TestRLS_ClientPortalUsersPracticeVisibilityScopedToOwnPractice proves the
// Staff SELECT policy only surfaces rows for Clients with an Engagement at
// the caller's current Practice -- required so InviteHandler's
// duplicate-invite detection can't see across Practices.
func TestRLS_ClientPortalUsersPracticeVisibilityScopedToOwnPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Visibility Practice A")
	practiceB := seedPractice(t, db, "Visibility Practice B")
	clientA, _ := seedClientEngagement(t, db, practiceA, "Client A", "cliana@example.com")
	clientB, _ := seedClientEngagement(t, db, practiceB, "Client B", "clianb@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid()), ($2, gen_random_uuid())`,
		clientA, clientB,
	); err != nil {
		t.Fatalf("seed portal rows: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM client_portal_users`).Scan(&count); err != nil {
		t.Fatalf("query client_portal_users: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 visible row (Practice A's own), got %d", count)
	}

	var visibleClientID string
	if err := tx.QueryRowContext(t.Context(), `SELECT client_id FROM client_portal_users`).Scan(&visibleClientID); err != nil {
		t.Fatalf("query client_id: %v", err)
	}
	if visibleClientID != clientA {
		t.Fatalf("visible row belongs to client %q, want %q", visibleClientID, clientA)
	}
}

// TestRLS_ClientPortalUsersInviteUpdateCannotTouchAcceptedRow proves the
// rotation door (client_portal_users_invite_update) only ever admits
// pending rows -- a Staff caller cannot use it to un-accept or hijack an
// already-claimed row.
func TestRLS_ClientPortalUsersInviteUpdateCannotTouchAcceptedRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Accepted Row Practice")
	clientID, _ := seedClientEngagement(t, db, practiceID, "Accepted Row Client", "acceptedrow@example.com")
	testdb.SeedPortalAccount(t, db, "already-accepted", "already-accepted@example.com")
	var rowID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ('already-accepted', $1) RETURNING id`,
		clientID,
	).Scan(&rowID); err != nil {
		t.Fatalf("seed accepted row: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	result, err := tx.ExecContext(t.Context(),
		`UPDATE client_portal_users SET invite_token = gen_random_uuid() WHERE id = $1`,
		rowID,
	)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected the accepted row to be untouched by the rotation door, got %d rows affected", rows)
	}
}

// TestRLS_ClientPortalUsersInviteUpdateCannotTouchCrossPracticeRow proves
// the rotation door is scoped to the caller's own Practice, not any
// pending row -- the same cross-Practice check invite_insert gets, applied
// to the update door too.
func TestRLS_ClientPortalUsersInviteUpdateCannotTouchCrossPracticeRow(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Update Cross Practice A")
	practiceB := seedPractice(t, db, "Update Cross Practice B")
	clientB, _ := seedClientEngagement(t, db, practiceB, "Client B", "clientb-update@example.com")
	var rowID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid()) RETURNING id`,
		clientB,
	).Scan(&rowID); err != nil {
		t.Fatalf("seed pending row for client B: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	result, err := tx.ExecContext(t.Context(),
		`UPDATE client_portal_users SET invite_token = gen_random_uuid() WHERE id = $1`,
		rowID,
	)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected client B's row to be untouched from practice A's context, got %d rows affected", rows)
	}
}

// TestRLS_ClientPortalUsersAcceptFailsClosedWithNoInviteTokenSet proves the
// accept-side policies deny a pending row with no app.invite_token set,
// even though a matching row genuinely exists.
func TestRLS_ClientPortalUsersAcceptFailsClosedWithNoInviteTokenSet(t *testing.T) {
	db := testdb.New(t)
	_, inviteToken := seedPendingPortalInvite(t, db)
	_ = inviteToken

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, "someone"); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM client_portal_users WHERE identity_uid IS NULL`).Scan(&count); err != nil {
		t.Fatalf("query pending rows with no invite_token set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 pending rows visible with no app.invite_token set, got %d", count)
	}
}

// TestRLS_ClientPortalUsersAcceptFailsClosedWithWrongInviteToken proves the
// accept-side policies deny a pending row when app.invite_token doesn't
// match.
func TestRLS_ClientPortalUsersAcceptFailsClosedWithWrongInviteToken(t *testing.T) {
	db := testdb.New(t)
	seedPendingPortalInvite(t, db)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.invite_token', $1, true)`, "00000000-0000-0000-0000-000000000000"); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM client_portal_users WHERE identity_uid IS NULL`).Scan(&count); err != nil {
		t.Fatalf("query pending rows with wrong invite_token: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 pending rows visible with a non-matching app.invite_token, got %d", count)
	}
}

// TestRLS_ClientPortalUsersAcceptSelectSucceedsWithMatchingToken proves the
// accept-side door actually opens for the matching case.
func TestRLS_ClientPortalUsersAcceptSelectSucceedsWithMatchingToken(t *testing.T) {
	db := testdb.New(t)
	clientID, inviteToken := seedPendingPortalInvite(t, db)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.invite_token', $1, true)`, inviteToken); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var gotClientID string
	if err := tx.QueryRowContext(t.Context(), `SELECT client_id FROM client_portal_users WHERE identity_uid IS NULL`).Scan(&gotClientID); err != nil {
		t.Fatalf("query pending row with matching invite_token: %v", err)
	}
	if gotClientID != clientID {
		t.Fatalf("client_id = %q, want %q", gotClientID, clientID)
	}
}

// TestRLS_ClientPortalUsersAcceptUpdateRejectsMismatchedIdentity proves
// client_portal_users_accept_update's WITH CHECK: presenting a valid
// invite_token isn't enough to claim a row for a different identity_uid
// than the caller's own -- the update must be rejected even though the
// USING clause (identity_uid IS NULL AND matching invite_token) is
// satisfied.
func TestRLS_ClientPortalUsersAcceptUpdateRejectsMismatchedIdentity(t *testing.T) {
	db := testdb.New(t)
	_, inviteToken := seedPendingPortalInvite(t, db)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, "the-real-caller"); err != nil {
		t.Fatalf("set_config identity: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.invite_token', $1, true)`, inviteToken); err != nil {
		t.Fatalf("set_config invite token: %v", err)
	}

	_, err = tx.ExecContext(t.Context(),
		`UPDATE client_portal_users SET identity_uid = 'someone-else', invite_token = NULL WHERE invite_token = $1`,
		inviteToken,
	)
	if err == nil {
		t.Fatal("expected the update to be rejected -- identity_uid must equal the caller's own app.current_identity_uid")
	}
}

// TestRLS_NotificationWorkerCannotReadClientPortalUsersWithoutTrustedFlag
// proves the outbox worker's own policies (00032) stay closed by default:
// with no session context at all -- the shape outbox.ProcessHandler runs
// under before it sets app.notification_worker_trusted -- neither this
// policy nor any pre-existing one admits a row.
func TestRLS_NotificationWorkerCannotReadClientPortalUsersWithoutTrustedFlag(t *testing.T) {
	db := testdb.New(t)
	seedPendingPortalInvite(t, db)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM client_portal_users`).Scan(&count); err != nil {
		t.Fatalf("query client_portal_users: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero visible rows with no session context set, got %d", count)
	}
}

// TestRLS_NotificationWorkerTrustedFlagOpensClientPortalUsersAndClients
// proves the door outbox.ProcessHandler relies on actually opens: with
// app.notification_worker_trusted set, every row of both tables is
// visible, regardless of Practice.
func TestRLS_NotificationWorkerTrustedFlagOpensClientPortalUsersAndClients(t *testing.T) {
	db := testdb.New(t)
	seedPendingPortalInvite(t, db)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var portalUserCount, clientCount int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM client_portal_users`).Scan(&portalUserCount); err != nil {
		t.Fatalf("query client_portal_users: %v", err)
	}
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM clients`).Scan(&clientCount); err != nil {
		t.Fatalf("query clients: %v", err)
	}
	if portalUserCount != 1 || clientCount != 1 {
		t.Fatalf("portalUserCount/clientCount = %d/%d, want 1/1", portalUserCount, clientCount)
	}
}
