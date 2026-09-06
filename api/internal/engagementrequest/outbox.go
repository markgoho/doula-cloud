package engagementrequest

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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

// Worker sends due engagement_request_outbox rows through
// outbox.MailWorker's shared claim-scan-compose-send-mark loop -- this
// kind's own share is only its table, claim query, row shape and Compose
// below.
type Worker = outbox.MailWorker[pendingRow]

// NewWorker builds the Engagement Request outbox worker around mailer.
func NewWorker(mailer outbox.Mailer) Worker {
	return Worker{
		Mailer:     mailer,
		Table:      "engagement_request_outbox",
		ClaimQuery: claimQuery,
		Scan:       scanRow,
		Compose:    compose(mailer),
	}
}

// pendingRow is one due outbox row joined to the Request it belongs to
// and the recipient it names.
type pendingRow struct {
	practiceID string
	state      string
	address    string
}

const claimQuery = `SELECT o.id, o.attempt_count, r.practice_id, r.state::text, s.email
	   FROM engagement_request_outbox o
	   JOIN engagement_requests r ON r.id = o.request_id
	   JOIN staff s ON s.id = o.staff_id
	  WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	  ORDER BY o.next_attempt_at
	  LIMIT $1
	  FOR UPDATE OF o SKIP LOCKED`

func scanRow(rows *sql.Rows) (outbox.RowMeta, pendingRow, error) {
	var meta outbox.RowMeta
	var p pendingRow
	if err := rows.Scan(&meta.ID, &meta.AttemptCount, &p.practiceID, &p.state, &p.address); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return meta, p, fmt.Errorf("engagementrequest: scan outbox row: %w", err)
	}
	return meta, p, nil
}

// compose joins the Request for its current state and Practice at send
// time, so a Request already decided or withdrawn through some other
// path before this row was sent resolves to ErrAlreadyDone, mirroring
// offer's own skip rule.
func compose(mailer outbox.Mailer) func(context.Context, *sql.Tx, pendingRow, time.Time) (string, string, string, error) {
	return func(_ context.Context, _ *sql.Tx, p pendingRow, _ time.Time) (string, string, string, error) {
		if p.state != statePending {
			return "", "", "", outbox.ErrAlreadyDone
		}
		link := mailer.AppBaseURL + "/practices/" + p.practiceID + "/clients"
		return p.address, requestSubject, requestText(link), nil
	}
}
