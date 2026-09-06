package portalinvite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/outbox"
)

// inviteSubject and inviteText are the Client portal invite's fixed,
// content-free copy (ADR-0009's rule, unconditional per #221: no Client
// name, no Engagement detail, and -- for v1 -- no Practice name either,
// anywhere in subject or body). link is the only variable.
const inviteSubject = "You've been invited to view your care details online"

func inviteText(link string) string {
	return "Hello,\n\n" +
		"You've been invited to view your care details online.\n\n" +
		link + "\n\n" +
		"If you weren't expecting this, you can safely ignore this email.\n"
}

// Worker sends due portal_invite_outbox rows through outbox.MailWorker's
// shared claim-scan-compose-send-mark loop -- this kind's own share is
// only its table, claim query, row shape and Compose below.
type Worker = outbox.MailWorker[pendingRow]

// NewWorker builds the Client portal invite outbox worker around mailer.
func NewWorker(mailer outbox.Mailer) Worker {
	return Worker{
		Mailer:     mailer,
		Table:      "portal_invite_outbox",
		ClaimQuery: claimQuery,
		Scan:       scanRow,
		Compose:    compose(mailer),
	}
}

type pendingRow struct {
	inviteToken sql.NullString
	identityUID sql.NullString
	// email is nullable since #396/ADR-0017 relaxed clients.email -- a
	// Practice may hold a Client with no address on file yet.
	email sql.NullString
}

const claimQuery = `SELECT o.id, o.attempt_count, pu.invite_token, pu.identity_uid, c.email
	 FROM portal_invite_outbox o
	 JOIN client_portal_users pu ON pu.id = o.client_portal_user_id
	 JOIN clients c ON c.id = pu.client_id
	 WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	 ORDER BY o.next_attempt_at
	 LIMIT $1
	 FOR UPDATE OF o SKIP LOCKED`

func scanRow(rows *sql.Rows) (outbox.RowMeta, pendingRow, error) {
	var meta outbox.RowMeta
	var r pendingRow
	if err := rows.Scan(&meta.ID, &meta.AttemptCount, &r.inviteToken, &r.identityUID, &r.email); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return meta, r, fmt.Errorf("portalinvite: scan outbox row: %w", err)
	}
	return meta, r, nil
}

// compose joins client_portal_users (and clients, for the recipient
// address) to read the *current* invite_token and email at send time, so
// a re-invite that rotated the token after this row was queued is never
// mailed stale. A row whose invite was already accepted before send
// resolves to ErrAlreadyDone -- the Client already has access. A row for
// a Client with no email on file (ADR-0017) is a *DeadLetterError, not
// scheduled for retry: nothing about waiting fixes a missing address; a
// Staff member must add one and send a fresh invite.
func compose(mailer outbox.Mailer) func(context.Context, *sql.Tx, pendingRow, time.Time) (string, string, string, error) {
	return func(_ context.Context, _ *sql.Tx, r pendingRow, _ time.Time) (string, string, string, error) {
		if r.identityUID.Valid {
			return "", "", "", outbox.ErrAlreadyDone
		}
		if !r.email.Valid || r.email.String == "" {
			return "", "", "", &outbox.DeadLetterError{Reason: "client has no email on file"}
		}
		link := mailer.AppBaseURL + "/portal/accept-invite?token=" + r.inviteToken.String
		return r.email.String, inviteSubject, inviteText(link), nil
	}
}

// queueOutboxSend inserts a pending portal_invite_outbox row for
// portalUserID, or -- if one is already pending (a re-invite) -- resets
// its attempt_count and next_attempt_at so the worker retries
// immediately. It never stores the invite_token itself; compose reads
// that fresh from client_portal_users at send time, so a rotation after
// this call is queued is always picked up, not mailed stale.
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
