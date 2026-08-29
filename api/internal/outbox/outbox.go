// Package outbox is the shared durable-mail-dispatch machinery ADR-0010
// and ADR-0013 describe once but this codebase used to implement eight
// times over: pending row -> backoff -> batch claim under
// FOR UPDATE SKIP LOCKED -> mark sent/failed/dead-lettered -> HTTP send
// wrapper. Every mail kind (portalinvite, billing's low-credit,
// payments' payout and payment-received, sessionnotice, staffinvite,
// offer, engagementrequest) keeps its own outbox table, claim query, row
// shape, recipient resolution, copy, and skip-at-send recheck -- those
// are what actually differ between kinds, so they stay in each package.
// What was identical eight times over -- the backoff schedule, the batch
// cap, the claim-scan-close-then-send loop, marking a row sent/failed/
// dead-lettered, and the Cloud-Scheduler-facing HTTP handler -- lives
// here once.
package outbox

import (
	"context"
	"database/sql"
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

// MaxBatch bounds how many rows one ProcessPending call sends, so a large
// backlog can't turn a single Scheduler tick into an unbounded
// transaction.
const MaxBatch = 100

// Worker holds what every mail kind's worker needs to mark a row sent,
// failed, or dead-lettered and to mail it. Table drives every UPDATE this
// package issues; it is always a compile-time constant a package passes
// in, never request data, so building it into SQL via fmt.Sprintf carries
// no injection risk. ClearOnTerminal names outbox columns to null out
// once a row leaves 'pending' for good (sent, or dead-lettered) -- only
// staffinvite and offer need this, for the plaintext invite tokens/access
// codes their outbox rows hold that practice_invitations/engagement_offers
// only ever store a digest of; every other kind leaves it nil.
type Worker struct {
	Sender          mail.Sender
	Now             func() time.Time
	From            string
	ReplyTo         string
	Table           string
	ClearOnTerminal []string
}

func (w Worker) clearClause() string {
	var b strings.Builder
	for _, col := range w.ClearOnTerminal {
		b.WriteString(", ")
		b.WriteString(col)
		b.WriteString(" = NULL")
	}
	return b.String()
}

// MarkSent records id as delivered.
func (w Worker) MarkSent(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	//nolint:gosec // w.Table is a compile-time constant each package supplies, never request data
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET status = 'sent', sent_at = $1, last_error = NULL%s WHERE id = $2`, w.Table, w.clearClause()),
		now, id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("outbox: mark %s row sent: %w", w.Table, err)
	}
	return nil
}

// MarkDeadLetteredNow dead-letters id outright, with no further retry --
// for a row whose own data means a retry could never succeed (e.g.
// portalinvite's Client with no email on file), unlike MarkFailed's
// transient-send-error retries.
func (w Worker) MarkDeadLetteredNow(ctx context.Context, tx *sql.Tx, id, reason string) error {
	//nolint:gosec // w.Table is a compile-time constant each package supplies, never request data
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET status = 'dead_lettered', last_error = $1%s WHERE id = $2`, w.Table, w.clearClause()),
		reason, id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("outbox: dead-letter %s row: %w", w.Table, err)
	}
	return nil
}

// MarkFailed schedules id's next retry per BackoffSchedule, or
// dead-letters it once attemptCount has exhausted the schedule.
func (w Worker) MarkFailed(ctx context.Context, tx *sql.Tx, id string, attemptCount int, sendErr error, now time.Time) error {
	nextAttempt := attemptCount + 1
	if nextAttempt >= len(BackoffSchedule) {
		//nolint:gosec // w.Table is a compile-time constant each package supplies, never request data
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`UPDATE %s SET status = 'dead_lettered', attempt_count = $1, last_error = $2%s WHERE id = $3`, w.Table, w.clearClause()),
			nextAttempt, sendErr.Error(), id,
		); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return fmt.Errorf("outbox: dead-letter %s row: %w", w.Table, err)
		}
		return nil
	}
	//nolint:gosec // w.Table is a compile-time constant each package supplies, never request data
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET attempt_count = $1, next_attempt_at = $2, last_error = $3 WHERE id = $4`, w.Table),
		nextAttempt, now.Add(BackoffSchedule[nextAttempt-1]), sendErr.Error(), id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("outbox: schedule %s retry: %w", w.Table, err)
	}
	return nil
}

// SendAll mails the same subject/text to every address, stopping at the
// first failure, and marks id accordingly -- sent once every attempted
// address succeeded (or addresses is empty: a Practice with nobody left
// to notify, e.g. billing/payments' owner-resolution kinds, is marked
// sent with nothing to mail), failed otherwise. Only the multi-recipient
// kinds (billing's low-credit, payments' payout and payment-received) use
// this; single-recipient kinds call w.Sender.Send directly.
func (w Worker) SendAll(ctx context.Context, tx *sql.Tx, id string, attemptCount int, now time.Time, addresses []string, subject, text string) error {
	var sendErr error
	for _, addr := range addresses {
		if sendErr = w.Sender.Send(ctx, mail.Message{
			To:      addr,
			From:    w.From,
			ReplyTo: w.ReplyTo,
			Subject: subject,
			Text:    text,
		}); sendErr != nil {
			break
		}
	}
	if sendErr == nil {
		return w.MarkSent(ctx, tx, id, now)
	}
	return w.MarkFailed(ctx, tx, id, attemptCount, sendErr, now)
}

// ProcessPending claims every row claimQuery selects (a query that must
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
// rule; it is enforced by convention in each caller's SQL, not by this
// function.
func ProcessPending[R any](
	ctx context.Context,
	tx *sql.Tx,
	w Worker,
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

	for _, r := range pending {
		if err := handle(ctx, tx, w, r, now); err != nil {
			return err
		}
	}
	return nil
}
