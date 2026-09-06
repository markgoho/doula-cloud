package authmail_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

const (
	testAppBaseURL = "https://app.example.test"
	testSenderAddr = "a@b.test"
	testReplyTo    = "support@b.test"

	statusSent = "sent"
)

// newTokenMailWorker and newEmailChangeWorker build a Worker around
// sender -- the one end-to-end test each keeps below needs it to prove
// claimQuery's columns still match Scan; every kind-specific branch
// belongs to compose_test.go (pure Compose), and skip/retry/dead-letter/
// suppression/terminal-clear belong to outbox.MailWorker's own suite.
func newTokenMailWorker(sender mail.Sender, accounts *authntest.FakeAccountManager) authmail.TokenMailWorker {
	return authmail.NewTokenMailWorker(outbox.Mailer{Sender: sender, Now: time.Now, AppBaseURL: testAppBaseURL, From: testSenderAddr, ReplyTo: testReplyTo}, accounts)
}

func newEmailChangeWorker(sender mail.Sender) authmail.EmailChangeWorker {
	return authmail.NewEmailChangeWorker(outbox.Mailer{Sender: sender, Now: time.Now, From: testSenderAddr, ReplyTo: testReplyTo})
}

func seedTokenMailRow(t *testing.T, db *testdb.DB, identityUID string, kind authmail.TokenMailKind, token string, attemptCount int, nextAttemptAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff_token_mail_outbox (identity_uid, kind, token, attempt_count, next_attempt_at)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		identityUID, kind, token, attemptCount, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed token mail row: %v", err)
	}
	return id
}

func tokenMailRowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int, token sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count, token FROM staff_token_mail_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount, &token); err != nil {
		t.Fatalf("query token mail row: %v", err)
	}
	return status, attemptCount, token
}

func seedEmailChangeRow(t *testing.T, db *testdb.DB, identityUID, oldEmail string, attemptCount int, nextAttemptAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff_email_change_outbox (identity_uid, old_email, attempt_count, next_attempt_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		identityUID, oldEmail, attemptCount, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed email change row: %v", err)
	}
	return id
}

func emailChangeRowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count FROM staff_email_change_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount); err != nil {
		t.Fatalf("query email change row: %v", err)
	}
	return status, attemptCount
}

func runTx(t *testing.T, db *testdb.DB, fn func(ctx context.Context, tx *sql.Tx) error) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(t.Context(), tx); err != nil {
		t.Fatalf("tx func: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestTokenMailWorker_ProcessPending_SendsVerificationAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-1", "person@example.com", false)
	rowID := seedTokenMailRow(t, db, "uid-1", authmail.KindEmailVerification, "verify-token", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runTx(t, db, newTokenMailWorker(sender, accounts).ProcessPending)

	status, _, token := tokenMailRowState(t, db, rowID)
	if status != statusSent {
		t.Fatalf("status = %q, want sent", status)
	}
	if token.Valid {
		t.Fatal("token still set once sent, want NULL")
	}
	sent := sender.Sent()
	if len(sent) != 1 || sent[0].To != "person@example.com" {
		t.Fatalf("sent = %+v, want one message to person@example.com", sent)
	}
}

func TestQueueTokenMail_InsertsPendingRowThenReRequestResetsToken(t *testing.T) {
	db := testdb.New(t)

	runTx(t, db, func(ctx context.Context, tx *sql.Tx) error {
		return authmail.QueueTokenMail(ctx, tx, "uid-8", authmail.KindPasswordReset, "first-token")
	})
	var firstID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT id FROM staff_token_mail_outbox WHERE identity_uid = $1`, "uid-8",
	).Scan(&firstID); err != nil {
		t.Fatalf("query first row: %v", err)
	}

	// A re-request resets the same row rather than inserting a second
	// one -- staff_token_mail_outbox_one_pending (00061) enforces this.
	runTx(t, db, func(ctx context.Context, tx *sql.Tx) error {
		return authmail.QueueTokenMail(ctx, tx, "uid-8", authmail.KindPasswordReset, "second-token")
	})

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM staff_token_mail_outbox WHERE identity_uid = $1`, "uid-8",
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1 (re-request resets, does not duplicate)", count)
	}

	var token string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT token FROM staff_token_mail_outbox WHERE id = $1`, firstID,
	).Scan(&token); err != nil {
		t.Fatalf("query token: %v", err)
	}
	if token != "second-token" {
		t.Fatalf("token = %q, want second-token", token)
	}
}

func TestEmailChangeWorker_ProcessPending_SendsAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	rowID := seedEmailChangeRow(t, db, "uid-9", "old@example.com", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runTx(t, db, newEmailChangeWorker(sender).ProcessPending)

	status, _ := emailChangeRowState(t, db, rowID)
	if status != statusSent {
		t.Fatalf("status = %q, want sent", status)
	}
	sent := sender.Sent()
	if len(sent) != 1 || sent[0].To != "old@example.com" {
		t.Fatalf("sent = %+v, want one message to old@example.com", sent)
	}
}

func TestQueueEmailChangeNotice_InsertsRow(t *testing.T) {
	db := testdb.New(t)

	runTx(t, db, func(ctx context.Context, tx *sql.Tx) error {
		return authmail.QueueEmailChangeNotice(ctx, tx, "uid-11", "old@example.com")
	})

	var oldEmail, status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT old_email, status FROM staff_email_change_outbox WHERE identity_uid = $1`, "uid-11",
	).Scan(&oldEmail, &status); err != nil {
		t.Fatalf("query row: %v", err)
	}
	if oldEmail != "old@example.com" || status != "pending" {
		t.Fatalf("old_email/status = %q/%q, want old@example.com/pending", oldEmail, status)
	}
}
