package payments

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
)

// paymentReceivedSubject and paymentReceivedText are the "a Payment
// arrived" Platform Notification's fixed copy. ADR-0009 calls a
// Notification "content-free... point[ing] at the product rather than
// carry[ing] it", and both shipped sibling notifications
// (billing.lowCreditText, payoutText) are link-only with no interpolated
// business data -- the ticket's own content-rule sketch hedged amount/
// date with "if that", and this body takes the same link-only reading
// rather than the permissive one, matching precedent.
const paymentReceivedSubject = "Doula Cloud: a Payment arrived"

// link points at the Practice dashboard, not a payment list -- no
// Practice-wide "all Payments" screen exists yet (a Payment renders only
// inline on its own Engagement's Contract page), so the copy says "sign
// in" rather than promising a specific view.
func paymentReceivedText(link string) string {
	return "Hello,\n\n" +
		"A Payment arrived for your Practice.\n\n" +
		"Sign in to Doula Cloud to see the details:\n" +
		link + "\n"
}

// QueuePaymentReceivedNotification inserts a pending
// payment_received_outbox row for paymentID, copying practiceID onto the
// row since PaymentReceivedWorker.ProcessPending runs with no Practice
// session and cannot re-resolve it later. Called on the same tx
// handleInvoicePaid already holds for the payments-row insert that
// caused it -- unlike billing.QueueOutOfCreditsNotification, there is no
// rollback this write needs to survive.
func QueuePaymentReceivedNotification(ctx context.Context, tx *sql.Tx, paymentID, practiceID string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO payment_received_outbox (payment_id, practice_id) VALUES ($1, $2)`,
		paymentID, practiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: queue payment-received notification: %w", err)
	}
	return nil
}

// PaymentReceivedWorker sends due payment_received_outbox rows -- the
// Cloud-Scheduler-driven half of ADR-0010's outbox for #344, mirroring
// Worker's shape (payout_outbox's own worker, kept as a distinct type
// since the two outbox tables, recipients, and copy are all unrelated).
// outbox.ProcessPending owns the claim/retry/dead-letter machinery every
// mail kind shares.
type PaymentReceivedWorker struct {
	Sender     mail.Sender
	Now        func() time.Time
	AppBaseURL string
	From       string
	ReplyTo    string
}

func (w PaymentReceivedWorker) inner() outbox.Worker {
	return outbox.Worker{Sender: w.Sender, Now: w.Now, From: w.From, ReplyTo: w.ReplyTo, Table: "payment_received_outbox"}
}

type paymentReceivedPendingRow struct {
	id           string
	practiceID   string
	attemptCount int
}

const paymentReceivedClaimQuery = `SELECT id, practice_id, attempt_count
	 FROM payment_received_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanPaymentReceivedRow(rows *sql.Rows) (paymentReceivedPendingRow, error) {
	var r paymentReceivedPendingRow
	err := rows.Scan(&r.id, &r.practiceID, &r.attemptCount)
	return r, wrapOutboxErr(err)
}

// ProcessPending sends every due payment_received_outbox row within tx.
// Unlike payout_outbox's worker, there is no live state to recheck at
// send time -- a Payment that already arrived cannot un-arrive -- so
// every due row with at least one Owner/Admin recipient is mailed and
// marked sent. Recipients are Owner and Admin (ADR-0006/ADR-0008's read
// table: Contract money and Invoice history), not Owner-only like
// payout_outbox's and low_credit_outbox's workers, whose notifications
// map to Owner-only responsive actions this one has none of.
func (w PaymentReceivedWorker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), paymentReceivedClaimQuery, scanPaymentReceivedRow, w.send))
}

func (w PaymentReceivedWorker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r paymentReceivedPendingRow, now time.Time) error {
	emails, err := ownerAndAdminEmails(ctx, tx, r.practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	link := w.AppBaseURL + "/practices/" + r.practiceID
	return wrapOutboxErr(inner.SendAll(ctx, tx, r.id, r.attemptCount, now, emails, paymentReceivedSubject, paymentReceivedText(link)))
}

// ownerAndAdminEmails returns the email of every Staff member holding
// the owner or admin role at practiceID -- distinct from ownerEmails
// (payout_outbox's resolver), which must stay Owner-only for #343's
// notification. Relies on the same 00033 app.notification_worker_trusted
// policies ownerEmails does.
func ownerAndAdminEmails(ctx context.Context, tx *sql.Tx, practiceID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT s.email FROM staff s
		 JOIN practice_memberships pm ON pm.staff_id = s.id
		 WHERE pm.practice_id = $1 AND (pm.roles && ARRAY['owner', 'admin']::practice_role[])`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("payments: resolve owner/admin emails: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("payments: scan owner/admin email: %w", err)
		}
		emails = append(emails, email)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("payments: iterate owner/admin emails: %w", err)
	}
	return emails, nil
}
