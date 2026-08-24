package billing

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/tasknudge"
)

// backoffSchedule mirrors portalinvite's outbox (ADR-0010): attempt N
// (1-indexed) waits backoffSchedule[N-1] before retrying, and a row whose
// attempt_count reaches len(backoffSchedule) is dead-lettered instead of
// scheduled again.
var backoffSchedule = []time.Duration{
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	6 * time.Hour,
	18 * time.Hour,
}

// lowCreditSubject and lowCreditText are the out-of-Credits Platform
// Notification's fixed copy (ADR-0009's content rule: no Client name, no
// Engagement detail, no Staff member -- nothing identifying who tripped
// the wall or which Client caused it). billingLink is the only variable.
const lowCreditSubject = "Doula Cloud: your Practice is out of Credits" //nolint:gosec // billing Credits copy, not a credential

func lowCreditText(billingLink string) string {
	return "Hello,\n\n" +
		"Your Practice has used all of its Credits. Each new Client uses one Credit, " +
		"and no new Client can be added until your Practice has more.\n\n" +
		"Buy more Credits here:\n" +
		billingLink + "\n\n" +
		"If you have questions, reply to this email.\n"
}

// ShouldQueueOutOfCreditsNotification reports whether QueueOutOfCreditsNotification
// should run for practiceID: true unless a low_credit_outbox row already
// exists for this Practice from the current "out of Credits" episode --
// one created after the Practice's most recent 'purchase' credit_ledger
// row (or, if it has never purchased, any row at all). It must run on
// the caller's own tx, before that tx is rolled back: credit_ledger is
// practice-tier RLS, and this is the last point in
// engagement.CreateHandler's request where app.current_practice_id is
// still set.
func ShouldQueueOutOfCreditsNotification(ctx context.Context, tx *sql.Tx, practiceID string) (bool, error) {
	var should bool
	err := tx.QueryRowContext(ctx,
		`SELECT NOT EXISTS (
			SELECT 1 FROM low_credit_outbox o
			WHERE o.practice_id = $1
			AND o.created_at > COALESCE(
				(SELECT MAX(created_at) FROM credit_ledger WHERE practice_id = $1 AND origin = 'purchase'),
				'-infinity'::timestamptz
			)
		)`,
		practiceID,
	).Scan(&should)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("billing: check out-of-credits notification dedupe: %w", err)
	}
	return should, nil
}

// QueueOutOfCreditsNotification inserts a pending low_credit_outbox row
// for practiceID. Unlike ConsumeCredit, it runs on db, not the caller's
// tx: the caller reaches this only after ErrNoCreditsRemaining, once
// that tx has already been rolled back (its Client/Engagement inserts
// must not survive), so the queued row needs its own, separately
// committed write to survive that rollback. ON CONFLICT DO NOTHING
// guards the race between two concurrent requests that both observed a
// zero balance before either's rollback ran. enq is ADR-0013's Cloud
// Tasks nudge; unlike portalinvite.InviteHandler and
// staffauth.EndSessionsHandler, this write commits on its own (the
// ExecContext above, autocommitted), so the nudge fires immediately
// rather than through tasknudge.Register/Drain -- there is no later
// commit to wait for. Nudging even when ON CONFLICT no-oped (the row was
// already pending) is harmless: the worker just processes a row that was
// already due.
func QueueOutOfCreditsNotification(ctx context.Context, db *sql.DB, practiceID string, enq tasknudge.Enqueuer) error {
	if _, err := db.ExecContext(ctx,
		`INSERT INTO low_credit_outbox (practice_id) VALUES ($1)
		 ON CONFLICT (practice_id) WHERE status = 'pending' DO NOTHING`,
		practiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("billing: queue out-of-credits notification: %w", err)
	}
	tasknudge.Fire(enq, tasknudge.LowCredit)(ctx)
	return nil
}

// Worker sends due low_credit_outbox rows -- the Cloud-Scheduler-driven
// half of ADR-0010's outbox, mirroring portalinvite.Worker's shape.
type Worker struct {
	Sender     mail.Sender
	Now        func() time.Time
	AppBaseURL string
	From       string
	ReplyTo    string
}

