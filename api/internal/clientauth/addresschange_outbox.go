package clientauth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
)

// AddressChangeWorker sends due portal_address_change_outbox rows --
// ADR-0010's outbox for #619's confirmation link. Unlike
// MagicLinkWorker, it resolves no recipient at all: the address this
// mail is going to is the one thing portal_accounts does not yet hold,
// so the row carries it.
type AddressChangeWorker struct {
	Sender     mail.Sender
	Now        func() time.Time
	AppBaseURL string
	From       string
	ReplyTo    string
}

func (w AddressChangeWorker) inner() outbox.Worker {
	return outbox.Worker{
		Sender: w.Sender, Now: w.Now, From: w.From, ReplyTo: w.ReplyTo,
		Table:           "portal_address_change_outbox",
		ClearOnTerminal: []string{"token"},
	}
}

type addressChangeRow struct {
	id           string
	attemptCount int
	token        sql.NullString
	toAddress    string
}

// No join, unlike magicLinkClaimQuery: to_address is on the row itself.
const addressChangeClaimQuery = `SELECT id, attempt_count, token, to_address
	 FROM portal_address_change_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanAddressChangeRow(rows *sql.Rows) (addressChangeRow, error) {
	var r addressChangeRow
	err := rows.Scan(&r.id, &r.attemptCount, &r.token, &r.toAddress)
	return r, wrapOutboxErr(err)
}

// ProcessPending sends every due portal_address_change_outbox row within tx.
func (w AddressChangeWorker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), addressChangeClaimQuery, scanAddressChangeRow, w.send))
}

func (w AddressChangeWorker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r addressChangeRow, now time.Time) error {
	if !r.token.Valid || r.token.String == "" {
		// coverage:ignore reason: every row this package queues carries a token; unreachable without writing to the table outside queueAddressChangeMail
		return wrapOutboxErr(inner.MarkDeadLetteredNow(ctx, tx, r.id, "outbox row carries no token"))
	}

	subject, text := addressChangeCopy(w.AppBaseURL, r.token.String)
	sendErr := w.Sender.Send(ctx, mail.Message{To: r.toAddress, From: w.From, ReplyTo: w.ReplyTo, Subject: subject, Text: text})
	if sendErr == nil {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
	}
	return wrapOutboxErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, sendErr, now))
}

// addressChangeCopy is the confirmation link's fixed, content-free copy
// (ADR-0009): no Client name, no Practice name, no old address -- this
// mail may be arriving in a mailbox belonging to somebody who never
// asked for it, and it must tell that person nothing.
func addressChangeCopy(appBaseURL, token string) (subject, text string) {
	link := appBaseURL + "/portal/confirm-sign-in-address?token=" + token
	return "Confirm your Doula Cloud sign-in address", "Hello,\n\n" +
		"Someone asked to use this address to sign in to Doula Cloud. Confirm it here:\n\n" +
		link + "\n\n" +
		"This link expires in 24 hours. Until you use it, the old address keeps signing in. " +
		"If you didn't ask for this, you can safely ignore this email.\n"
}

// queueAddressChangeMail inserts a pending portal_address_change_outbox
// row for identifier carrying token and the address to send it to, or --
// if one is already pending (she asked twice, possibly naming a
// different address the second time) -- resets it via ON CONFLICT so the
// worker sends only the fresher request. The superseded token is already
// gone: authtoken.Mint deleted it before this runs.
func queueAddressChangeMail(ctx context.Context, tx *sql.Tx, identifier, toAddress, token string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO portal_address_change_outbox (identity_uid, to_address, token) VALUES ($1, $2, $3)
		 ON CONFLICT (identity_uid) WHERE status = 'pending'
		 DO UPDATE SET to_address = $2, token = $3, attempt_count = 0, next_attempt_at = now(), last_error = NULL`,
		identifier, toAddress, token,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("clientauth: queue address change mail: %w", err)
	}
	return nil
}
