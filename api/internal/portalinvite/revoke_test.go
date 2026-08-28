package portalinvite_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/testdb"
)

// TestRevokePending_DeadLettersThePendingRow proves the direct call
// contract: a pending portal_invite_outbox row for clientID is
// dead-lettered.
func TestRevokePending_DeadLettersThePendingRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "revoke-owner")
	clientID, _ := seedClientEngagement(t, db, practiceID, "Revoke Client", "revoke@example.com")
	var portalUserID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid()) RETURNING id`,
		clientID,
	).Scan(&portalUserID); err != nil {
		t.Fatalf("seed portal user: %v", err)
	}
	outboxID := seedOutboxRow(t, db, portalUserID, 0, time.Now())

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	// client_portal_users carries practice-tier RLS, reached through
	// clients.practice_id -- RevokePending's UPDATE joins through it, so
	// the session needs the same app.current_practice_id staffauth.Middleware
	// would have set on the real edit-handler request.
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}

	if err := portalinvite.RevokePending(t.Context(), tx, clientID); err != nil {
		t.Fatalf("RevokePending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	status, _ := outboxRowState(t, db, outboxID)
	if status != "dead_lettered" {
		t.Fatalf("status = %q, want dead_lettered", status)
	}
	var lastErr string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT last_error FROM portal_invite_outbox WHERE id = $1`, outboxID).Scan(&lastErr); err != nil {
		t.Fatalf("read last_error: %v", err)
	}
	if lastErr == "" {
		t.Fatal("last_error is empty, want a reason")
	}
}

// TestRevokePending_NoPendingRowIsANoop proves RevokePending returns
// cleanly when there is nothing to revoke.
func TestRevokePending_NoPendingRowIsANoop(t *testing.T) {
	db := testdb.New(t)
	clientID, _ := seedPendingPortalInvite(t, db)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if err := portalinvite.RevokePending(t.Context(), tx, clientID); err != nil {
		t.Fatalf("RevokePending on a Client with no outbox row: %v", err)
	}
}
