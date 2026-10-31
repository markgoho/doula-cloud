// Package welcomecredits is #1381: one email to a Practice that signed
// up and started no Engagement, fourteen days after signup, and nothing
// after. Decided on #1372 (Q3, Q10, P3) and recorded in ADR-0043's third
// section: her signup is the act and the date is its consequence, so the
// row is enqueued in the signup transaction with its fire time set there
// -- the day-23 deletion reminder's shape (ADR-0031) -- and nothing scans
// for quiet Practices (ADR-0038).
//
// The worker rechecks the Practice live when the row comes due and skips,
// marking the row sent with nothing mailed and the reason on the row,
// when the Practice has an Engagement, holds a founding grant, is
// deleting or deleted, has no Owner, or every Owner's address is
// suppressed (ADR-0029). Recipients are every current Owner, resolved at
// send time and written back to the row (ADR-0035's
// notified_owner_staff_ids). A send records one Activity row with
// DoulaCloud as the actor; a skip records none.
package welcomecredits

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/outbox"
)

// Delay is how long after signup the message fires.
const Delay = 14 * 24 * time.Hour

const table = "welcome_credits_outbox"

// Enqueue writes the Practice's one row, due Delay after now, on the
// signup transaction.
func Enqueue(ctx context.Context, tx *sql.Tx, practiceID string, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO welcome_credits_outbox (practice_id, next_attempt_at, created_at) VALUES ($1, $2, $3)`,
		practiceID, now.Add(Delay), now,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("welcomecredits: enqueue: %w", err)
	}
	return nil
}

//nolint:gosec // email copy, not a credential
const subject = "DoulaCloud: your 3 Welcome credits are still there"

func text(link string) string {
	return "Hello,\n\n" +
		"Your Practice on DoulaCloud is two weeks old, and its 3 Welcome credits are still there.\n\n" +
		"Adding a Client is free. Starting work with a Client uses 1 Credit.\n\n" +
		"Start with a Client here:\n" +
		link + "\n\n" +
		"This is the only email about it. If you have questions, reply to this email.\n"
}

type skipReason string

const (
	skipEngagementStarted skipReason = "engagement_started"
	skipFoundingGrant     skipReason = "founding_grant"
	skipPracticeDeleting  skipReason = "practice_deleting"
	skipOwnersSuppressed  skipReason = "owners_suppressed"
	skipNoOwners          skipReason = "no_owners"
)

// Worker sends due welcome_credits_outbox rows. Hand-written rather than
// outbox.MailWorker[R] because it mails every current Owner through
// SendAll, as practicedeletion.Worker's reminder does.
type Worker struct {
	outbox.Mailer
}

func (w Worker) inner() outbox.Worker {
	return w.Worker(table)
}

type pendingRow struct {
	id           string
	practiceID   string
	attemptCount int
}

const claimQuery = `SELECT id, practice_id, attempt_count
	 FROM welcome_credits_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanRow(rows *sql.Rows) (pendingRow, error) {
	var r pendingRow
	if err := rows.Scan(&r.id, &r.practiceID, &r.attemptCount); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return r, fmt.Errorf("welcomecredits: scan outbox row: %w", err)
	}
	return r, nil
}

// ProcessPending sends every due row within tx.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	if err := w.inner().HandlePending(ctx, tx, claimQuery, scanRow, w.send); err != nil {
		// coverage:ignore reason: only reached by a DB failure, not exercised by unit tests
		return fmt.Errorf("welcomecredits: %w", err)
	}
	return nil
}

