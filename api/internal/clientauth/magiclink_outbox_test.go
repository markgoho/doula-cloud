package clientauth_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/testdb"
)

const (
	testAppBaseURL = "https://app.example.test"
	testSenderAddr = "a@b.test"
	testReplyTo    = "support@b.test"

	statusPending      = "pending"
	statusSent         = "sent"
	statusDeadLettered = "dead_lettered"
)

func newMagicLinkWorker(sender mail.Sender) clientauth.MagicLinkWorker {
	return clientauth.MagicLinkWorker{
		Sender: sender, Now: time.Now,
		AppBaseURL: testAppBaseURL, From: testSenderAddr, ReplyTo: testReplyTo,
	}
}

func seedMagicLinkRow(t *testing.T, db *testdb.DB, identityUID string, attemptCount int, nextAttemptAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO portal_magic_link_outbox (identity_uid, token, attempt_count, next_attempt_at)
		 VALUES ($1, 'link-token', $2, $3) RETURNING id`,
		identityUID, attemptCount, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed magic link row: %v", err)
	}
	return id
}

func magicLinkRowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int, token sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count, token FROM portal_magic_link_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount, &token); err != nil {
		t.Fatalf("query magic link row: %v", err)
	}
	return status, attemptCount, token
}

func runMagicLinkTx(t *testing.T, db *testdb.DB, fn func(ctx context.Context, tx *sql.Tx) error) {
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

func TestMagicLinkWorker_ProcessPending_SendsAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_worker-send"
	testdb.SeedPortalAccount(t, db, identifier, "worker-send@example.com")
	rowID := seedMagicLinkRow(t, db, identifier, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runMagicLinkTx(t, db, newMagicLinkWorker(sender).ProcessPending)

	status, _, token := magicLinkRowState(t, db, rowID)
	if status != statusSent {
		t.Fatalf("status = %q, want sent", status)
	}
	if token.Valid {
		t.Fatal("token still set once sent, want NULL")
	}
	sent := sender.Sent()
	if len(sent) != 1 || sent[0].To != "worker-send@example.com" {
		t.Fatalf("sent = %+v, want one message to worker-send@example.com", sent)
	}
}

func TestMagicLinkWorker_ProcessPending_SkipsRowNotYetDue(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_worker-not-due"
	testdb.SeedPortalAccount(t, db, identifier, "worker-not-due@example.com")
	rowID := seedMagicLinkRow(t, db, identifier, 0, time.Now().Add(time.Hour))

	sender := &mail.FakeSender{}
	runMagicLinkTx(t, db, newMagicLinkWorker(sender).ProcessPending)

	status, _, _ := magicLinkRowState(t, db, rowID)
	if status != statusPending {
		t.Fatalf("status = %q, want pending", status)
	}
	if len(sender.Sent()) != 0 {
		t.Fatal("expected no send for a not-yet-due row")
	}
}

func TestMagicLinkWorker_ProcessPending_RetriesOnSendFailure(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_worker-retry"
	testdb.SeedPortalAccount(t, db, identifier, "worker-retry@example.com")
	rowID := seedMagicLinkRow(t, db, identifier, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runMagicLinkTx(t, db, newMagicLinkWorker(sender).ProcessPending)

	status, attemptCount, token := magicLinkRowState(t, db, rowID)
	if status != statusPending || attemptCount != 1 {
		t.Fatalf("status/attempt_count = %q/%d, want pending/1", status, attemptCount)
	}
	if !token.Valid {
		t.Fatal("token cleared after a retry, want it preserved for the next attempt")
	}
}

func TestMagicLinkWorker_ProcessPending_DeadLettersAfterFinalAttempt(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_worker-final"
	testdb.SeedPortalAccount(t, db, identifier, "worker-final@example.com")
	rowID := seedMagicLinkRow(t, db, identifier, 4, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runMagicLinkTx(t, db, newMagicLinkWorker(sender).ProcessPending)

	status, attemptCount, token := magicLinkRowState(t, db, rowID)
	if status != statusDeadLettered || attemptCount != 5 {
		t.Fatalf("status/attempt_count = %q/%d, want dead_lettered/5", status, attemptCount)
	}
	if token.Valid {
		t.Fatal("token still set once dead-lettered, want NULL -- it will never be sent")
	}
}

// TestMagicLinkWorker_ProcessPending_ErasureCascadesAwayThePendingRow
// proves the FK does the work a dead-letter branch would otherwise have
// to: deleting the Portal Account behind a still-pending row deletes the
// row with it (ON DELETE CASCADE, 00074), rather than leaving it to be
// claimed with nothing to send to.
func TestMagicLinkWorker_ProcessPending_ErasureCascadesAwayThePendingRow(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_worker-ghost"
	testdb.SeedPortalAccount(t, db, identifier, "worker-ghost@example.com")
	rowID := seedMagicLinkRow(t, db, identifier, 0, time.Now().Add(-time.Minute))
	if _, err := db.Admin.ExecContext(t.Context(), `DELETE FROM portal_accounts WHERE identifier = $1`, identifier); err != nil {
		t.Fatalf("delete portal account: %v", err)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM portal_magic_link_outbox WHERE id = $1`, rowID).Scan(&count); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	if count != 0 {
		t.Fatal("expected the outbox row to be cascade-deleted with its Portal Account")
	}
}
