// Package staffinvite is the Staff invitation Notification (RA-G1, #339,
// ADR-0010, map #213) -- the outbox and worker that mail a
// practice_invitations row's accept link, mirroring portalinvite's
// shape. No handler in this codebase yet writes practice_invitations
// (#316 builds InviteHandler/accept); Queue is the seam that handler
// calls once it exists, in the same transaction as the invite/rotate.
package staffinvite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
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

// staffInviteSubject and staffInviteText are the Staff invitation's
// fixed, content-free copy. Platform voice (ADR-0009): the invited
// person is told she has been invited to join a practice on Doula
// Cloud, never which one, and no Client name or Engagement detail
// (neither applies here regardless). link is the only variable.
const staffInviteSubject = "You've been invited to join a practice on Doula Cloud"

func staffInviteText(link string) string {
	return "Hello,\n\n" +
		"You've been invited to join a practice on Doula Cloud.\n\n" +
		link + "\n\n" +
		"If you weren't expecting this, you can safely ignore this email.\n"
}

// Queue queues a pending staff_invite_outbox send for invitationID, or --
// if one is already pending (a re-invite) -- overwrites its token and
// resets attempt_count/next_attempt_at so the worker retries
// immediately. Callers pass token in the clear: practice_invitations
// (00030) stores only its digest, so this row is the only place the
// worker can read a live, mailable token from at send time. Must run in
// the same transaction as the practice_invitations insert/rotate that
// minted token, mirroring portalinvite.queueOutboxSend's shape.
func Queue(ctx context.Context, tx *sql.Tx, invitationID, token string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO staff_invite_outbox (invitation_id, invite_token) VALUES ($1, $2)
		 ON CONFLICT (invitation_id) WHERE status = 'pending'
		 DO UPDATE SET invite_token = $2, attempt_count = 0, next_attempt_at = now(), last_error = NULL`,
		invitationID, token,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffinvite: queue outbox send: %w", err)
	}
	return nil
}

// Worker sends due staff_invite_outbox rows -- the Cloud-Scheduler-driven
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

// ProcessPending sends every due staff_invite_outbox row within tx: it
// joins practice_invitations for the recipient's address and current
// status at send time, so an Invitation accepted, revoked, or expired
// through some other path before this row was sent is never mailed. The
// mailable token itself comes from the outbox row, not the join --
// practice_invitations only ever holds its digest (00030) -- and is
// cleared once sent, keeping its plaintext exposure window to "queued
// but not yet sent".
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	now := w.Now()

	// The due-check compares against Postgres's own now(), not w.Now(),
	// mirroring portalinvite.Worker.ProcessPending's reasoning.
	rows, err := tx.QueryContext(ctx,
		`SELECT o.id, o.attempt_count, o.invite_token, pi.address, pi.status, pi.expires_at
		 FROM staff_invite_outbox o
		 JOIN practice_invitations pi ON pi.id = o.invitation_id
		 WHERE o.status = 'pending' AND o.next_attempt_at <= now()
		 ORDER BY o.next_attempt_at
		 LIMIT $1
		 FOR UPDATE OF o SKIP LOCKED`,
		maxOutboxBatch,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffinvite: query pending outbox rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type pendingRow struct {
		id           string
		attemptCount int
		inviteToken  sql.NullString
		address      string
		status       string
		expiresAt    time.Time
	}
	var pending []pendingRow
	for rows.Next() {
		var r pendingRow
		if err := rows.Scan(&r.id, &r.attemptCount, &r.inviteToken, &r.address, &r.status, &r.expiresAt); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return fmt.Errorf("staffinvite: scan outbox row: %w", err)
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return fmt.Errorf("staffinvite: iterate outbox rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		// coverage:ignore reason: DB row close failure, not exercised by unit tests
		return fmt.Errorf("staffinvite: close outbox rows: %w", err)
	}

	for _, r := range pending {
		// Not (still) 'pending', or past its own expires_at -- accepted,
		// revoked, or expired, whether or not something has gotten
		// around to flipping the status column yet -- either way,
		// nothing to deliver.
		if r.status != "pending" || !r.expiresAt.After(now) {
			if err := w.markSent(ctx, tx, r.id, now); err != nil {
				// coverage:ignore reason: DB update failure, not exercised by unit tests
				return err
			}
			continue
		}

		link := w.AppBaseURL + "/accept-invite?token=" + r.inviteToken.String
		sendErr := w.Sender.Send(ctx, mail.Message{
			To:      r.address,
			From:    w.From,
			ReplyTo: w.ReplyTo,
			Subject: staffInviteSubject,
			Text:    staffInviteText(link),
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

func (w Worker) markSent(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE staff_invite_outbox SET status = 'sent', sent_at = $1, last_error = NULL, invite_token = NULL WHERE id = $2`,
		now, id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("staffinvite: mark outbox row sent: %w", err)
	}
	return nil
}

func (w Worker) markFailed(ctx context.Context, tx *sql.Tx, id string, attemptCount int, sendErr error, now time.Time) error {
	nextAttempt := attemptCount + 1
	if nextAttempt >= len(backoffSchedule) {
		// invite_token is cleared here too, not just by markSent: a
		// dead-lettered row is done retrying, and this package's whole
		// justification for holding a plaintext token at all is that its
		// exposure window is "queued but not yet sent" -- a row that will
		// never be sent doesn't get to keep it either.
		if _, err := tx.ExecContext(ctx,
			`UPDATE staff_invite_outbox SET status = 'dead_lettered', attempt_count = $1, last_error = $2, invite_token = NULL WHERE id = $3`,
			nextAttempt, sendErr.Error(), id,
		); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return fmt.Errorf("staffinvite: dead-letter outbox row: %w", err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE staff_invite_outbox SET attempt_count = $1, next_attempt_at = $2, last_error = $3 WHERE id = $4`,
		nextAttempt, now.Add(backoffSchedule[nextAttempt-1]), sendErr.Error(), id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("staffinvite: schedule outbox retry: %w", err)
	}
	return nil
}
