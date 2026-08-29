package portalinvite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
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

// Worker sends due portal_invite_outbox rows -- the Cloud-Scheduler-driven
// half of ADR-0010's outbox, mirroring push.Pusher's real/fake split via
// the injected mail.Sender. Now is injectable so retry/dead-letter tests
// don't depend on a real clock. outbox.ProcessPending owns the claim/
// retry/dead-letter machinery every mail kind shares.
type Worker struct {
	Sender     mail.Sender
	Now        func() time.Time
	AppBaseURL string
	From       string
	ReplyTo    string
}

func (w Worker) inner() outbox.Worker {
	return outbox.Worker{Sender: w.Sender, Now: w.Now, From: w.From, ReplyTo: w.ReplyTo, Table: "portal_invite_outbox"}
}

type pendingRow struct {
	id           string
	attemptCount int
	inviteToken  sql.NullString
	identityUID  sql.NullString
	// email is nullable since #396/ADR-0017 relaxed clients.email --
	// a Practice may hold a Client with no address on file yet.
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

func scanRow(rows *sql.Rows) (pendingRow, error) {
	var r pendingRow
	err := rows.Scan(&r.id, &r.attemptCount, &r.inviteToken, &r.identityUID, &r.email)
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
	return fmt.Errorf("portalinvite: %w", err)
}

// ProcessPending sends every due portal_invite_outbox row within tx: it
// joins client_portal_users (and clients, for the recipient address) to
// read the *current* invite_token and email at send time, so a re-invite
// that rotated the token after this row was queued is never mailed
// stale. A row whose invite was already accepted before send is marked
// sent without mailing anything -- the Client already has access.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), claimQuery, scanRow, w.send))
}

func (w Worker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingRow, now time.Time) error {
	if r.identityUID.Valid {
		// Already accepted -- through some other path -- before this row
		// got sent. Nothing to deliver; record it sent so it never
		// retries.
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
	}

	if !r.email.Valid || r.email.String == "" {
		// ADR-0017: clients.email is nullable now, and this row must
		// refuse rather than mail a live token to an empty string.
		// Dead-lettered outright, not scheduled for retry -- nothing
		// about waiting fixes a missing address; a Staff member must add
		// one and send a fresh invite.
		return wrapOutboxErr(inner.MarkDeadLetteredNow(ctx, tx, r.id, "client has no email on file"))
	}

	link := w.AppBaseURL + "/portal/accept-invite?token=" + r.inviteToken.String
	sendErr := w.Sender.Send(ctx, mail.Message{
		To:      r.email.String,
		From:    w.From,
		ReplyTo: w.ReplyTo,
		Subject: inviteSubject,
		Text:    inviteText(link),
	})
	if sendErr == nil {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
	}
	return wrapOutboxErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, sendErr, now))
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
