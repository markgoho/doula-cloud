package mfarecoverymail_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mfarecoverymail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

const (
	testSenderAddr = "a@b.test"
	testReplyTo    = "support@b.test"

	statusPending = "pending"
	statusSent    = "sent"
)

// newWorker builds a Worker around sender -- the one end-to-end test
// below needs it to prove claimQuery's columns still match Scan; every
// kind-specific branch belongs to compose_test.go (pure Compose), and
// skip/retry/dead-letter/suppression/terminal-clear belong to
// outbox.MailWorker's own suite.
func newWorker(sender mail.Sender, accounts *authntest.FakeAccountManager) mfarecoverymail.Worker {
	return mfarecoverymail.NewWorker(outbox.Mailer{Sender: sender, Now: time.Now, From: testSenderAddr, ReplyTo: testReplyTo}, accounts)
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

// seedMFARecoveryOutboxRow inserts a pending staff_mfa_recovery_outbox
// row directly, using the superuser Admin connection. Stays local under
// this name rather than testdb: each outbox table (staff_mfa_recovery,
// portal_invite, staff_invite, session_notice) has its own columns, so
// there is no single shared row shape to promote.
func seedMFARecoveryOutboxRow(t *testing.T, db *testdb.DB, recipientIdentityUID, subjectStaffID, token string, nextAttemptAt time.Time) string {
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

func rowState(t *testing.T, db *testdb.DB, id string) (status string, token sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, token FROM staff_mfa_recovery_outbox WHERE id = $1`, id,
	).Scan(&status, &token); err != nil {
		t.Fatalf("query outbox row: %v", err)
	}
	return status, token
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
	// The vouching Owner needs a staff row of her own, not only an
	// Identity Platform account: #892's recheck reads her live row before
	// the Admin SDK is reached at all.
	seedStaffRow(t, db, "owner-uid-1", "Renata Alves")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-1", "owner@example.com", true)
	rowID := seedMFARecoveryOutboxRow(t, db, "owner-uid-1", subjectID, "87654321", time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runTx(t, db, newWorker(sender, accounts).ProcessPending)

	status, token := rowState(t, db, rowID)
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

// TestWorker_ProcessPending_DeletedRecipientMarksSentWithNoMail is
// #892's skip-at-send recheck seen from the worker rather than from
// Compose: the vouching Owner deleted her own login between approving
// the request and this row being claimed. Her Identity Platform account
// went with it, so the row would otherwise dead-letter on
// ErrAccountNotFound; instead it is marked sent, with nothing mailed.
func TestWorker_ProcessPending_DeletedRecipientMarksSentWithNoMail(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedStaffRow(t, db, "subject-2", "Priya Raman")
	ownerStaffID := seedStaffRow(t, db, "owner-uid-2", "Renata Alves")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-2", "owner@example.com", true)
	rowID := seedMFARecoveryOutboxRow(t, db, "owner-uid-2", subjectID, "24682468", time.Now().Add(-time.Minute))
	testdb.RedactDeletedLogin(t, db, ownerStaffID)

	sender := &mail.FakeSender{}
	runTx(t, db, newWorker(sender, accounts).ProcessPending)

	status, _ := rowState(t, db, rowID)
	if status != statusSent {
		t.Fatalf("status = %q, want sent", status)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail for an Owner who deleted her login, got %d", len(sender.Sent()))
	}
}
