package staffinvite_test

import (
	"database/sql"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/staffinvite"
	"doula-cloud/api/internal/testdb"
)

const (
	testOutboxStatusPending = "pending"
	testOutboxStatusSent    = "sent"
	testAppBaseURL          = "https://app.example.test"
	testSenderAddr          = "a@b.test"
	testInvitedAddress      = "invited-staff@example.com"
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

// queuedInvite is an Invitation of testInvitedAddress with a pending
// staff_invite_outbox row due a minute ago, carrying token. The row is
// inserted with the superuser Admin connection -- the table carries no RLS
// (00038), so db.App would work too, but Admin matches this package's
// seeding convention.
func queuedInvite(t *testing.T, practiceName, token string) (db *testdb.DB, outboxID string) {
	t.Helper()
	db = testdb.New(t)
	invitationID := seedPracticeInvitation(t, db, testdb.SeedPractice(t, db, practiceName), testInvitedAddress)
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff_invite_outbox (invitation_id, invite_token, attempt_count, next_attempt_at)
		 VALUES ($1, $2, 0, $3) RETURNING id`,
		invitationID, token, time.Now().Add(-time.Minute),
	).Scan(&outboxID); err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	return db, outboxID
}

// newTestWorker builds a Worker around sender with this package's
// stand-in AppBaseURL/From/ReplyTo -- the one end-to-end test needs it to
// prove claimQuery's columns still match scanRow; every other case
// belongs to compose_test.go (pure Compose) or outbox.MailWorker's own
// suite (skip/retry/dead-letter/suppression/terminal-clear).
func newTestWorker(sender mail.Sender) staffinvite.Worker {
	return staffinvite.NewWorker(outbox.Mailer{Sender: sender, Now: time.Now, AppBaseURL: testAppBaseURL, From: testSenderAddr, ReplyTo: "support@b.test"})
}

// runWorker runs ProcessPending in a transaction with the worker trusted,
// the way outbox.ProcessHandler does after its secret check, and commits
// -- exercising the worker exactly as the handler drives it, without going
// through HTTP.
func runWorker(t *testing.T, db *testdb.DB, w staffinvite.Worker) {
	t.Helper()
	db.RunAsWorker(t, w.ProcessPending)
}

func outboxRowState(t *testing.T, db *testdb.DB, id string) (status string, inviteToken sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, invite_token::text FROM staff_invite_outbox WHERE id = $1`, id,
	).Scan(&status, &inviteToken); err != nil {
		t.Fatalf("query outbox row: %v", err)
	}
	return status, inviteToken
}

// queue runs staffinvite.Queue for invitationID in a transaction of its
// own and commits it.
func queue(t *testing.T, db *testdb.DB, invitationID, token string) {
	t.Helper()
	tx := db.BeginAs(t)
	if err := staffinvite.Queue(t.Context(), tx, invitationID, token); err != nil {
		t.Fatalf("Queue: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
