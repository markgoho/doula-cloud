// Package outbox is the shared durable-mail-dispatch machinery ADR-0010
// and ADR-0013 describe once but this codebase used to implement eight
// times over: pending row -> backoff -> batch claim under
// FOR UPDATE SKIP LOCKED -> mark sent/skipped/failed/dead-lettered -> HTTP
// send wrapper. lifecycle.go declares the row's states and the moves
// between them once; every Mark* method here is one of those moves. Every mail kind (portalinvite, billing's low-credit,
// payments' payout and payment-received, sessionnotice, staffinvite,
// offer, engagementrequest) keeps its own outbox table, claim query, row
// shape, recipient resolution, copy, and skip-at-send recheck -- those
// are what actually differ between kinds, so they stay in each package.
// What was identical eight times over -- the backoff schedule, the batch
// cap, the claim-scan-close-then-send loop, marking a row sent/failed/
// dead-lettered, and the Cloud-Scheduler-facing HTTP handler -- lives
// here once.
//
// registry.go carries the second half of that consolidation. An outbox
// used to restate its own identity in seven places, only one of which
// held any behavior; a Registration is now the one value that names its
// path, its Postgres door, its ADR-0013 nudge target and its worker, and
// Register mounts the lot. The two non-mail outboxes ride the same
// machinery because the shape is the same: #394's Client erasure, which
// calls Stripe and Identity Platform, and #443's site rebuild, which is
// also the one outbox that needs no door.
package outbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"doula-cloud/api/internal/mail"
)

// BackoffSchedule is ADR-0010's "roughly five attempts... over about a
// day": attempt N (1-indexed) waits BackoffSchedule[N-1] before retrying.
// A row whose attempt_count reaches len(BackoffSchedule) is dead-lettered
// instead of scheduled again.
var BackoffSchedule = []time.Duration{
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	6 * time.Hour,
	18 * time.Hour,
}

// DeadLetterReason is the outbox_dead_letter_reason a dead-lettered row
// carries (ADR-0062).
type DeadLetterReason string

// RetriesExhausted is MarkFailed's, and is alerted on; OwnData is every
// row dead-lettered at once because its own data meant no send could
// succeed, and is only shown.
const (
	RetriesExhausted DeadLetterReason = "retries_exhausted"
	OwnData          DeadLetterReason = "own_data"
)

// MaxBatch bounds how many rows one ProcessPending call sends, so a large
// backlog can't turn a single Scheduler tick into an unbounded
// transaction.
const MaxBatch = 100

// Worker holds what every mail kind's worker needs to move a row through
// the lifecycle lifecycle.go declares and to mail it. Table drives every
// UPDATE this package issues; it is always a compile-time constant a
// package passes in, never request data, so building it into SQL via
// fmt.Sprintf carries no injection risk. ClearOnTerminal names outbox
// columns to null out once a row leaves 'pending' for good (sent,
// skipped, or dead-lettered) -- the plaintext tokens and access codes a
// few kinds' rows hold, of which the record they serve only ever stores
// a digest; every other kind leaves it nil.
type Worker struct {
	Sender          mail.Sender
	Now             func() time.Time
	From            string
	ReplyTo         string
	Table           string
	ClearOnTerminal []string
}

// apply runs t against id: it sets t.To (when the move changes the
// state), the move's own columns in set (placeholders $1.. in order,
// filled from args), and on an exit from pending the ClearOnTerminal
// columns, all only WHERE the row is still in t.From. A statement that
// moved no row is ErrNotInState, never silently done.
func (w Worker) apply(ctx context.Context, tx *sql.Tx, t Transition, id string, set []string, args ...any) error {
	if t.To != t.From {
		set = append([]string{fmt.Sprintf("status = '%s'", t.To)}, set...)
	}
	if t.LeavesPending() {
		for _, col := range w.ClearOnTerminal {
			set = append(set, col+" = NULL")
		}
	}
	//nolint:gosec // query is built from w.Table, a compile-time constant each package supplies, and this file's own column names and declared states -- never request data
	query := fmt.Sprintf(`UPDATE %s SET %s WHERE id = $%d AND status = '%s'`,
		w.Table, strings.Join(set, ", "), len(args)+1, t.From)
	res, err := tx.ExecContext(ctx, query, append(args, id)...)
	if err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("outbox: %s %s row: %w", t.Name, w.Table, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: driver failure, not exercised by unit tests
		return fmt.Errorf("outbox: %s %s row: %w", t.Name, w.Table, err)
	}
	if affected != 1 {
		return fmt.Errorf("outbox: %s %s row %s: %w", t.Name, w.Table, id, ErrNotInState)
	}
	return nil
}

