package outbox

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"doula-cloud/api/internal/mail"
)

// Mailer is the sending identity a mail-kind worker used to redeclare as its
// own {Sender, Now, AppBaseURL, From, ReplyTo} struct -- thirteen times over,
// each just copying the same five fields into an outbox.Worker. main.go now
// builds it once (plus one derived copy for portalinvite's Practice-voice
// ReplyTo, ADR-0011), and every worker -- scaffolded or hand-written --
// holds one of these instead of its own five fields.
type Mailer struct {
	Sender     mail.Sender
	Now        func() time.Time
	AppBaseURL string
	From       string
	ReplyTo    string
}

// Worker builds the outbox.Worker m's Mark* methods run against, for table
// and (optionally) the columns to clear on a terminal row. The one line a
// hand-written kind's own inner() now takes instead of copying five fields.
func (m Mailer) Worker(table string, clearOnTerminal ...string) Worker {
	return Worker{
		Sender: m.Sender, Now: m.Now, From: m.From, ReplyTo: m.ReplyTo,
		Table: table, ClearOnTerminal: clearOnTerminal,
	}
}

// ErrAlreadyDone is what a MailWorker's Compose returns instead of a
// message when the row's own event already resolved through some other
// path -- an invitation already accepted, a code already verified. Nothing
// to deliver, but not a failure: ProcessPending marks the row sent with no
// send attempted, so it never retries.
var ErrAlreadyDone = errors.New("outbox: nothing to send, row already resolved")

// DeadLetterError is what a MailWorker's Compose returns instead of a
// message when no retry could help: a missing address, no Identity
// Platform account for the row's identity, or the row's own missing
// token. ProcessPending dead-letters the row on the spot with Reason,
// skipping BackoffSchedule entirely -- the same treatment MarkFailed
// already gives mail.ErrSuppressed (ADR-0029), generalized to every kind's
// own permanent-refusal case.
//
// Built as a literal (&DeadLetterError{Reason: "..."}), never through a
// constructor function: a Compose closure returning one is then a value
// construction, not a call into this package, so it carries no wrapcheck
// obligation the way calling an exported outbox function would.
type DeadLetterError struct {
	Reason string
}

func (e *DeadLetterError) Error() string { return e.Reason }

// RowMeta is the id/attempt_count pair every outbox row scan already read
// out by hand for its own Mark* calls. A MailWorker's Scan returns it
// alongside the kind's own row shape, so Compose itself never has to know
// either field.
type RowMeta struct {
	ID           string
	AttemptCount int
}

// claimedRow pairs a claimed row's RowMeta with the kind-specific data
// Compose reads -- the single value outbox.ProcessPending's existing
// claim-scan-handle loop is instantiated over. Unexported: nothing outside
// this file ever names it.
type claimedRow[R any] struct {
	meta RowMeta
	row  R
}

// MailWorker owns the claim-scan-compose-send-mark loop ADR-0010 describes
// once, that a single-recipient mail kind used to hand-write as its own
// inner(), pendingRow, scanRow, wrapOutboxErr and ProcessPending. A kind now
// names only what actually differs: its table, its claim query, its row
// shape (via Scan) and its Compose. It is built on top of the existing
// Worker and ProcessPending, not a replacement for either -- the claim
// query shape, the backoff schedule, and the Mark* methods are untouched.
//
// Compose decides what happens to one claimed row: return a message to
// send; ErrAlreadyDone if the row's own event already resolved elsewhere
// (marked sent, nothing mailed); a *DeadLetterError if no retry could help
// (dead-lettered on the spot); or any other error to retry per
// BackoffSchedule -- the same three outcomes a hand-written send() used to
// reach by calling inner.MarkSent, inner.MarkDeadLetteredNow and
// inner.MarkFailed directly. ctx and tx are threaded through Compose, not
// just the row and now, because two migrated kinds need them:
// authmail.TokenMailWorker and mfarecoverymail.Worker each resolve the
// recipient's *current* address via an authn.AccountManager call, and
// mfarecoverymail also reads a subject's name with a query against tx.
type MailWorker[R any] struct {
	Mailer          Mailer
	Table           string
	ClearOnTerminal []string
	ClaimQuery      string
	Scan            func(*sql.Rows) (RowMeta, R, error)
	Compose         func(ctx context.Context, tx *sql.Tx, row R, now time.Time) (to, subject, text string, err error)
}

func (w MailWorker[R]) inner() Worker {
	return w.Mailer.Worker(w.Table, w.ClearOnTerminal...)
}

func (w MailWorker[R]) scan(rows *sql.Rows) (claimedRow[R], error) {
	meta, row, err := w.Scan(rows)
	return claimedRow[R]{meta: meta, row: row}, err
}

// ProcessPending sends every due row within tx: claims, scans, composes and
// marks each one -- what a hand-written kind's own ProcessPending used to
// do by calling outbox.ProcessPending directly, now done once here.
func (w MailWorker[R]) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return ProcessPending(ctx, tx, w.inner(), w.ClaimQuery, w.scan, w.handle)
}

func (w MailWorker[R]) handle(ctx context.Context, tx *sql.Tx, inner Worker, c claimedRow[R], now time.Time) error {
	to, subject, text, err := w.Compose(ctx, tx, c.row, now)
	if errors.Is(err, ErrAlreadyDone) {
		return inner.MarkSent(ctx, tx, c.meta.ID, now)
	}
	if err != nil {
		var dl *DeadLetterError
		if errors.As(err, &dl) {
			return inner.MarkDeadLetteredNow(ctx, tx, c.meta.ID, dl.Reason)
		}
		return inner.MarkFailed(ctx, tx, c.meta.ID, c.meta.AttemptCount, err, now)
	}
	sendErr := w.Mailer.Sender.Send(ctx, mail.Message{
		To: to, From: w.Mailer.From, ReplyTo: w.Mailer.ReplyTo, Subject: subject, Text: text,
	})
	if sendErr != nil {
		return inner.MarkFailed(ctx, tx, c.meta.ID, c.meta.AttemptCount, sendErr, now)
	}
	return inner.MarkSent(ctx, tx, c.meta.ID, now)
}