// maxOutboxBatch bounds how many rows one ProcessPending call sends, so a
// large backlog can't turn a single Scheduler tick into an unbounded
// transaction.
const maxOutboxBatch = 100

// ProcessPending sends every due low_credit_outbox row within tx,
// resolving the Practice's current Owners at send time (not stored on
// the row) and mailing each of them separately. A row is marked sent
// once every resolved Owner has been mailed without error; a Practice
// with zero Owners at send time is marked sent with nothing to mail --
// there is no one left to notify.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	now := w.Now()

	// The due-check compares against Postgres's own now(), not w.Now():
	// next_attempt_at's default (QueueOutOfCreditsNotification's INSERT)
	// is also Postgres's clock, and even a few milliseconds of skew
	// against the Go process's clock could make a row queued this
	// instant look not yet due to a w.Now()-based comparison run
	// immediately after -- mirrors portalinvite.Worker.ProcessPending.
	rows, err := tx.QueryContext(ctx,
		`SELECT id, practice_id, attempt_count
		 FROM low_credit_outbox
		 WHERE status = 'pending' AND next_attempt_at <= now()
		 ORDER BY next_attempt_at
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`,
		maxOutboxBatch,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("billing: query pending low-credit outbox rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type pendingRow struct {
		id           string
		practiceID   string
		attemptCount int
	}
	var pending []pendingRow
	for rows.Next() {
		var r pendingRow
		if err := rows.Scan(&r.id, &r.practiceID, &r.attemptCount); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return fmt.Errorf("billing: scan low-credit outbox row: %w", err)
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return fmt.Errorf("billing: iterate low-credit outbox rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		// coverage:ignore reason: DB row close failure, not exercised by unit tests
		return fmt.Errorf("billing: close low-credit outbox rows: %w", err)
	}

	for _, r := range pending {
		emails, err := ownerEmails(ctx, tx, r.practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}

		link := w.AppBaseURL + "/practices/" + r.practiceID + "/billing"
		var sendErr error
		for _, email := range emails {
			if sendErr = w.Sender.Send(ctx, mail.Message{
				To:      email,
				From:    w.From,
				ReplyTo: w.ReplyTo,
				Subject: lowCreditSubject,
				Text:    lowCreditText(link),
			}); sendErr != nil {
				break
			}
		}

		if sendErr == nil {
			if err := w.markSent(ctx, tx, r.id, now); err != nil {
				// coverage:ignore reason: DB update failure, not exercised by unit tests
				return err
			}
			continue
		}
		if err := w.markFailed(ctx, tx, r.id, r.attemptCount, sendErr, now); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return err
		}
	}
	return nil
}

// ownerEmails returns the email of every Staff member holding the owner
// role at practiceID.
func ownerEmails(ctx context.Context, tx *sql.Tx, practiceID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT s.email FROM staff s
		 JOIN practice_memberships pm ON pm.staff_id = s.id
		 WHERE pm.practice_id = $1 AND 'owner' = ANY(pm.roles)`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: resolve owner emails: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("billing: scan owner email: %w", err)
		}
		emails = append(emails, email)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: iterate owner emails: %w", err)
	}
	return emails, nil
}

func (w Worker) markSent(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE low_credit_outbox SET status = 'sent', sent_at = $1, last_error = NULL WHERE id = $2`,
		now, id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("billing: mark low-credit outbox row sent: %w", err)
	}
	return nil
}

func (w Worker) markFailed(ctx context.Context, tx *sql.Tx, id string, attemptCount int, sendErr error, now time.Time) error {
	nextAttempt := attemptCount + 1
	if nextAttempt >= len(backoffSchedule) {
		if _, err := tx.ExecContext(ctx,
			`UPDATE low_credit_outbox SET status = 'dead_lettered', attempt_count = $1, last_error = $2 WHERE id = $3`,
			nextAttempt, sendErr.Error(), id,
		); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return fmt.Errorf("billing: dead-letter low-credit outbox row: %w", err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE low_credit_outbox SET attempt_count = $1, next_attempt_at = $2, last_error = $3 WHERE id = $4`,
		nextAttempt, now.Add(backoffSchedule[nextAttempt-1]), sendErr.Error(), id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("billing: schedule low-credit outbox retry: %w", err)
	}
	return nil
}