func (w Worker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingRow, now time.Time) error {
	// engagements and credit_ledger are practice-scoped under RLS, and the
	// worker's transaction carries no Practice otherwise.
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, r.practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("welcomecredits: set current practice id: %w", err)
	}

	reason, err := practiceSkip(ctx, tx, r.practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}

	var staffIDs, emails []string
	if reason == "" {
		staffIDs, emails, err = ownerRecipients(ctx, tx, r.practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
		reason, err = recipientSkip(ctx, tx, emails)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
	}

	if reason != "" {
		return markSkipped(ctx, tx, inner, r.id, reason, now)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE welcome_credits_outbox SET notified_owner_staff_ids = $2::uuid[] WHERE id = $1`,
		r.id, staffIDs,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("welcomecredits: record recipients: %w", err)
	}

	link := w.AppBaseURL + "/practices/" + r.practiceID
	if err := inner.SendAll(ctx, tx, r.id, r.attemptCount, now, emails, subject, text(link)); err != nil {
		// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
		return fmt.Errorf("welcomecredits: %w", err)
	}

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status::text FROM welcome_credits_outbox WHERE id = $1`, r.id).Scan(&status); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("welcomecredits: read status: %w", err)
	}
	if status != "sent" {
		return nil
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  r.practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   r.practiceID,
		Action:      string(activity.ActionWelcomeCreditsEmailSent),
		Actor:       activity.SystemActor(),
		CreatedAt:   now,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("welcomecredits: record send: %w", err)
	}
	return nil
}

func markSkipped(ctx context.Context, tx *sql.Tx, inner outbox.Worker, id string, reason skipReason, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE welcome_credits_outbox SET skip_reason = $2 WHERE id = $1`, id, string(reason),
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("welcomecredits: record skip: %w", err)
	}
	if err := inner.MarkSent(ctx, tx, id, now); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("welcomecredits: %w", err)
	}
	return nil
}

// practiceSkip answers the three skip conditions the Practice's own
// records decide, or "" when none holds.
func practiceSkip(ctx context.Context, tx *sql.Tx, practiceID string) (skipReason, error) {
	var deleting, engaged, founding bool
	if err := tx.QueryRowContext(ctx,
		`SELECT
			p.deletion_requested_at IS NOT NULL OR p.deleted_at IS NOT NULL,
			EXISTS (SELECT 1 FROM engagements e WHERE e.practice_id = p.id),
			EXISTS (SELECT 1 FROM credit_ledger c WHERE c.practice_id = p.id AND c.origin = 'founding_grant')
		 FROM practices p WHERE p.id = $1`,
		practiceID,
	).Scan(&deleting, &engaged, &founding); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("welcomecredits: recheck practice: %w", err)
	}
	switch {
	case deleting:
		return skipPracticeDeleting, nil
	case founding:
		return skipFoundingGrant, nil
	case engaged:
		return skipEngagementStarted, nil
	}
	return "", nil
}

// recipientSkip answers the two conditions the resolved Owners decide:
// nobody to write to, or nobody whose address may be written to.
func recipientSkip(ctx context.Context, tx *sql.Tx, emails []string) (skipReason, error) {
	if len(emails) == 0 {
		return skipNoOwners, nil
	}
	var allSuppressed bool
	if err := tx.QueryRowContext(ctx,
		`SELECT bool_and(EXISTS (
			SELECT 1 FROM email_suppressions es
			WHERE es.address = lower(btrim(a)) AND es.cleared_at IS NULL
		 )) FROM unnest($1::text[]) AS a`,
		emails,
	).Scan(&allSuppressed); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("welcomecredits: read suppressions: %w", err)
	}
	if allSuppressed {
		return skipOwnersSuppressed, nil
	}
	return "", nil
}

func ownerRecipients(ctx context.Context, tx *sql.Tx, practiceID string) (staffIDs, emails []string, err error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT s.id, s.email FROM staff s
		 JOIN practice_memberships pm ON pm.staff_id = s.id
		 WHERE pm.practice_id = $1 AND 'owner' = ANY(pm.roles)
		 ORDER BY s.id`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, nil, fmt.Errorf("welcomecredits: resolve owners: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, nil, fmt.Errorf("welcomecredits: scan owner: %w", err)
		}
		staffIDs = append(staffIDs, id)
		emails = append(emails, email)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return nil, nil, fmt.Errorf("welcomecredits: iterate owners: %w", err)
	}
	return staffIDs, emails, nil
}
