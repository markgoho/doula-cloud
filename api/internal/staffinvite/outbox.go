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

	"doula-cloud/api/internal/outbox"
)

// staffInviteSubject and staffInviteText are the Staff invitation's
// fixed, content-free copy. Platform voice (ADR-0009): the invited
// person is told she has been invited to join a practice on Doula
// Cloud, never which one, and no Client name or Engagement detail
// (neither applies here regardless). link is the only variable.
const staffInviteSubject = "You've been invited to join a practice on Doula Cloud"

// invitationStatusPending mirrors practice_invitations.status's live
// value (00030): a row still open to being accepted.
const invitationStatusPending = "pending"

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

// Worker sends due staff_invite_outbox rows through outbox.MailWorker's
// shared claim-scan-compose-send-mark loop -- this kind's own share is
// only its table, claim query, row shape and Compose below. invite_token
// is cleared on both sent and dead-lettered terminal states, not only on
// sent: this table's whole justification for holding plaintext at all is
// that its exposure window is "queued but not yet sent", and a
// dead-lettered row is done retrying too.
type Worker = outbox.MailWorker[pendingRow]

// NewWorker builds the Staff invitation outbox worker around mailer.
func NewWorker(mailer outbox.Mailer) Worker {
	return Worker{
		Mailer:          mailer,
		Table:           "staff_invite_outbox",
		ClearOnTerminal: []string{"invite_token"},
		ClaimQuery:      claimQuery,
		Scan:            scanRow,
		Compose:         compose(mailer),
	}
}

type pendingRow struct {
	inviteToken sql.NullString
	address     string
	status      string
	expiresAt   time.Time
}

const claimQuery = `SELECT o.id, o.attempt_count, o.invite_token, pi.address, pi.status, pi.expires_at
	 FROM staff_invite_outbox o
	 JOIN practice_invitations pi ON pi.id = o.invitation_id
	 WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	 ORDER BY o.next_attempt_at
	 LIMIT $1
	 FOR UPDATE OF o SKIP LOCKED`

func scanRow(rows *sql.Rows) (outbox.RowMeta, pendingRow, error) {
	var meta outbox.RowMeta
	var r pendingRow
	if err := rows.Scan(&meta.ID, &meta.AttemptCount, &r.inviteToken, &r.address, &r.status, &r.expiresAt); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return meta, r, fmt.Errorf("staffinvite: scan outbox row: %w", err)
	}
	return meta, r, nil
}

// compose resolves one pending row: skipped if the Invitation is no
// longer pending or has expired -- accepted, revoked, or expired,
// whether or not something has gotten around to flipping the status
// column yet, either way nothing to deliver -- mailed otherwise. It
// joins practice_invitations for the recipient's address and current
// status at send time, so an Invitation resolved through some other path
// before this row was sent is never mailed. The mailable token itself
// comes from the outbox row, not the join -- practice_invitations only
// ever holds its digest (00030).
func compose(mailer outbox.Mailer) func(context.Context, *sql.Tx, pendingRow, time.Time) (string, string, string, error) {
	return func(_ context.Context, _ *sql.Tx, r pendingRow, now time.Time) (string, string, string, error) {
		if r.status != invitationStatusPending || !r.expiresAt.After(now) {
			return "", "", "", outbox.ErrAlreadyDone
		}
		link := mailer.AppBaseURL + "/accept-invite?token=" + r.inviteToken.String
		return r.address, staffInviteSubject, staffInviteText(link), nil
	}
}
