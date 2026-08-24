package payments

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
)

// backoffSchedule mirrors billing's own low-credit outbox (ADR-0010):
// attempt N (1-indexed) waits backoffSchedule[N-1] before retrying, and a
// row whose attempt_count reaches len(backoffSchedule) is dead-lettered
// instead of scheduled again.
var backoffSchedule = []time.Duration{
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	6 * time.Hour,
	18 * time.Hour,
}

// payoutSubject and payoutText are the payout-account-incomplete Platform
// Notification's fixed copy (ADR-0009's content rule: no Client name, no
// Engagement/payment detail, nothing identifying who or what tripped
// it). It deliberately does not name which Stripe fields are
// outstanding -- some are personal (date of birth, SSN last 4) and none
// of that belongs in an email body. payoutLink is the only variable.
const payoutSubject = "Doula Cloud: your Practice's payout account needs more information" //nolint:gosec // payout-account copy, not a credential

func payoutText(payoutLink string) string {
	return "Hello,\n\n" +
		"Stripe needs more information before your Practice's payout account can receive payments.\n\n" +
		"Finish setting it up here:\n" +
		payoutLink + "\n\n" +
		"If you have questions, reply to this email.\n"
}

// QueuePayoutIncompleteNotification inserts a pending payout_outbox row
// for practiceID, unless one is already pending for this episode (the
// partial unique index 00034 added makes the INSERT a no-op in that
// case). Called on the same tx PostAccountWebhookHandler already holds
// for the capability-status update that caused it -- unlike
// billing.QueueOutOfCreditsNotification, there is no rollback this write
// needs to survive, so it does not need its own, separately committed
// write.
func QueuePayoutIncompleteNotification(ctx context.Context, tx *sql.Tx, practiceID string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO payout_outbox (practice_id) VALUES ($1)
		 ON CONFLICT (practice_id) WHERE status = 'pending' DO NOTHING`,
		practiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: queue payout-incomplete notification: %w", err)
	}
	return nil
}

// Worker sends due payout_outbox rows -- the Cloud-Scheduler-driven half
// of ADR-0010's outbox, mirroring billing.Worker's shape.
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

// ProcessPending sends every due payout_outbox row within tx. Each row's
// grace window (00034's 48-hour next_attempt_at default) may have
// already resolved itself by the time it comes due, so this rechecks the
// Practice's live stripe_connect_requirements_due before mailing anyone:
// a Practice with nothing outstanding any more is marked sent with
// nothing to mail, same as a Practice with zero Owners. Owners are
// resolved at send time too (not stored on the row) and mailed
// separately; a row is marked sent once every resolved Owner has been
// mailed without error.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	now := w.Now()

	// The due-check compares against Postgres's own now(), not w.Now(),
	// mirroring billing.Worker.ProcessPending -- see that function's own
	// comment for why.
	rows, err := tx.QueryContext(ctx,
		`SELECT id, practice_id, attempt_count
		 FROM payout_outbox
		 WHERE status = 'pending' AND next_attempt_at <= now()
		 ORDER BY next_attempt_at
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`,
		maxOutboxBatch,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: query pending payout outbox rows: %w", err)
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
			return fmt.Errorf("payments: scan payout outbox row: %w", err)
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return fmt.Errorf("payments: iterate payout outbox rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		// coverage:ignore reason: DB row close failure, not exercised by unit tests
		return fmt.Errorf("payments: close payout outbox rows: %w", err)
	}

	for _, r := range pending {
		stillOutstanding, err := requirementsStillOutstanding(ctx, tx, r.practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
		if !stillOutstanding {
			if err := w.markSent(ctx, tx, r.id, now); err != nil {
				// coverage:ignore reason: DB update failure, not exercised by unit tests
				return err
			}
			continue
		}

		emails, err := ownerEmails(ctx, tx, r.practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}

		link := w.AppBaseURL + "/practices/" + r.practiceID + "/settings/payments"
		var sendErr error
		for _, email := range emails {
			if sendErr = w.Sender.Send(ctx, mail.Message{
				To:      email,
				From:    w.From,
				ReplyTo: w.ReplyTo,
				Subject: payoutSubject,
				Text:    payoutText(link),
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

// requirementsStillOutstanding reports whether practiceID's live
// stripe_connect_requirements_due is non-empty right now. cardinality(),
// not a Scan into []string: database/sql has no array type, and how a
// bare Scan decodes text[] depends on the driver -- avoided the same way
// the account-webhook tests avoid it (see connectState's comment).
func requirementsStillOutstanding(ctx context.Context, tx *sql.Tx, practiceID string) (bool, error) {
	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT cardinality(stripe_connect_requirements_due) FROM practices WHERE id = $1`,
		practiceID,
	).Scan(&count); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("payments: check outstanding requirements: %w", err)
	}
	return count > 0, nil
}

// ownerEmails returns the email of every Staff member holding the owner
// role at practiceID. Relies on 00033's app.notification_worker_trusted
// policies on staff/practice_memberships -- table-generic, so this
// worker reuses them rather than 00034 minting its own.
func ownerEmails(ctx context.Context, tx *sql.Tx, practiceID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT s.email FROM staff s
		 JOIN practice_memberships pm ON pm.staff_id = s.id
		 WHERE pm.practice_id = $1 AND 'owner' = ANY(pm.roles)`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("payments: resolve owner emails: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("payments: scan owner email: %w", err)
		}
		emails = append(emails, email)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("payments: iterate owner emails: %w", err)
	}
	return emails, nil
}

func (w Worker) markSent(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE payout_outbox SET status = 'sent', sent_at = $1, last_error = NULL WHERE id = $2`,
		now, id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("payments: mark payout outbox row sent: %w", err)
	}
	return nil
}

func (w Worker) markFailed(ctx context.Context, tx *sql.Tx, id string, attemptCount int, sendErr error, now time.Time) error {
	nextAttempt := attemptCount + 1
	if nextAttempt >= len(backoffSchedule) {
		if _, err := tx.ExecContext(ctx,
			`UPDATE payout_outbox SET status = 'dead_lettered', attempt_count = $1, last_error = $2 WHERE id = $3`,
			nextAttempt, sendErr.Error(), id,
		); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return fmt.Errorf("payments: dead-letter payout outbox row: %w", err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE payout_outbox SET attempt_count = $1, next_attempt_at = $2, last_error = $3 WHERE id = $4`,
		nextAttempt, now.Add(backoffSchedule[nextAttempt-1]), sendErr.Error(), id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("payments: schedule payout outbox retry: %w", err)
	}
	return nil
}
