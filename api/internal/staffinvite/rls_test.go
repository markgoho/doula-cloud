package staffinvite_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the practice_invitations_notification_worker
// policy from 00038_staff_invite_outbox.sql directly via db.App and
// set_config, mirroring portalinvite's equivalent pair for
// client_portal_users/clients (00032).

// TestRLS_NotificationWorkerCannotReadPracticeInvitationsWithoutTrustedFlag
// proves the policy stays closed by default: with no session context at
// all -- the shape outbox.ProcessHandler runs under before it sets
// app.notification_worker_trusted -- practice_invitations_practice_visibility
// (00030) admits nothing either, since app.current_practice_id is unset.
func TestRLS_NotificationWorkerCannotReadPracticeInvitationsWithoutTrustedFlag(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "No Session Practice")
	seedPracticeInvitation(t, db, practiceID, testInvitedAddress)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM practice_invitations`).Scan(&count); err != nil {
		t.Fatalf("query practice_invitations: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero visible rows with no session context set, got %d", count)
	}
}

// TestRLS_NotificationWorkerTrustedFlagOpensPracticeInvitationsAcrossPractices
// proves the door outbox.ProcessHandler relies on actually opens: with
// app.notification_worker_trusted set, every Invitation is visible,
// regardless of Practice -- required since the worker has no single
// Practice's session to scope by.
func TestRLS_NotificationWorkerTrustedFlagOpensPracticeInvitationsAcrossPractices(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Worker Visibility A")
	practiceB := seedPractice(t, db, "Worker Visibility B")
	seedPracticeInvitation(t, db, practiceA, "a@example.com")
	seedPracticeInvitation(t, db, practiceB, "b@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM practice_invitations`).Scan(&count); err != nil {
		t.Fatalf("query practice_invitations: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2 (both Practices' Invitations visible)", count)
	}
}
