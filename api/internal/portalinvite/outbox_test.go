package portalinvite_test

import (
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/testdb"
)

// Shared literals across this package's outbox tests, pulled out per
// golangci-lint's goconst check.
const (
	testOutboxStatusPending = "pending"
	testOutboxStatusSent    = "sent"
	testAppBaseURL          = "https://app.example.test"
	testSenderAddr          = "a@b.test"
)

// newTestWorker builds a Worker around sender with this file's stand-in
// AppBaseURL/From/ReplyTo -- the one end-to-end test below needs it to
// prove claimQuery's columns still match scanRow; every other case
// belongs to compose_test.go (pure Compose) or outbox.MailWorker's own
// suite (skip/retry/dead-letter/suppression/terminal-clear).
func newTestWorker(sender mail.Sender) portalinvite.Worker {
	return portalinvite.NewWorker(outbox.Mailer{Sender: sender, Now: time.Now, AppBaseURL: testAppBaseURL, From: testSenderAddr, ReplyTo: "noreply@b.test"})
}

// seedPortalInviteOutboxRow inserts a pending portal_invite_outbox row for
// portalUserID with the given attempt_count/next_attempt_at, using the
// superuser Admin connection -- the table carries no RLS (00032), so
// db.App would work too, but Admin matches this package's seeding
// convention.
func seedPortalInviteOutboxRow(t *testing.T, db *testdb.DB, portalUserID string, attemptCount int, nextAttemptAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO portal_invite_outbox (client_portal_user_id, attempt_count, next_attempt_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		portalUserID, attemptCount, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	return id
}

// portalUserIDForClient looks up the client_portal_users row id
// seedPendingPortalInvite created for clientID.
func portalUserIDForClient(t *testing.T, db *testdb.DB, clientID string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT id FROM client_portal_users WHERE client_id = $1`, clientID).Scan(&id); err != nil {
		t.Fatalf("look up portal user id: %v", err)
	}
	return id
}

// runWorker begins a tx on db.App, sets the trusted session var
// outbox.ProcessHandler would otherwise set after its secret check, runs
// ProcessPending, and commits -- exercising the worker exactly as the
// handler drives it, without going through HTTP.
func runWorker(t *testing.T, db *testdb.DB, w portalinvite.Worker) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("set trusted session var: %v", err)
	}
	if err := w.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func outboxRowState(t *testing.T, db *testdb.DB, id string) (status string) {
	t.Helper()
	var attemptCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count FROM portal_invite_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount); err != nil {
		t.Fatalf("query outbox row: %v", err)
	}
	return status
}

func TestWorker_ProcessPending_SendsDueRowAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	clientID, inviteToken := seedPendingPortalInvite(t, db)
	portalUserID := portalUserIDForClient(t, db, clientID)
	outboxID := seedPortalInviteOutboxRow(t, db, portalUserID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testOutboxStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	if sent[0].To != "invited@example.com" {
		t.Fatalf("To = %q", sent[0].To)
	}
	wantLink := testAppBaseURL + "/portal/accept-invite?token=" + inviteToken
	if !strings.Contains(sent[0].Text, wantLink) {
		t.Fatalf("body %q does not contain link %q", sent[0].Text, wantLink)
	}
}
