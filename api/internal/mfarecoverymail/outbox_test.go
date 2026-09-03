package mfarecoverymail_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mfarecoverymail"
	"doula-cloud/api/internal/testdb"
)

const (
	testSenderAddr = "a@b.test"
	testReplyTo    = "support@b.test"

	statusPending      = "pending"
	statusSent         = "sent"
	statusDeadLettered = "dead_lettered"
)

func newWorker(sender mail.Sender, accounts *authntest.FakeAccountManager) mfarecoverymail.Worker {
	return mfarecoverymail.Worker{Sender: sender, Accounts: accounts, Now: time.Now, From: testSenderAddr, ReplyTo: testReplyTo}
}

func seedStaffRow(t *testing.T, db *testdb.DB, identityUID, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, $2, $1 || '@example.com', 'NY') RETURNING id`,
		identityUID, name,
	).Scan(&id); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	return id
}

func seedOutboxRow(t *testing.T, db *testdb.DB, recipientIdentityUID, subjectStaffID, token string, nextAttemptAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff_mfa_recovery_outbox (recipient_identity_uid, subject_staff_id, token, next_attempt_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		recipientIdentityUID, subjectStaffID, token, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	return id
}

func rowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int, token sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count, token FROM staff_mfa_recovery_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount, &token); err != nil {
		t.Fatalf("query outbox row: %v", err)
	}
	return status, attemptCount, token
}

// runTx mirrors outbox.ProcessHandler's own transaction setup: the real
// caller sets app.notification_worker_trusted before running a worker, so
// a test bypassing that handler (to call ProcessPending directly) has to
// set the same flag itself -- staff_notification_worker (00033) is what
// lets subjectName's plain `staff` SELECT past RLS at all.
func runTx(t *testing.T, db *testdb.DB, fn func(ctx context.Context, tx *sql.Tx) error) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("set trusted flag: %v", err)
	}
	if err := fn(t.Context(), tx); err != nil {
		t.Fatalf("tx func: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestQueueVouchedCodeMail_InsertsPendingRow(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedStaffRow(t, db, "subject-queue", "Priya Raman")

	runTx(t, db, func(ctx context.Context, tx *sql.Tx) error {
		return mfarecoverymail.QueueVouchedCodeMail(ctx, tx, "owner-uid-queue", subjectID, "12345678")
	})

	var recipientUID, token string
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT recipient_identity_uid, token, status FROM staff_mfa_recovery_outbox WHERE subject_staff_id = $1`, subjectID,
	).Scan(&recipientUID, &token, &status); err != nil {
		t.Fatalf("query row: %v", err)
	}
	if recipientUID != "owner-uid-queue" || token != "12345678" || status != statusPending {
		t.Fatalf("row = (%q, %q, %q), want (owner-uid-queue, 12345678, pending)", recipientUID, token, status)
	}
}

func TestWorker_ProcessPending_SendsToRecipientAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedStaffRow(t, db, "subject-1", "Priya Raman")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-1", "owner@example.com", true)
	rowID := seedOutboxRow(t, db, "owner-uid-1", subjectID, "87654321", time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runTx(t, db, newWorker(sender, accounts).ProcessPending)

	status, _, token := rowState(t, db, rowID)
	if status != statusSent {
		t.Fatalf("status = %q, want sent", status)
	}
	if token.Valid {
		t.Fatal("token still set once sent, want NULL")
	}
	sent := sender.Sent()
	if len(sent) != 1 || sent[0].To != "owner@example.com" {
		t.Fatalf("sent = %+v, want one message to owner@example.com", sent)
	}
	if !strings.Contains(sent[0].Text, "Priya Raman") || !strings.Contains(sent[0].Text, "87654321") {
		t.Fatalf("mail body = %q, want it to name the subject and carry the code", sent[0].Text)
	}
}

func TestWorker_ProcessPending_SkipsRowNotYetDue(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedStaffRow(t, db, "subject-2", "Someone")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-2", "owner2@example.com", true)
	rowID := seedOutboxRow(t, db, "owner-uid-2", subjectID, "11112222", time.Now().Add(time.Hour))

	sender := &mail.FakeSender{}
	runTx(t, db, newWorker(sender, accounts).ProcessPending)

	status, _, _ := rowState(t, db, rowID)
	if status != statusPending {
		t.Fatalf("status = %q, want pending", status)
	}
	if len(sender.Sent()) != 0 {
		t.Fatal("expected no send for a not-yet-due row")
	}
}

func TestWorker_ProcessPending_RetriesOnSendFailure(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedStaffRow(t, db, "subject-3", "Someone")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-3", "owner3@example.com", true)
	rowID := seedOutboxRow(t, db, "owner-uid-3", subjectID, "33334444", time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runTx(t, db, newWorker(sender, accounts).ProcessPending)

	status, attemptCount, token := rowState(t, db, rowID)
	if status != statusPending || attemptCount != 1 {
		t.Fatalf("status/attempt_count = %q/%d, want pending/1", status, attemptCount)
	}
	if !token.Valid {
		t.Fatal("token cleared after a retry, want it preserved for the next attempt")
	}
}

func TestWorker_ProcessPending_AccountManagerErrorRetries(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedStaffRow(t, db, "subject-5", "Someone")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-5", "owner5@example.com", true)
	accounts.Err = errors.New("admin sdk unreachable")
	rowID := seedOutboxRow(t, db, "owner-uid-5", subjectID, "77778888", time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runTx(t, db, newWorker(sender, accounts).ProcessPending)

	status, attemptCount, _ := rowState(t, db, rowID)
	if status != statusPending || attemptCount != 1 {
		t.Fatalf("status/attempt_count = %q/%d, want pending/1", status, attemptCount)
	}
}

func TestWorker_ProcessPending_DeadLettersUnknownRecipient(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedStaffRow(t, db, "subject-4", "Someone")
	accounts := authntest.NewFakeAccountManager() // no account seeded for the recipient
	rowID := seedOutboxRow(t, db, "owner-uid-gone", subjectID, "55556666", time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runTx(t, db, newWorker(sender, accounts).ProcessPending)

	status, _, _ := rowState(t, db, rowID)
	if status != statusDeadLettered {
		t.Fatalf("status = %q, want dead_lettered", status)
	}
	if len(sender.Sent()) != 0 {
		t.Fatal("expected no send for an unknown recipient")
	}
}
