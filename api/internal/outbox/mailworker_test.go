package outbox_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// mwRow is the scratch row shape every MailWorker test composes from --
// only what a Compose function needs to decide skip/dead-letter/send/error.
type mwRow struct {
	outcome string
	to      string
}

func scanMWRow(outcome, to string) func(*sql.Rows) (outbox.RowMeta, mwRow, error) {
	return func(rows *sql.Rows) (outbox.RowMeta, mwRow, error) {
		var meta outbox.RowMeta
		if err := rows.Scan(&meta.ID, &meta.AttemptCount); err != nil {
			return meta, mwRow{}, fmt.Errorf("scan mw row: %w", err)
		}
		return meta, mwRow{outcome: outcome, to: to}, nil
	}
}

// newMailWorker builds a MailWorker[mwRow] whose Compose branches on
// outcome: "send" returns a message to r.to, "already" returns
// ErrAlreadyDone, "deadletter" returns a *DeadLetterError, and anything
// else is returned as a plain retryable error.
func newMailWorker(sender mail.Sender, outcome, to string) outbox.MailWorker[mwRow] {
	return outbox.MailWorker[mwRow]{
		Mailer:     outbox.Mailer{Sender: sender, Now: time.Now, From: testFrom, ReplyTo: testReplyTo},
		Table:      testTable,
		ClaimQuery: testClaimQuery,
		Scan:       scanMWRow(outcome, to),
		Compose: func(_ context.Context, _ *sql.Tx, r mwRow, _ time.Time) (string, string, string, error) {
			switch r.outcome {
			case "send":
				return r.to, "subject", "text", nil
			case "already":
				return "", "", "", outbox.ErrAlreadyDone
			case "deadletter":
				return "", "", "", &outbox.DeadLetterError{Reason: testDeadReason}
			default:
				return "", "", "", errors.New("compose boom")
			}
		},
	}
}

func runMailWorker(t *testing.T, db *testdb.DB, w outbox.MailWorker[mwRow]) {
	t.Helper()
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := w.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestDeadLetterError_ErrorReturnsReason(t *testing.T) {
	err := &outbox.DeadLetterError{Reason: testDeadReason}
	if err.Error() != testDeadReason {
		t.Fatalf("Error() = %q, want %q", err.Error(), testDeadReason)
	}
}

func TestMailWorker_ComposeSendsAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := &mail.FakeSender{}

	runMailWorker(t, db, newMailWorker(sender, "send", "recipient@example.test"))

	sent := sender.Sent()
	if len(sent) != 1 || sent[0].To != "recipient@example.test" {
		t.Fatalf("sent = %+v, want one message to recipient@example.test", sent)
	}
	got := readTestRow(t, db, "row-1")
	if got.status != testStatusSent {
		t.Fatalf("status = %q, want %s", got.status, testStatusSent)
	}
}

func TestMailWorker_ComposeAlreadyDoneMarksSentWithoutSending(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := &mail.FakeSender{}

	runMailWorker(t, db, newMailWorker(sender, "already", ""))

	if len(sender.Sent()) != 0 {
		t.Fatalf("sent %d messages, want 0 for an already-resolved row", len(sender.Sent()))
	}
	got := readTestRow(t, db, "row-1")
	if got.status != testStatusSent {
		t.Fatalf("status = %q, want %s", got.status, testStatusSent)
	}
}

func TestMailWorker_ComposeDeadLetterErrorDeadLettersOnTheSpot(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := &mail.FakeSender{}

	runMailWorker(t, db, newMailWorker(sender, "deadletter", ""))

	if len(sender.Sent()) != 0 {
		t.Fatalf("sent %d messages, want 0 for a dead-lettered row", len(sender.Sent()))
	}
	got := readTestRow(t, db, "row-1")
	if got.status != testStatusDead {
		t.Fatalf("status = %q, want %s", got.status, testStatusDead)
	}
	if got.attemptCount != 0 {
		t.Fatalf("attempt_count = %d, want 0 (outright dead-letter, not a retry)", got.attemptCount)
	}
	if got.lastError.String != testDeadReason {
		t.Fatalf("last_error = %q, want the DeadLetterError's Reason", got.lastError.String)
	}
}

// "error wrap" case: with the claim-scan-compose-send-mark loop living
// inside this package now, a kind's own Compose returning a plain error
// crosses no package boundary that needs a wrapOutboxErr of its own --
// MailWorker.ProcessPending schedules the retry directly, storing
// Compose's own error text on the row unwrapped.
func TestMailWorker_ComposePlainErrorSchedulesRetryWithOwnErrorText(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := &mail.FakeSender{}

	runMailWorker(t, db, newMailWorker(sender, "boom", ""))

	got := readTestRow(t, db, "row-1")
	if got.status != testStatusPending {
		t.Fatalf("status = %q, want %s (scheduled for retry)", got.status, testStatusPending)
	}
	if got.attemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1", got.attemptCount)
	}
	if got.lastError.String != "compose boom" {
		t.Fatalf("last_error = %q, want Compose's own error text", got.lastError.String)
	}
}

func TestMailWorker_SendFailureSchedulesRetry(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := &mail.FakeSender{Err: errors.New("mailgun down")}

	runMailWorker(t, db, newMailWorker(sender, "send", "recipient@example.test"))

	got := readTestRow(t, db, "row-1")
	if got.status != testStatusPending {
		t.Fatalf("status = %q, want %s", got.status, testStatusPending)
	}
	if got.attemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1", got.attemptCount)
	}
}

// A suppressed address (ADR-0029) dead-letters on the spot, the same as
// every other kind that sends through the wrapped Sender main.go
// constructs -- MailWorker never special-cases mail.ErrSuppressed itself;
// it reaches the same MarkFailed every hand-written kind's send error did.
func TestMailWorker_SuppressedAddressDeadLettersThroughWrappedSender(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := newSuppressingSender("recipient@example.test")

	runMailWorker(t, db, newMailWorker(sender, "send", "recipient@example.test"))

	got := readTestRow(t, db, "row-1")
	if got.status != testStatusDead {
		t.Fatalf("status = %q, want %s (suppressed, no retry)", got.status, testStatusDead)
	}
	if got.attemptCount != 0 {
		t.Fatalf("attempt_count = %d, want 0 (dead-lettered outright)", got.attemptCount)
	}
}

func TestMailWorker_MarkSentClearsConfiguredTerminalColumns(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := &mail.FakeSender{}

	w := newMailWorker(sender, "send", "recipient@example.test")
	w.ClearOnTerminal = []string{"secret_a", "secret_b"}
	runMailWorker(t, db, w)

	got := readTestRow(t, db, "row-1")
	if got.secretA.Valid || got.secretB.Valid {
		t.Fatalf("secrets = %+v/%+v, want both cleared on sent", got.secretA, got.secretB)
	}
}
