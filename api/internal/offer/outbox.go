package offer

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
)

// offerSubject and offerText are the Offer email's fixed copy. Platform
// voice (ADR-0009): she is told she has been offered work at a practice
// on Doula Cloud, never which practice, never the Client, never anything
// the Offer's own thin page will show her once she opens it with the
// code. link and code are the only variables, and neither is a fact about
// the work.
const offerSubject = "You've been offered work on Doula Cloud"

func offerText(link, code string) string {
	return "Hello,\n\n" +
		"You've been offered work at a practice on Doula Cloud.\n\n" +
		link + "\n\n" +
		"Your access code is " + code + ".\n\n" +
		"If you weren't expecting this, you can safely ignore this email.\n"
}

// queue queues a pending engagement_offer_outbox send for offerID, or --
// if one is somehow already pending -- overwrites its credentials and
// resets the retry schedule. Callers pass the token and code in the
// clear: engagement_offers keeps only the code's digest and
// practice_invitations only the token's, so this row is the only place
// the worker can read mailable ones from at send time. Must run in the
// same transaction as the Offer insert that minted them.
func queue(ctx context.Context, tx *sql.Tx, offerID, token, code string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO engagement_offer_outbox (offer_id, invite_token, access_code) VALUES ($1, $2, $3)
		 ON CONFLICT (offer_id) WHERE status = 'pending'
		 DO UPDATE SET invite_token = $2, access_code = $3, attempt_count = 0,
		               next_attempt_at = now(), last_error = NULL`,
		offerID, token, code,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("offer: queue outbox send: %w", err)
	}
	return nil
}

// Worker sends due engagement_offer_outbox rows -- the
// Cloud-Scheduler-driven half of ADR-0010's outbox (outbox.ProcessPending
// owns the claim/retry/dead-letter machinery every mail kind shares).
type Worker struct {
	Sender     mail.Sender
	Now        func() time.Time
	AppBaseURL string
	From       string
	ReplyTo    string
}

// invite_token/access_code are cleared on both sent and dead-lettered
// terminal states, not only on sent: this table's whole justification
// for holding plaintext at all is that its exposure window is "queued
// but not yet resolved", and a dead-lettered row is done retrying too.
func (w Worker) inner() outbox.Worker {
	return outbox.Worker{
		Sender: w.Sender, Now: w.Now, From: w.From, ReplyTo: w.ReplyTo,
		Table:           "engagement_offer_outbox",
		ClearOnTerminal: []string{"invite_token", "access_code"},
	}
}

// pendingRow is one due outbox row joined to the Offer and Invitation it
// mails.
type pendingRow struct {
	id           string
	offerID      string
	attemptCount int
	inviteToken  sql.NullString
	accessCode   sql.NullString
	address      string
	state        string
	expiresAt    time.Time
}

const claimQuery = `SELECT o.id, o.offer_id, o.attempt_count, o.invite_token, o.access_code,
	        pi.address, eo.state::text, eo.expires_at
	   FROM engagement_offer_outbox o
	   JOIN engagement_offers eo ON eo.id = o.offer_id
	   JOIN practice_invitations pi ON pi.id = eo.invitation_id
	  WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	  ORDER BY o.next_attempt_at
	  LIMIT $1
	  FOR UPDATE OF o SKIP LOCKED`

func scanRow(rows *sql.Rows) (pendingRow, error) {
	var p pendingRow
	err := rows.Scan(&p.id, &p.offerID, &p.attemptCount, &p.inviteToken, &p.accessCode,
		&p.address, &p.state, &p.expiresAt)
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
	return fmt.Errorf("offer: %w", err)
}

// ProcessPending sends every due engagement_offer_outbox row within tx.
// It joins the Offer and its Invitation for the recipient's address and
// the Offer's state at send time, so an Offer withdrawn, declined,
// superseded, or expired through some other path before this row was
// sent is never mailed. The credentials come from the outbox row, not
// the join, and are cleared once the row leaves 'pending'.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), claimQuery, scanRow, w.send))
}

// send resolves one pending row: skipped if the Offer is no longer open,
// mailed otherwise, and marked either way.
func (w Worker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, p pendingRow, now time.Time) error {
	// Not (still) open, or past its own expires_at -- decided, withdrawn,
	// or run out, whether or not something has gotten around to flipping
	// the state column yet -- either way, nothing to deliver.
	if p.state != stateOffered || !p.expiresAt.After(now) {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, p.id, now))
	}

	link := w.AppBaseURL + "/offers/" + p.offerID + "?token=" + p.inviteToken.String
	sendErr := w.Sender.Send(ctx, mail.Message{
		To:      p.address,
		From:    w.From,
		ReplyTo: w.ReplyTo,
		Subject: offerSubject,
		Text:    offerText(link, p.accessCode.String),
	})
	if sendErr != nil {
		return wrapOutboxErr(inner.MarkFailed(ctx, tx, p.id, p.attemptCount, sendErr, now))
	}
	// access_code_sent_at is the Offer's own record that its code is in
	// the post -- 00030 gave the column, and this is the only moment that
	// is true.
	if _, err := tx.ExecContext(ctx,
		`UPDATE engagement_offers SET access_code_sent_at = $1 WHERE id = $2`, now, p.offerID,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("offer: stamp access code sent: %w", err)
	}
	return wrapOutboxErr(inner.MarkSent(ctx, tx, p.id, now))
}
