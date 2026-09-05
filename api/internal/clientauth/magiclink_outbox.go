package clientauth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
)

// wrapOutboxErr gives an error from the outbox package (a sibling
// package, so wrapcheck treats its errors as external) this package's
// own prefix -- portalinvite and authmail each keep the same small
// helper rather than sharing one, since it does nothing but rename an
// error.
func wrapOutboxErr(err error) error {
	if err == nil {
		return nil
	}
	// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
	return fmt.Errorf("clientauth: %w", err)
}

// MagicLinkWorker sends due portal_magic_link_outbox rows -- the
// Cloud-Scheduler-driven half of ADR-0010's outbox for #617's sign-in
// link. Unlike authmail.TokenMailWorker, the recipient address is
// resolved with a plain join against portal_accounts rather than an
// Admin SDK call: it is this product's own column, not Identity
// Platform's.
type MagicLinkWorker struct {
	Sender     mail.Sender
	Now        func() time.Time
	AppBaseURL string
	From       string
	ReplyTo    string
}

func (w MagicLinkWorker) inner() outbox.Worker {
	return outbox.Worker{
		Sender: w.Sender, Now: w.Now, From: w.From, ReplyTo: w.ReplyTo,
		Table:           "portal_magic_link_outbox",
		ClearOnTerminal: []string{"token"},
	}
}

type magicLinkRow struct {
	id            string
	attemptCount  int
	token         sql.NullString
	signInAddress string
}

// The join is INNER, not LEFT: portal_magic_link_outbox.identity_uid
// carries a real ON DELETE CASCADE FK to portal_accounts (00074), so a
// pending row can never outlive the account it names -- erasing a Client
// mid-flight deletes this row along with her Portal Account rather than
// leaving it to be claimed with nothing to send to.
const magicLinkClaimQuery = `SELECT o.id, o.attempt_count, o.token, pa.sign_in_address
	 FROM portal_magic_link_outbox o
	 JOIN portal_accounts pa ON pa.identifier = o.identity_uid
	 WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	 ORDER BY o.next_attempt_at
	 LIMIT $1
	 FOR UPDATE OF o SKIP LOCKED`

func scanMagicLinkRow(rows *sql.Rows) (magicLinkRow, error) {
	var r magicLinkRow
	err := rows.Scan(&r.id, &r.attemptCount, &r.token, &r.signInAddress)
	return r, wrapOutboxErr(err)
}

// ProcessPending sends every due portal_magic_link_outbox row within tx.
func (w MagicLinkWorker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), magicLinkClaimQuery, scanMagicLinkRow, w.send))
}

func (w MagicLinkWorker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r magicLinkRow, now time.Time) error {
	if !r.token.Valid || r.token.String == "" {
		// coverage:ignore reason: every row this package queues carries a token; unreachable without writing to the table outside queueMagicLinkMail
		return wrapOutboxErr(inner.MarkDeadLetteredNow(ctx, tx, r.id, "outbox row carries no token"))
	}

	subject, text := magicLinkCopy(w.AppBaseURL, r.token.String)
	sendErr := w.Sender.Send(ctx, mail.Message{To: r.signInAddress, From: w.From, ReplyTo: w.ReplyTo, Subject: subject, Text: text})
	if sendErr == nil {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
	}
	return wrapOutboxErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, sendErr, now))
}

// magicLinkCopy is the sign-in link's fixed, content-free copy (ADR-0009):
// no Client name, no Practice name, only the link.
func magicLinkCopy(appBaseURL, token string) (subject, text string) {
	link := appBaseURL + "/portal/sign-in?token=" + token
	return "Your Doula Cloud sign-in link", "Hello,\n\n" +
		"Here is your sign-in link for Doula Cloud.\n\n" +
		link + "\n\n" +
		"This link expires in 15 minutes. If you didn't request this, you can safely ignore this email.\n"
}

// queueMagicLinkMail inserts a pending portal_magic_link_outbox row for
// identifier carrying token, or -- if one is already pending (she asked
// twice before reading the first mail) -- resets it via ON CONFLICT so
// the worker retries immediately with the fresh token rather than
// mailing a superseded one.
func queueMagicLinkMail(ctx context.Context, tx *sql.Tx, identifier, token string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO portal_magic_link_outbox (identity_uid, token) VALUES ($1, $2)
		 ON CONFLICT (identity_uid) WHERE status = 'pending'
		 DO UPDATE SET token = $2, attempt_count = 0, next_attempt_at = now(), last_error = NULL`,
		identifier, token,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("clientauth: queue magic link mail: %w", err)
	}
	return nil
}
