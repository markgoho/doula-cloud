package payments

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
)

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
// of ADR-0010's outbox (outbox.ProcessPending owns the claim/retry/
// dead-letter machinery every mail kind shares).
type Worker struct {
	Sender     mail.Sender
	Now        func() time.Time
	AppBaseURL string
	From       string
	ReplyTo    string
}

func (w Worker) inner() outbox.Worker {
	return outbox.Worker{Sender: w.Sender, Now: w.Now, From: w.From, ReplyTo: w.ReplyTo, Table: "payout_outbox"}
}

type payoutPendingRow struct {
	id           string
	practiceID   string
	attemptCount int
}

const payoutClaimQuery = `SELECT id, practice_id, attempt_count
	 FROM payout_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanPayoutRow(rows *sql.Rows) (payoutPendingRow, error) {
	var r payoutPendingRow
	err := rows.Scan(&r.id, &r.practiceID, &r.attemptCount)
	return r, wrapOutboxErr(err)
}

// wrapOutboxErr gives an error from the outbox package (a sibling
// package, so wrapcheck treats its errors as external) this package's
// own prefix, without outbox's own already-descriptive message. Shared
// by both Worker (payout_outbox) and PaymentReceivedWorker
// (payment_outbox.go) -- one prefix for both, since it's a package-level
// concern, not a per-table one.
func wrapOutboxErr(err error) error {
	if err == nil {
		return nil
	}
	// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
	return fmt.Errorf("payments: %w", err)
}

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
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), payoutClaimQuery, scanPayoutRow, w.send))
}

func (w Worker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r payoutPendingRow, now time.Time) error {
	stillOutstanding, err := requirementsStillOutstanding(ctx, tx, r.practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if !stillOutstanding {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
	}

	emails, err := ownerEmails(ctx, tx, r.practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	link := w.AppBaseURL + "/practices/" + r.practiceID + "/settings/payments"
	return wrapOutboxErr(inner.SendAll(ctx, tx, r.id, r.attemptCount, now, emails, payoutSubject, payoutText(link)))
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
