package mfarecoverymail_test

import (
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

func seedStaffRow(t *testing.T, db *testdb.DB, identityUID, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, first_name, last_name, email, work_state) VALUES ($1, $2, '', $1 || '@example.com', 'NY') RETURNING id`,
		identityUID, name,
	).Scan(&id); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	return id
}

// recovery is a pending staff_mfa_recovery_outbox row for a vouching Owner
// who has a staff row and an Identity Platform account, and the worker
// that sends it. The one end-to-end test below needs the worker to prove
// claimQuery's columns still match Scan; every kind-specific branch
// belongs to compose_test.go, and skip/retry/dead-letter/suppression/
// terminal-clear belong to outbox.MailWorker's own suite.
type recovery struct {
	db      *testdb.DB
	rowID   string
	ownerID string
	sender  *mail.FakeSender
	worker  mfarecoverymail.Worker
}

// seedRecovery inserts the row directly, using the superuser Admin
// connection. Stays local rather than in testdb: each outbox table
// (staff_mfa_recovery, portal_invite, staff_invite, session_notice) has
// its own columns, so there is no single shared row shape to promote.
//
// The vouching Owner needs a staff row of her own, not only an Identity
// Platform account: #892's recheck reads her live row before the Admin SDK
// is reached at all.
func seedRecovery(t *testing.T, token string) recovery {
	t.Helper()
	db := testdb.New(t)
	subjectID := seedStaffRow(t, db, "subject", "Priya Raman")
	ownerID := seedStaffRow(t, db, "owner-uid", "Renata Alves")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid", "owner@example.com", true)
	var rowID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff_mfa_recovery_outbox (recipient_identity_uid, subject_staff_id, token, next_attempt_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		"owner-uid", subjectID, token, time.Now().Add(-time.Minute),
	).Scan(&rowID); err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	sender := &mail.FakeSender{}
	mailer := outbox.Mailer{Sender: sender, Now: time.Now, From: testSenderAddr, ReplyTo: testReplyTo}
	return recovery{db: db, rowID: rowID, ownerID: ownerID, sender: sender, worker: mfarecoverymail.NewWorker(mailer, accounts)}
}

// process runs the worker the way outbox.ProcessHandler does: in a
// transaction with the worker trusted, committed.
func (r recovery) process(t *testing.T) {
	t.Helper()
	tx := r.db.BeginAs(t, testdb.NotificationWorker())
	if err := r.worker.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func (r recovery) state(t *testing.T) (status string, token sql.NullString) {
	t.Helper()
	if err := r.db.Admin.QueryRowContext(t.Context(),
		`SELECT status, token FROM staff_mfa_recovery_outbox WHERE id = $1`, r.rowID,
	).Scan(&status, &token); err != nil {
		t.Fatalf("query outbox row: %v", err)
	}
	return status, token
}

func TestQueueVouchedCodeMail_InsertsPendingRow(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedStaffRow(t, db, "subject-queue", "Priya Raman")

	tx := db.BeginAs(t, testdb.NotificationWorker())
	if err := mfarecoverymail.QueueVouchedCodeMail(t.Context(), tx, "owner-uid-queue", subjectID, "12345678"); err != nil {
		t.Fatalf("tx func: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var recipientUID, token, status string
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
	r := seedRecovery(t, "87654321")

	r.process(t)

	status, token := r.state(t)
	if status != statusSent {
		t.Fatalf("status = %q, want sent", status)
	}
	if token.Valid {
		t.Fatal("token still set once sent, want NULL")
	}
	sent := r.sender.Sent()
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
	r := seedRecovery(t, "24682468")
	testdb.RedactDeletedLogin(t, r.db, r.ownerID)

	r.process(t)

	status, _ := r.state(t)
	if status != statusSent {
		t.Fatalf("status = %q, want sent", status)
	}
	if len(r.sender.Sent()) != 0 {
		t.Fatalf("expected no mail for an Owner who deleted her login, got %d", len(r.sender.Sent()))
	}
}
