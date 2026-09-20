package oncall

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/outbox"
)

// gapNoticeSubject and gapNoticeText are the Notification's fixed copy.
// ADR-0009's content rule is unconditional and ADR-0011 removed its one
// exception: no Practice name, no Client, no Doula, no date -- nothing
// identifying, in From, subject or body. Who cannot be reached, on which
// birth and when, is behind the Practice's own sign-in, one link away.
const gapNoticeSubject = "Doula Cloud: a birth needs on-call cover"

func gapNoticeText(link string) string {
	return "Hello,\n\n" +
		"Someone at your Practice recorded a time when a doula on call cannot be reached, and nobody is covering it yet.\n\n" +
		"See who is on call:\n" +
		link + "\n\n" +
		"If you have questions, reply to this email.\n"
}

// GapNoticeWorker sends due coverage_gap_outbox rows -- the
// Cloud-Scheduler-driven half of ADR-0010's outbox for #1093.
//
// Hand-written rather than built on outbox.MailWorker for the reason
// payments.ConnectNudgeWorker is: it mails every Owner and Admin the
// Practice has at send time through SendAll, not one fixed address.
type GapNoticeWorker struct {
	outbox.Mailer
}

func (w GapNoticeWorker) inner() outbox.Worker {
	return w.Worker("coverage_gap_outbox")
}

type gapNoticeRow struct {
	id, practiceID, gapID string
	attemptCount          int
}

const gapNoticeClaimQuery = `SELECT id, practice_id, gap_id, attempt_count
	 FROM coverage_gap_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanGapNoticeRow(rows *sql.Rows) (gapNoticeRow, error) {
	var r gapNoticeRow
	if err := rows.Scan(&r.id, &r.practiceID, &r.gapID, &r.attemptCount); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return r, fmt.Errorf("oncall: %w", err)
	}
	return r, nil
}

// ProcessPending sends every due coverage_gap_outbox row within tx.
func (w GapNoticeWorker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	if err := outbox.ProcessPending(ctx, tx, w.inner(), gapNoticeClaimQuery, scanGapNoticeRow, w.send); err != nil {
		// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
		return fmt.Errorf("oncall: %w", err)
	}
	return nil
}

// send mails one claimed row, rechecking the gap first. A gap that has
// since been covered, cleared, or has ended -- a retry after a Mailgun
// failure can be most of a day later -- is marked sent with nothing
// mailed, rather than asking every Owner to cover a hole that is gone.
//
// The recheck reads engagement_coverage_gaps with no Practice set, which
// is what 00114's engagement_coverage_gaps_notification_worker policy
// admits under the door this outbox is registered with.
func (w GapNoticeWorker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r gapNoticeRow, now time.Time) error {
	var stillOpen bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM engagement_coverage_gaps
		    WHERE id = $1 AND cleared_at IS NULL AND covering_staff_id IS NULL AND ends_at > now())`,
		r.gapID,
	).Scan(&stillOpen); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("oncall: recheck coverage gap: %w", err)
	}
	if !stillOpen {
		return wrap(inner.MarkSent(ctx, tx, r.id, now))
	}

	staffIDs, emails, err := ownerAndAdminRecipients(ctx, tx, r.practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	// Written before the send for connect_nudge's reason: SendAll marks
	// the row terminal itself, and a retry overwrites this with what that
	// attempt actually addressed.
	if staffIDs == nil {
		staffIDs = []string{}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE coverage_gap_outbox SET notified_staff_ids = $2::uuid[] WHERE id = $1`, r.id, staffIDs,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("oncall: record coverage gap recipients: %w", err)
	}
	link := w.AppBaseURL + "/practices/" + r.practiceID + "/on-call"
	return wrap(inner.SendAll(ctx, tx, r.id, r.attemptCount, now, emails, gapNoticeSubject, gapNoticeText(link)))
}

// ownerAndAdminRecipients resolves every current Owner and Admin at send
// time, as payments' ownerEmails does for #343, with the staff ids
// beside the addresses so the row can say whom it addressed. Relies on
// 00033's app.notification_worker_trusted policies on staff and
// practice_memberships.
func ownerAndAdminRecipients(ctx context.Context, tx *sql.Tx, practiceID string) (staffIDs, emails []string, err error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT s.id, s.email FROM staff s
		 JOIN practice_memberships pm ON pm.staff_id = s.id
		 WHERE pm.practice_id = $1 AND (pm.roles && ARRAY['owner', 'admin']::practice_role[])
		 ORDER BY s.id`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, nil, fmt.Errorf("oncall: resolve owner and admin recipients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, nil, fmt.Errorf("oncall: scan recipient: %w", err)
		}
		staffIDs = append(staffIDs, id)
		emails = append(emails, email)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: cursor failure, not exercised by unit tests
		return nil, nil, fmt.Errorf("oncall: read recipients: %w", err)
	}
	return staffIDs, emails, nil
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
	return fmt.Errorf("oncall: %w", err)
}
