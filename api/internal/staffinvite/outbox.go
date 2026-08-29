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
	"doula-cloud/api/internal/outbox"
)

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

// Refresh replaces the token on a pending staff_invite_outbox row for
// invitationID, and does nothing if there is no pending row. It exists
// for the Offer flow (#317): an Offer to an email address rotates the
// Invitation's token, which would leave a Staff invitation email still
// waiting in this outbox holding a token that no longer opens anything.
// Refresh keeps that row mailable without queueing a second Notification
// -- the Offer's own email carries the same link, so a fresh Queue here
// would mail the same person twice for one event.
func Refresh(ctx context.Context, tx *sql.Tx, invitationID, token string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE staff_invite_outbox SET invite_token = $2 WHERE invitation_id = $1 AND status = 'pending'`,
		invitationID, token,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffinvite: refresh outbox token: %w", err)
	}
	return nil
}

// Worker sends due staff_invite_outbox rows -- the Cloud-Scheduler-driven
// half of ADR-0010's outbox (outbox.ProcessPending owns the claim/retry/
// dead-letter machinery every mail kind shares).
type Worker struct {
	Sender     mail.Sender
	Now        func() time.Time
	AppBaseURL string
	From       string
	ReplyTo    string
}

// invite_token is cleared on both sent and dead-lettered terminal states,
// not only on sent: this table's whole justification for holding
// plaintext at all is that its exposure window is "queued but not yet
// sent", and a dead-lettered row is done retrying too.
func (w Worker) inner() outbox.Worker {
	return outbox.Worker{
		Sender: w.Sender, Now: w.Now, From: w.From, ReplyTo: w.ReplyTo,
		Table:           "staff_invite_outbox",
		ClearOnTerminal: []string{"invite_token"},
	}
}

type pendingRow struct {
	id           string
	attemptCount int
	inviteToken  sql.NullString
	address      string
	status       string
	expiresAt    time.Time
}

const claimQuery = `SELECT o.id, o.attempt_count, o.invite_token, pi.address, pi.status, pi.expires_at
	 FROM staff_invite_outbox o
	 JOIN practice_invitations pi ON pi.id = o.invitation_id
	 WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	 ORDER BY o.next_attempt_at
	 LIMIT $1
	 FOR UPDATE OF o SKIP LOCKED`

func scanRow(rows *sql.Rows) (pendingRow, error) {
	var r pendingRow
	err := rows.Scan(&r.id, &r.attemptCount, &r.inviteToken, &r.address, &r.status, &r.expiresAt)
	return r, wrapOutboxErr(err)
}

// wrapOutboxErr gives an error from the outbox package (a sibling
// package, so wrapcheck treats its errors as external) this package's
// own prefix, without outbox's own already-descriptive message.
func wrapOutboxErr(err error) error {
	if err == nil {
		return nil
	}
	// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
	return fmt.Errorf("staffinvite: %w", err)
}

// ProcessPending sends every due staff_invite_outbox row within tx: it
// joins practice_invitations for the recipient's address and current
// status at send time, so an Invitation accepted, revoked, or expired
// through some other path before this row was sent is never mailed. The
// mailable token itself comes from the outbox row, not the join --
// practice_invitations only ever holds its digest (00030) -- and is
// cleared once sent, keeping its plaintext exposure window to "queued
// but not yet sent".
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), claimQuery, scanRow, w.send))
}

func (w Worker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingRow, now time.Time) error {
	// Not (still) 'pending', or past its own expires_at -- accepted,
	// revoked, or expired, whether or not something has gotten around to
	// flipping the status column yet -- either way, nothing to deliver.
	if r.status != "pending" || !r.expiresAt.After(now) {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
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
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
	}
	return wrapOutboxErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, sendErr, now))
}
