package portalinvite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
)

// backoffSchedule is ADR-0010's "roughly five attempts... over about a
// day": attempt N (1-indexed) waits backoffSchedule[N-1] before retrying.
// A row whose attempt_count reaches len(backoffSchedule) is dead-lettered
// instead of scheduled again.
var backoffSchedule = []time.Duration{
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	6 * time.Hour,
	18 * time.Hour,
}

// inviteSubject and inviteText are the Client portal invite's fixed,
// content-free copy (ADR-0009's rule, unconditional per #221: no Client
// name, no Engagement detail, and -- for v1 -- no Practice name either,
// anywhere in subject or body). link is the only variable.
const inviteSubject = "You have something waiting online"

func inviteText(link string) string {
	return "Hello,\n\n" +
		"You've been invited to view your care details online.\n\n" +
		link + "\n\n" +
		"If you weren't expecting this, you can safely ignore this email.\n"
}

// Worker sends due portal_invite_outbox rows -- the Cloud-Scheduler-driven
// half of ADR-0010's outbox, mirroring push.Pusher's real/fake split via
// the injected mail.Sender. Now is injectable so retry/dead-letter tests
// don't depend on a real clock.
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

// ProcessPending sends every due portal_invite_outbox row within tx: it
// joins client_portal_users (and clients, for the recipient address) to
// read the *current* invite_token and email at send time, so a re-invite
// that rotated the token after this row was queued is never mailed
// stale. A row whose invite was already accepted before send is marked
// sent without mailing anything -- the Client already has access.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	now := w.Now()

	// The due-check compares against Postgres's own now(), not w.Now():
	// next_attempt_at's default (queueOutboxSend's INSERT) is also
	// Postgres's clock, and even a few milliseconds of skew against the
	// Go process's clock could make a row queued this instant look not
	// yet due to a w.Now()-based comparison run immediately after.
	rows, err := tx.QueryContext(ctx,
		`SELECT o.id, o.attempt_count, pu.invite_token, pu.identity_uid, c.email
		 FROM portal_invite_outbox o
		 JOIN client_portal_users pu ON pu.id = o.client_portal_user_id
		 JOIN clients c ON c.id = pu.client_id
		 WHERE o.status = 'pending' AND o.next_attempt_at <= now()
		 ORDER BY o.next_attempt_at
		 LIMIT $1
		 FOR UPDATE OF o SKIP LOCKED`,
		maxOutboxBatch,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("portalinvite: query pending outbox rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type pendingRow struct {
		id           string
		attemptCount int
		inviteToken  sql.NullString
		identityUID  sql.NullString
		email        string
	}
	var pending []pendingRow
	for rows.Next() {
		var r pendingRow
		if err := rows.Scan(&r.id, &r.attemptCount, &r.inviteToken, &r.identityUID, &r.email); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return fmt.Errorf("portalinvite: scan outbox row: %w", err)
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return fmt.Errorf("portalinvite: iterate outbox rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		// coverage:ignore reason: DB row close failure, not exercised by unit tests
		return fmt.Errorf("portalinvite: close outbox rows: %w", err)
	}

	for _, r := range pending {
		if r.identityUID.Valid {
			// Already accepted -- through some other path -- before this
			// row got sent. Nothing to deliver; record it sent so it
			// never retries.
			if err := w.markSent(ctx, tx, r.id, now); err != nil {
				// coverage:ignore reason: DB update failure, not exercised by unit tests
				return err
			}
			continue
		}

		link := w.AppBaseURL + "/portal/accept-invite?token=" + r.inviteToken.String
		sendErr := w.Sender.Send(ctx, mail.Message{
			To:      r.email,
			From:    w.From,
			ReplyTo: w.ReplyTo,
			Subject: inviteSubject,
			Text:    inviteText(link),
		})
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

// queueOutboxSend inserts a pending portal_invite_outbox row for
// portalUserID, or -- if one is already pending (a re-invite) -- resets
// its attempt_count and next_attempt_at so the worker retries
// immediately. It never stores the invite_token itself; ProcessPending
// reads that fresh from client_portal_users at send time, so a rotation
// after this call is queued is always picked up, not mailed stale.
func queueOutboxSend(ctx context.Context, tx *sql.Tx, portalUserID string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO portal_invite_outbox (client_portal_user_id) VALUES ($1)
		 ON CONFLICT (client_portal_user_id) WHERE status = 'pending'
		 DO UPDATE SET attempt_count = 0, next_attempt_at = now(), last_error = NULL`,
		portalUserID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("portalinvite: queue outbox send: %w", err)
	}
	return nil
}

func (w Worker) markSent(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE portal_invite_outbox SET status = 'sent', sent_at = $1, last_error = NULL WHERE id = $2`,
		now, id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("portalinvite: mark outbox row sent: %w", err)
	}
	return nil
}

func (w Worker) markFailed(ctx context.Context, tx *sql.Tx, id string, attemptCount int, sendErr error, now time.Time) error {
	nextAttempt := attemptCount + 1
	if nextAttempt >= len(backoffSchedule) {
		if _, err := tx.ExecContext(ctx,
			`UPDATE portal_invite_outbox SET status = 'dead_lettered', attempt_count = $1, last_error = $2 WHERE id = $3`,
			nextAttempt, sendErr.Error(), id,
		); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return fmt.Errorf("portalinvite: dead-letter outbox row: %w", err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE portal_invite_outbox SET attempt_count = $1, next_attempt_at = $2, last_error = $3 WHERE id = $4`,
		nextAttempt, now.Add(backoffSchedule[nextAttempt-1]), sendErr.Error(), id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("portalinvite: schedule outbox retry: %w", err)
	}
	return nil
}
