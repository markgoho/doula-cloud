package engagementrequest

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
)

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
// Cloud-Scheduler-driven half of ADR-0010's outbox (outbox.ProcessPending
// owns the claim/retry/dead-letter machinery every mail kind shares).
type Worker struct {
	Sender     mail.Sender
	Now        func() time.Time
	AppBaseURL string
	From       string
	ReplyTo    string
}

func (w Worker) inner() outbox.Worker {
	return outbox.Worker{Sender: w.Sender, Now: w.Now, From: w.From, ReplyTo: w.ReplyTo, Table: "engagement_request_outbox"}
}

// pendingRow is one due outbox row joined to the Request it belongs to
// and the recipient it names.
type pendingRow struct {
	id           string
	attemptCount int
	practiceID   string
	state        string
	address      string
}

const claimQuery = `SELECT o.id, o.attempt_count, r.practice_id, r.state::text, s.email
	   FROM engagement_request_outbox o
	   JOIN engagement_requests r ON r.id = o.request_id
	   JOIN staff s ON s.id = o.staff_id
	  WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	  ORDER BY o.next_attempt_at
	  LIMIT $1
	  FOR UPDATE OF o SKIP LOCKED`

func scanRow(rows *sql.Rows) (pendingRow, error) {
	var p pendingRow
	err := rows.Scan(&p.id, &p.attemptCount, &p.practiceID, &p.state, &p.address)
	return p, wrapOutboxErr(err)
}

// wrapOutboxErr gives an error from the outbox package (a sibling
// package, so wrapcheck treats its errors as external) this package's
// own prefix, without outbox's own already-descriptive message.
func wrapOutboxErr(err error) error {
	if err == nil {
		return nil
	}
	// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
	return fmt.Errorf("engagementrequest: %w", err)
}

// ProcessPending sends every due engagement_request_outbox row within tx.
// It joins the Request for its current state and Practice -- a Request
// already decided or withdrawn through some other path before this row
// was sent is never mailed, mirroring offer.Worker's own skip rule.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), claimQuery, scanRow, w.send))
}

// send resolves one pending row: skipped if the Request is no longer
// pending, mailed otherwise, and marked either way.
func (w Worker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, p pendingRow, now time.Time) error {
	if p.state != statePending {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, p.id, now))
	}

	link := w.AppBaseURL + "/practices/" + p.practiceID + "/clients"
	sendErr := w.Sender.Send(ctx, mail.Message{
		To:      p.address,
		From:    w.From,
		ReplyTo: w.ReplyTo,
		Subject: requestSubject,
		Text:    requestText(link),
	})
	if sendErr != nil {
		return wrapOutboxErr(inner.MarkFailed(ctx, tx, p.id, p.attemptCount, sendErr, now))
	}
	return wrapOutboxErr(inner.MarkSent(ctx, tx, p.id, now))
}