// MarkSent records id as sent: mailed, or its job done.
func (w Worker) MarkSent(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	return w.apply(ctx, tx, TransitionSend, id, []string{"sent_at = $1", "last_error = NULL"}, now)
}

// MarkSkipped records id as resolved with nothing sent, and why: the
// send-time recheck, or the act that queued it, found nothing left to do.
// It is what a row used to be marked sent for, and is told apart from a
// send now (00169).
func (w Worker) MarkSkipped(ctx context.Context, tx *sql.Tx, id string, reason SkipReason, now time.Time) error {
	return w.apply(ctx, tx, TransitionSkip, id, []string{"skip_reason = $1", "skipped_at = $2", "last_error = NULL"}, reason, now)
}

// MarkDeadLetteredNow dead-letters id outright, with no further retry --
// for a row whose own data means a retry could never succeed (e.g.
// portalinvite's Client with no email on file), unlike MarkFailed's
// transient-send-error retries.
func (w Worker) MarkDeadLetteredNow(ctx context.Context, tx *sql.Tx, id, reason string) error {
	return w.apply(ctx, tx, TransitionDeadLetter, id, []string{"dead_letter_reason = $1", "last_error = $2"}, OwnData, reason)
}

// MarkReported moves a sent id to what Mailgun later reported of it,
// t being TransitionBounce or TransitionComplain. Any other transition is
// refused before the database is asked.
func (w Worker) MarkReported(ctx context.Context, tx *sql.Tx, id string, t Transition) error {
	if t != TransitionBounce && t != TransitionComplain {
		return fmt.Errorf("outbox: %s is not a delivery report", t.Name)
	}
	return w.apply(ctx, tx, t, id, nil)
}

// LockPending runs query, which selects the id of each row of w.Table to
// resolve and takes its lock (FOR UPDATE), and returns those ids. It is
// how an act outside a worker -- a login deletion, an edit that revokes
// an invitation -- reaches the rows queued for it, so that it moves each
// through the same declared transition a worker would, ClearOnTerminal
// included, rather than with a status UPDATE of its own. The owning
// package writes query, as it writes its claim query.
func (w Worker) LockPending(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("outbox: lock pending %s rows: %w", w.Table, err)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("outbox: scan %s id: %w", w.Table, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("outbox: iterate %s ids: %w", w.Table, err)
	}
	return ids, nil
}

// SkipPending marks every row LockPending's query selects skipped, with
// reason.
func (w Worker) SkipPending(ctx context.Context, tx *sql.Tx, query string, reason SkipReason, now time.Time, args ...any) error {
	ids, err := w.LockPending(ctx, tx, query, args...)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := w.MarkSkipped(ctx, tx, id, reason, now); err != nil {
			return err
		}
	}
	return nil
}

// DeadLetterPending dead-letters every row LockPending's query selects,
// outright, with reason as MarkDeadLetteredNow records it.
func (w Worker) DeadLetterPending(ctx context.Context, tx *sql.Tx, query, reason string, args ...any) error {
	ids, err := w.LockPending(ctx, tx, query, args...)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := w.MarkDeadLetteredNow(ctx, tx, id, reason); err != nil {
			return err
		}
	}
	return nil
}

// MarkFailed schedules id's next retry per BackoffSchedule, or
// dead-letters it once attemptCount has exhausted the schedule.
//
// A mail.ErrSuppressed failure skips the schedule entirely and
// dead-letters on the spot (ADR-0029): the address is on the suppression
// list, so five more attempts over about a day could only produce the
// same refusal, and Mailgun would decline the send server-side even if
// this process did not. Every mail kind reaches this one branch, which is
// why the guard lives in a mail.Sender decorator rather than in eleven
// separate claim queries.
func (w Worker) MarkFailed(ctx context.Context, tx *sql.Tx, id string, attemptCount int, sendErr error, now time.Time) error {
	if errors.Is(sendErr, mail.ErrSuppressed) {
		return w.MarkDeadLetteredNow(ctx, tx, id, sendErr.Error())
	}
	nextAttempt := attemptCount + 1
	if nextAttempt >= len(BackoffSchedule) {
		return w.apply(ctx, tx, TransitionDeadLetter, id,
			[]string{"dead_letter_reason = $1", "attempt_count = $2", "last_error = $3"},
			RetriesExhausted, nextAttempt, sendErr.Error())
	}
	return w.apply(ctx, tx, TransitionRetry, id,
		[]string{"attempt_count = $1", "next_attempt_at = $2", "last_error = $3"},
		nextAttempt, now.Add(BackoffSchedule[nextAttempt-1]), sendErr.Error())
}

