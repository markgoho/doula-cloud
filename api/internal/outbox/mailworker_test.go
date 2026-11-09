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

const (
	composeSend       = "send"
	composeAlready    = "already"
	composeDeadLetter = "deadletter"
	composeBoom       = "boom"
)

// newMailWorker builds a MailWorker[mwRow] whose Compose branches on
// outcome: composeSend returns a message to r.to, composeAlready returns
// ErrAlreadyDone, composeDeadLetter returns a *DeadLetterError, and
// anything else is returned as a plain retryable error.
func newMailWorker(sender mail.Sender, outcome, to string) outbox.MailWorker[mwRow] {
	return outbox.MailWorker[mwRow]{
		Mailer:     outbox.Mailer{Sender: sender, Now: time.Now, From: testFrom, ReplyTo: testReplyTo},
		Table:      testTable,
		ClaimQuery: testClaimQuery,
		Scan:       scanMWRow(outcome, to),
		Compose: func(_ context.Context, _ *sql.Tx, r mwRow, _ time.Time) (string, string, string, error) {
			switch r.outcome {
			case composeSend:
				return r.to, "subject", "text", nil
			case composeAlready:
				return "", "", "", outbox.ErrAlreadyDone
			case composeDeadLetter:
				return "", "", "", &outbox.DeadLetterError{Reason: testDeadReason}
			default:
				return "", "", "", errors.New("compose boom")
			}
		},
	}
}

func TestDeadLetterError_ErrorReturnsReason(t *testing.T) {
	err := &outbox.DeadLetterError{Reason: testDeadReason}
	if err.Error() != testDeadReason {
		t.Fatalf("Error() = %q, want %q", err.Error(), testDeadReason)
	}
}

func TestMailWorker_Outcomes(t *testing.T) {
	cases := []struct {
		name          string
		compose       string
		sender        *scriptSender
		clear         []string
		wantDelivered int
		want          rowWant
	}{
		{
			name:          "compose sends and marks sent",
			compose:       composeSend,
			sender:        &scriptSender{},
			wantDelivered: 1,
			want:          rowWant{status: testStatusSent},
		},
		{
			name:    "already done marks sent without sending",
			compose: composeAlready,
			sender:  &scriptSender{},
			want:    rowWant{status: testStatusSent},
		},
		{
			name:    "a DeadLetterError dead-letters on the spot",
			compose: composeDeadLetter,
			sender:  &scriptSender{},
			want:    rowWant{status: testStatusDead, reason: outbox.OwnData, lastError: testDeadReason},
		},
		{
			// With the claim-scan-compose-send-mark loop inside this
			// package, a kind's own Compose returning a plain error crosses
			// no package boundary that needs a wrapOutboxErr of its own:
			// the retry is scheduled directly, storing Compose's own error
			// text on the row unwrapped.
			name:    "a plain Compose error schedules a retry with its own text",
			compose: composeBoom,
			sender:  &scriptSender{},
			want:    rowWant{status: testStatusPending, attempts: 1, lastError: "compose boom"},
		},
		{
			name:          "a send failure schedules a retry",
			compose:       composeSend,
			sender:        &scriptSender{failAll: true},
			wantDelivered: 1,
			want:          rowWant{status: testStatusPending, attempts: 1},
		},
		{
			// A suppressed address (ADR-0029) dead-letters on the spot, the
			// same as every other kind that sends through the wrapped
			// Sender main.go constructs: MailWorker never special-cases
			// mail.ErrSuppressed itself; it reaches the same MarkFailed.
			name:    "a suppressed address dead-letters through the wrapped sender",
			compose: composeSend,
			sender:  &scriptSender{refuse: refusing(recipient)},
			want:    rowWant{status: testStatusDead, reason: outbox.OwnData},
		},
		{
			name:          "marking sent clears the configured terminal columns",
			compose:       composeSend,
			sender:        &scriptSender{},
			clear:         []string{secretA, secretB},
			wantDelivered: 1,
			want:          rowWant{status: testStatusSent, cleared: []string{secretA, secretB}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := newRowEnv(t, 0)
			w := newMailWorker(c.sender, c.compose, recipient)
			w.ClearOnTerminal = c.clear

			env.inTx(func(tx *sql.Tx) error { return w.ProcessPending(t.Context(), tx) })

			if len(c.sender.sent) != c.wantDelivered {
				t.Errorf("sender saw %d messages, want %d", len(c.sender.sent), c.wantDelivered)
			}
			if c.wantDelivered > 0 && c.sender.sent[0].To != recipient {
				t.Errorf("first message to %q, want %q", c.sender.sent[0].To, recipient)
			}
			env.expect(c.want)
		})
	}
}
