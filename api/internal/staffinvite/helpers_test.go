package staffinvite_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

// seedPractice inserts a Practice using the superuser Admin connection.
func seedPractice(t *testing.T, db *testdb.DB, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(), `INSERT INTO practices (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed practice %q: %v", name, err)
	}
	return id
}

// seedStaff inserts a Staff member (an Invitation's invited_by) using the
// superuser Admin connection.
func seedStaff(t *testing.T, db *testdb.DB, identityUID string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, 'Inviting Staff', 'inviter@example.com', 'NY') RETURNING id`,
		identityUID,
	).Scan(&id); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	return id
}

// seedPracticeInvitation inserts a practice_invitations row (00030) at
// practiceID, inviting address. token_digest is a fixed placeholder --
// this package's tests never verify it, since #316's accept flow (not
// built yet) is what will ever read it back.
func seedPracticeInvitation(t *testing.T, db *testdb.DB, practiceID, address string) string {
	t.Helper()
	invitedBy := seedStaff(t, db, "inviting-owner-"+address)
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practice_invitations (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, $2, '{doula}', 'employee', 'placeholder-digest', $3, $4) RETURNING id`,
		practiceID, address, invitedBy, time.Now().Add(72*time.Hour),
	).Scan(&id); err != nil {
		t.Fatalf("seed practice invitation: %v", err)
	}
	return id
}

// setInvitationStatus updates invitationID's status, using the superuser
// Admin connection -- how a test simulates an Invitation resolved (any
// status other than pending) through some other path (#316's future
// accept/revoke) before the worker gets to its outbox row.
func setInvitationStatus(t *testing.T, db *testdb.DB, invitationID, status string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE practice_invitations SET status = $1 WHERE id = $2`, status, invitationID); err != nil {
		t.Fatalf("set invitation status: %v", err)
	}
}

// setInvitationExpiresAt updates invitationID's expires_at, using the
// superuser Admin connection -- how a test simulates an Invitation whose
// window has lapsed without anything (no expiry sweep exists yet) ever
// flipping its status column to 'expired'.
func setInvitationExpiresAt(t *testing.T, db *testdb.DB, invitationID string, expiresAt time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE practice_invitations SET expires_at = $1 WHERE id = $2`, expiresAt, invitationID); err != nil {
		t.Fatalf("set invitation expires_at: %v", err)
	}
}
