package portalinvite_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/testdb"
)

// Shared literals across this package's outbox tests (outbox_test.go and
// outbox_handler_test.go), pulled out per golangci-lint's goconst check.
const (
	testOutboxStatusPending      = "pending"
	testOutboxStatusSent         = "sent"
	testOutboxStatusDeadLettered = "dead_lettered"
	testAppBaseURL               = "https://app.example.test"
	testSenderAddr               = "a@b.test"
)

// newTestWorker builds a Worker around sender with this file's stand-in
// AppBaseURL/From/ReplyTo -- every outbox test needs one, only the
// injected Sender and (occasionally) Now vary.
func newTestWorker(sender mail.Sender) portalinvite.Worker {
	return portalinvite.Worker{Sender: sender, Now: time.Now, AppBaseURL: testAppBaseURL, From: testSenderAddr, ReplyTo: "noreply@b.test"}
}

// seedOutboxRow inserts a pending portal_invite_outbox row for
// portalUserID with the given attempt_count/next_attempt_at, using the
// superuser Admin connection -- the table carries no RLS (00032), so
// db.App would work too, but Admin matches this package's seeding
// convention.
func seedOutboxRow(t *testing.T, db *testdb.DB, portalUserID string, attemptCount int, nextAttemptAt time.Time) string {
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

func outboxRowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count FROM portal_invite_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount); err != nil {
		t.Fatalf("query outbox row: %v", err)
	}
	return status, attemptCount
}

func TestWorker_ProcessPending_SendsDueRowAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	clientID, inviteToken := seedPendingPortalInvite(t, db)
	portalUserID := portalUserIDForClient(t, db, clientID)
	outboxID := seedOutboxRow(t, db, portalUserID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _ := outboxRowState(t, db, outboxID)
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

// TestWorker_ProcessPending_DeadLettersRowForClientWithNoEmail proves
// ADR-0017's ride-along: outbox.go must refuse a Client with no email
// rather than send to an empty string. A row for such a Client is
// dead-lettered outright, not scheduled for retry, and nothing is sent.
func TestWorker_ProcessPending_DeadLettersRowForClientWithNoEmail(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "no-email-owner")
	clientID, _ := seedClientEngagement(t, db, practiceID, "No Email Client", "")
	var portalUserID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid()) RETURNING id`,
		clientID,
	).Scan(&portalUserID); err != nil {
		t.Fatalf("seed portal user: %v", err)
	}
	outboxID := seedOutboxRow(t, db, portalUserID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusDeadLettered {
		t.Fatalf("status = %q, want %s", status, testOutboxStatusDeadLettered)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no send for a Client with no email, got %d", len(sender.Sent()))
	}
}

func TestWorker_ProcessPending_SkipsRowNotYetDue(t *testing.T) {
	db := testdb.New(t)
	clientID, _ := seedPendingPortalInvite(t, db)
	portalUserID := portalUserIDForClient(t, db, clientID)
	outboxID := seedOutboxRow(t, db, portalUserID, 0, time.Now().Add(time.Hour))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusPending {
		t.Fatalf("status = %q, want %s (not due yet)", status, testOutboxStatusPending)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no send for a not-yet-due row")
	}
}

func TestWorker_ProcessPending_RetriesOnSendFailure(t *testing.T) {
	db := testdb.New(t)
	clientID, _ := seedPendingPortalInvite(t, db)
	portalUserID := portalUserIDForClient(t, db, clientID)
	outboxID := seedOutboxRow(t, db, portalUserID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runWorker(t, db, newTestWorker(sender))

	status, attemptCount := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusPending || attemptCount != 1 {
		t.Fatalf("status/attempt_count = %q/%d, want %s/1", status, attemptCount, testOutboxStatusPending)
	}
}

func TestWorker_ProcessPending_DeadLettersAfterFinalAttempt(t *testing.T) {
	db := testdb.New(t)
	clientID, _ := seedPendingPortalInvite(t, db)
	portalUserID := portalUserIDForClient(t, db, clientID)
	// One attempt short of the schedule's length -- this failure is the
	// last one before dead-letter.
	outboxID := seedOutboxRow(t, db, portalUserID, 4, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runWorker(t, db, newTestWorker(sender))

	status, attemptCount := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusDeadLettered || attemptCount != 5 {
		t.Fatalf("status/attempt_count = %q/%d, want dead_lettered/5", status, attemptCount)
	}
}

func TestWorker_ProcessPending_AlreadyAcceptedSkipsSend(t *testing.T) {
	db := testdb.New(t)
	clientID, _ := seedPendingPortalInvite(t, db)
	portalUserID := portalUserIDForClient(t, db, clientID)
	// The Client claimed the invite through some other path before the
	// worker got to this row.
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE client_portal_users SET identity_uid = 'already-claimed', invite_token = NULL WHERE id = $1`, portalUserID); err != nil {
		t.Fatalf("mark accepted: %v", err)
	}
	outboxID := seedOutboxRow(t, db, portalUserID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testOutboxStatusSent)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent for an already-accepted invite")
	}
}