// SendAll mails the same subject/text to every address, stopping at the
// first failure, and marks id accordingly -- sent once every attempted
// address succeeded, skipped (SkipNoRecipients) when addresses is empty
// (a Practice with nobody left to notify, e.g. billing/payments'
// owner-resolution kinds), failed otherwise. Only the multi-recipient
// kinds (billing's low-credit, payments' payout and payment-received) use
// this; single-recipient kinds call w.Sender.Send directly.
//
// A suppressed address (ADR-0029) is skipped rather than treated as a
// failure: one owner who complained must not cost the Practice's other
// owners a Notification they never objected to. Only if *every* address
// was suppressed does the row dead-letter, so the reason is on the row
// instead of the send looking like a success with nothing sent.
func (w Worker) SendAll(ctx context.Context, tx *sql.Tx, id string, attemptCount int, now time.Time, addresses []string, subject, text string) error {
	var sendErr error
	var suppressedErr error
	suppressed := 0
	for _, addr := range addresses {
		err := w.Sender.Send(ctx, mail.Message{
			To:      addr,
			From:    w.From,
			ReplyTo: w.ReplyTo,
			Subject: subject,
			Text:    text,
		})
		if errors.Is(err, mail.ErrSuppressed) {
			suppressed++
			suppressedErr = err
			continue
		}
		if err != nil {
			sendErr = err
			break
		}
	}
	if sendErr != nil {
		return w.MarkFailed(ctx, tx, id, attemptCount, sendErr, now)
	}
	if suppressed > 0 && suppressed == len(addresses) {
		return w.MarkDeadLetteredNow(ctx, tx, id, suppressedErr.Error())
	}
	if len(addresses) == 0 {
		return w.MarkSkipped(ctx, tx, id, SkipNoRecipients, now)
	}
	return w.MarkSent(ctx, tx, id, now)
}

// HandlePending claims every row claimQuery selects (a query that must
// end in "LIMIT $1 FOR UPDATE [OF ...] SKIP LOCKED" over exactly one
// outbox table, ordered by next_attempt_at, taking MaxBatch as its one
// placeholder), scans each with scan, and -- only once the claiming
// cursor is fully read and closed, so Sender.Send never runs while it's
// still open -- calls handle once per row with w and a shared now.
//
// The claim query compares against Postgres's own now(), not w.Now():
// next_attempt_at's default is also Postgres's clock, and even a few
// milliseconds of skew against the Go process's clock could make a row
// queued this instant look not yet due to a w.Now()-based comparison run
// immediately after. Every claimQuery this package receives follows that
// rule by spelling its predicate as ClaimDue or ClaimDueAliasO
// (lifecycle.go), and lifecycle_test.go fails on a claim that restates
// the predicate instead.
//
// #1992 weighed moving the claims to w.Now() and kept them here. The
// reason above still holds for a row's first attempt, which every
// immediately-due outbox takes from its column's DEFAULT now(); the
// re-enqueues that reset next_attempt_at = now() keep it for the same
// reason. Under a simulation run the two clocks agree anyway: sim.now()
// and the BFF's clock both add sim.offset_row's delta (#1833). Moving
// would mean every enqueue writing next_attempt_at from the Go clock,
// every claim and reset taking it as a parameter, and docs/testing/api.md's
// due-time fixture rule turned round. A retry is the exception: MarkFailed
// writes next_attempt_at from w.Now() plus at least five minutes, so a
// retry already meets the database's clock, with a margin far larger than
// any skew. So does a row whose enqueue sets a later due time itself
// (payments.QueuePayoutIncompleteNotification's 48 hours).
func (w Worker) HandlePending[R any](
	ctx context.Context,
	tx *sql.Tx,
	claimQuery string,
	scan func(*sql.Rows) (R, error),
	handle func(ctx context.Context, tx *sql.Tx, w Worker, row R, now time.Time) error,
) error {
	now := w.Now()

	rows, err := tx.QueryContext(ctx, claimQuery, MaxBatch)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("outbox: query pending %s rows: %w", w.Table, err)
	}
	defer func() { _ = rows.Close() }()

	var pending []R
	for rows.Next() {
		r, err := scan(rows)
		if err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return fmt.Errorf("outbox: scan %s row: %w", w.Table, err)
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return fmt.Errorf("outbox: iterate %s rows: %w", w.Table, err)
	}
	if err := rows.Close(); err != nil {
		// coverage:ignore reason: DB row close failure, not exercised by unit tests
		return fmt.Errorf("outbox: close %s rows: %w", w.Table, err)
	}

	ctx = mail.WithTx(ctx, tx)
	for _, r := range pending {
		if err := handle(ctx, tx, w, r, now); err != nil {
			return err
		}
	}
	return nil
}
