package staffinvite_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

// seedPracticeInvitation inserts a practice_invitations row (00030) at
// practiceID, inviting address. token_digest is a fixed placeholder --
// this package's tests never verify it, since #316's accept flow (not
// built yet) is what will ever read it back.
func seedPracticeInvitation(t *testing.T, db *testdb.DB, practiceID, address string) string {
	t.Helper()
	invitedBy := testdb.SeedStaff(t, db, "inviting-owner-"+address)
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
