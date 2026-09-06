package clientauth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/outbox"
)

// AddressChangeWorker sends due portal_address_change_outbox rows --
// ADR-0010's outbox for #619's confirmation link -- through the same
// outbox.MailWorker loop as MagicLinkWorker. Unlike MagicLinkWorker, it
// resolves no recipient at all: the address this mail is going to is the
// one thing portal_accounts does not yet hold, so the row carries it.
type AddressChangeWorker = outbox.MailWorker[addressChangeRow]

// NewAddressChangeWorker builds the sign-in-address confirmation outbox
// worker around mailer.
func NewAddressChangeWorker(mailer outbox.Mailer) AddressChangeWorker {
	return AddressChangeWorker{
		Mailer:          mailer,
		Table:           "portal_address_change_outbox",
		ClearOnTerminal: []string{"token"},
		ClaimQuery:      addressChangeClaimQuery,
		Scan:            scanAddressChangeRow,
		Compose:         composeAddressChange(mailer),
	}
}

type addressChangeRow struct {
	token     sql.NullString
	toAddress string
}

// No join, unlike magicLinkClaimQuery: to_address is on the row itself.
const addressChangeClaimQuery = `SELECT id, attempt_count, token, to_address
	 FROM portal_address_change_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanAddressChangeRow(rows *sql.Rows) (outbox.RowMeta, addressChangeRow, error) {
	var meta outbox.RowMeta
	var r addressChangeRow
	if err := rows.Scan(&meta.ID, &meta.AttemptCount, &r.token, &r.toAddress); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return meta, r, fmt.Errorf("clientauth: scan address change outbox row: %w", err)
	}
	return meta, r, nil
}

func composeAddressChange(mailer outbox.Mailer) func(context.Context, *sql.Tx, addressChangeRow, time.Time) (string, string, string, error) {
	return func(_ context.Context, _ *sql.Tx, r addressChangeRow, _ time.Time) (string, string, string, error) {
		if !r.token.Valid || r.token.String == "" {
			// coverage:ignore reason: every row this package queues carries a token; unreachable without writing to the table outside queueAddressChangeMail
			return "", "", "", &outbox.DeadLetterError{Reason: "outbox row carries no token"}
		}
		subject, text := addressChangeCopy(mailer.AppBaseURL, r.token.String)
		return r.toAddress, subject, text, nil
	}
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
