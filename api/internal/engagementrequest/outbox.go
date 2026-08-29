package engagementrequest

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
)

// backoffSchedule mirrors every other outbox in this codebase (ADR-0010):
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

// requestSubject and requestText are the Engagement Request email's fixed
// copy. Content-free per CONTEXT.md/ADR-0017: no kind, due date, or
// Client name, the same restraint engagement_offers_notification_worker
// (00041) already observes -- only a pointer back to the dashboard.
const requestSubject = "Doula Cloud: a new Engagement Request is waiting"

func requestText(link string) string {
	return "Hello,\n\n" +
		"A new Engagement Request is waiting for your decision at your Practice.\n\n" +
		link + "\n\n" +
		"If you have questions, reply to this email.\n"
}

// queueOutbox queues one pending engagement_request_outbox row per Owner
// and Admin at practiceID -- ADR-0017: "there is no single approver ...
// mailing one of them picked by some rule means a Request waits on
// whichever person happens to be away." Must run in the same transaction
// as the Request insert.
func queueOutbox(ctx context.Context, tx *sql.Tx, practiceID, requestID string) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT staff_id FROM practice_memberships
		  WHERE practice_id = $1 AND roles && ARRAY['owner', 'admin']::practice_role[]`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("engagementrequest: resolve owner/admin recipients: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var staffIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return fmt.Errorf("engagementrequest: scan recipient: %w", err)
		}
		staffIDs = append(staffIDs, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return fmt.Errorf("engagementrequest: iterate recipients: %w", err)
	}
	if err := rows.Close(); err != nil {
		// coverage:ignore reason: DB row close failure, not exercised by unit tests
		return fmt.Errorf("engagementrequest: close recipients: %w", err)
	}

	for _, staffID := range staffIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO engagement_request_outbox (request_id, staff_id) VALUES ($1, $2)`,
			requestID, staffID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("engagementrequest: queue outbox row: %w", err)
		}
	}
	return nil
}

// Worker sends due engagement_request_outbox rows -- the
// Cloud-Scheduler-driven half of ADR-0010's outbox, mirroring
// offer.Worker's shape.
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

// pendingRow is one due outbox row joined to the Request it belongs to
// and the recipient it names.
type pendingRow struct {
	id           string
	attemptCount int
	practiceID   string
	state        string
	address      string
}

// ProcessPending sends every due engagement_request_outbox row within tx.
// It joins the Request for its current state and Practice -- a Request
// already decided or withdrawn through some other path before this row
// was sent is never mailed, mirroring offer.Worker's own skip rule.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	now := w.Now()

	rows, err := tx.QueryContext(ctx,
		`SELECT o.id, o.attempt_count, r.practice_id, r.state::text, s.email
		   FROM engagement_request_outbox o
		   JOIN engagement_requests r ON r.id = o.request_id
		   JOIN staff s ON s.id = o.staff_id
		  WHERE o.status = 'pending' AND o.next_attempt_at <= now()
		  ORDER BY o.next_attempt_at
		  LIMIT $1
		  FOR UPDATE OF o SKIP LOCKED`,
		maxOutboxBatch,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("engagementrequest: query pending outbox rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pending []pendingRow
	for rows.Next() {
		var p pendingRow
		if err := rows.Scan(&p.id, &p.attemptCount, &p.practiceID, &p.state, &p.address); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return fmt.Errorf("engagementrequest: scan outbox row: %w", err)
		}
		pending = append(pending, p)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return fmt.Errorf("engagementrequest: iterate outbox rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		// coverage:ignore reason: DB row close failure, not exercised by unit tests
		return fmt.Errorf("engagementrequest: close outbox rows: %w", err)
	}

	for _, p := range pending {
		if err := w.send(ctx, tx, p, now); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return err
		}
	}
	return nil
}

// send resolves one pending row: skipped if the Request is no longer
// pending, mailed otherwise, and marked either way.
func (w Worker) send(ctx context.Context, tx *sql.Tx, p pendingRow, now time.Time) error {
	if p.state != statePending {
		return w.markSent(ctx, tx, p.id, now)
	}

	link := w.AppBaseURL + "/practices/" + p.practiceID + "/clients"
	if sendErr := w.Sender.Send(ctx, mail.Message{
		To:      p.address,
		From:    w.From,
		ReplyTo: w.ReplyTo,
		Subject: requestSubject,
		Text:    requestText(link),
	}); sendErr != nil {
		return w.markFailed(ctx, tx, p.id, p.attemptCount, sendErr, now)
	}
	return w.markSent(ctx, tx, p.id, now)
}

func (w Worker) markSent(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE engagement_request_outbox SET status = 'sent', sent_at = $1, last_error = NULL WHERE id = $2`,
		now, id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("engagementrequest: mark outbox row sent: %w", err)
	}
	return nil
}

func (w Worker) markFailed(ctx context.Context, tx *sql.Tx, id string, attemptCount int, sendErr error, now time.Time) error {
	nextAttempt := attemptCount + 1
	if nextAttempt >= len(backoffSchedule) {
		if _, err := tx.ExecContext(ctx,
			`UPDATE engagement_request_outbox SET status = 'dead_lettered', attempt_count = $1, last_error = $2 WHERE id = $3`,
			nextAttempt, sendErr.Error(), id,
		); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return fmt.Errorf("engagementrequest: dead-letter outbox row: %w", err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE engagement_request_outbox SET attempt_count = $1, next_attempt_at = $2, last_error = $3 WHERE id = $4`,
		nextAttempt, now.Add(backoffSchedule[nextAttempt-1]), sendErr.Error(), id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("engagementrequest: schedule outbox retry: %w", err)
	}
	return nil
}
