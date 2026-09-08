// Package mfarecoverymail is ADR-0010's outbox for #615's Owner-vouched
// recovery code: it delivers a single-use code to the vouching Owner's
// own address, never to the locked-out person's -- #605's "her own
// address, not Priya's, which is what makes it a second channel".
//
// Deliberately a separate table and worker from authmail's
// staff_token_mail_outbox, not a third TokenMailKind on it: that
// worker resolves its recipient as the identity the token itself
// belongs to, which for this code is the locked-out person, not the
// Owner who has to receive the mail.
package mfarecoverymail

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/outbox"
)

// CodeLifetime is #605's §4.2.1.3 expiry for an issued recovery code.
const CodeLifetime = 24 * time.Hour

// QueueVouchedCodeMail inserts a pending staff_mfa_recovery_outbox row
// delivering code to recipientIdentityUID (the vouching Owner), naming
// subjectStaffID (the locked-out person) so the mail copy can say who
// the code is for. Unlike staff_token_mail_outbox's one-pending-per-kind
// index, this carries no such constraint: an Owner vouching for two
// people close together must queue two rows, not clobber one.
func QueueVouchedCodeMail(ctx context.Context, tx *sql.Tx, recipientIdentityUID, subjectStaffID, code string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO staff_mfa_recovery_outbox (recipient_identity_uid, subject_staff_id, token) VALUES ($1, $2, $3)`,
		recipientIdentityUID, subjectStaffID, code,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("mfarecoverymail: queue vouched code mail: %w", err)
	}
	return nil
}

// Worker sends due staff_mfa_recovery_outbox rows through
// outbox.MailWorker's shared claim-scan-compose-send-mark loop -- this
// kind's own share is only its table, claim query, row shape and Compose
// below. Accounts resolves the recipient Owner's *current* address at
// send time, mirroring authmail.TokenMailWorker's own reasoning (#614:
// staff.email can drift from the account Identity Platform actually
// holds).
type Worker = outbox.MailWorker[pendingRow]

// NewWorker builds the recovery-code outbox worker around mailer and
// accounts.
func NewWorker(mailer outbox.Mailer, accounts authn.AccountManager) Worker {
	return Worker{
		Mailer:          mailer,
		Table:           "staff_mfa_recovery_outbox",
		ClearOnTerminal: []string{"token"},
		ClaimQuery:      claimQuery,
		Scan:            scanRow,
		Compose:         compose(accounts),
	}
}

type pendingRow struct {
	recipientIdentityUID string
	subjectStaffID       string
	token                sql.NullString
}

const claimQuery = `SELECT id, attempt_count, recipient_identity_uid, subject_staff_id, token
	 FROM staff_mfa_recovery_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanRow(rows *sql.Rows) (outbox.RowMeta, pendingRow, error) {
	var meta outbox.RowMeta
	var r pendingRow
	if err := rows.Scan(&meta.ID, &meta.AttemptCount, &r.recipientIdentityUID, &r.subjectStaffID, &r.token); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return meta, r, fmt.Errorf("mfarecoverymail: scan outbox row: %w", err)
	}
	return meta, r, nil
}

// compose resolves the vouching Owner's current Identity Platform account
// (ctx: a network call) and the locked-out colleague's name (tx: a query
// in the same transaction the row was claimed in) -- the two reasons this
// kind's Compose needs both.
//
// Both of those people are rechecked against `staff` before either
// resolution runs, and either one having deleted her login since the
// vouch resolves the row to ErrAlreadyDone -- marked sent, nothing
// mailed. The recheck comes first rather than after GetAccount because
// a deleted login has no Identity Platform account left: reaching the
// Admin SDK for it would answer ErrAccountNotFound and dead-letter a
// row that has simply been overtaken by events.
func compose(accounts authn.AccountManager) func(context.Context, *sql.Tx, pendingRow, time.Time) (string, string, string, error) {
	return func(ctx context.Context, tx *sql.Tx, r pendingRow, _ time.Time) (string, string, string, error) {
		recipientLive, err := recipientHasLiveLogin(ctx, tx, r.recipientIdentityUID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return "", "", "", err
		}
		if !recipientLive {
			return "", "", "", outbox.ErrAlreadyDone
		}

		account, err := accounts.GetAccount(ctx, r.recipientIdentityUID)
		if errors.Is(err, authn.ErrAccountNotFound) {
			return "", "", "", &outbox.DeadLetterError{Reason: "no Identity Platform account for this recipient"}
		}
		if err != nil {
			return "", "", "", fmt.Errorf("mfarecoverymail: resolve account: %w", err)
		}

		if !r.token.Valid || r.token.String == "" {
			// coverage:ignore reason: every row this package queues carries a token; unreachable without writing to the table outside QueueVouchedCodeMail
			return "", "", "", &outbox.DeadLetterError{Reason: "outbox row carries no token"}
		}

		name, subjectDeleted, err := subjectName(ctx, tx, r.subjectStaffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return "", "", "", err
		}
		if subjectDeleted {
			return "", "", "", outbox.ErrAlreadyDone
		}

		subject, text := vouchedCodeCopy(name, r.token.String)
		return account.Email, subject, text, nil
	}
}

// subjectName reads the name of the person a vouched code is for, so the
// mail can say who to hand it to, and reports whether she has since
// deleted her own login.
//
// staff_mfa_recovery_outbox.subject_staff_id carries a plain
// (non-cascading) foreign key to staff(id), and nothing in this codebase
// ever deletes a staff row -- RemoveMembershipHandler deletes the
// Membership, not the person, and #892's deletion redacts the row in
// place precisely because sixteen tables point at its id. So the row
// this queries always exists, and there is no missing-row case to fall
// back for.
//
// What #892 does add is a row that exists and names nobody: name reads
// "Deleted Staff Member" and deleted_at is stamped. A code issued for
// her is a code for a login that no longer exists, so deleted is
// returned rather than swallowed -- mailing an Owner a recovery code
// naming a person who cannot use it is worse than mailing nothing, and
// the mail would be the first she heard of it.
func subjectName(ctx context.Context, tx *sql.Tx, staffID string) (name string, deleted bool, err error) {
	if err := tx.QueryRowContext(ctx,
		`SELECT name, deleted_at IS NOT NULL FROM staff WHERE id = $1`, staffID,
	).Scan(&name, &deleted); err != nil {
		// coverage:ignore reason: the FK guarantees a matching staff row; only a DB failure could reach here, not exercised by unit tests
		return "", false, fmt.Errorf("mfarecoverymail: read subject name: %w", err)
	}
	return name, deleted, nil
}

// recipientHasLiveLogin reports whether recipientIdentityUID still names
// a Staff person who has not deleted her own login.
//
// Absence is read as deletion here, and that is sound rather than
// convenient: the only writer of this table is QueueVouchedCodeMail, and
// the only caller of that is the vouching Owner's own request, so every
// row's recipient held a staff row at the moment it was queued. The
// deleted_at predicate cannot be what catches her on its own --
// staffauth.DeleteLoginHandler rewrites identity_uid to a
// 'deleted:<id>' sentinel in the same UPDATE that stamps deleted_at, so
// a deleted person's uid matches no row at all -- but it is written
// where the read happens so the rule is legible, and so this query keeps
// refusing a redacted row if that sentinel is ever dropped.
func recipientHasLiveLogin(ctx context.Context, tx *sql.Tx, recipientIdentityUID string) (bool, error) {
	var live bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM staff WHERE identity_uid = $1 AND deleted_at IS NULL)`,
		recipientIdentityUID,
	).Scan(&live); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("mfarecoverymail: check recipient login: %w", err)
	}
	return live, nil
}

// vouchedCodeCopy is the issued code's fixed mail copy. It names the
// locked-out colleague -- a fellow Staff member, not a Client or a
// Practice, so ADR-0009's content restriction does not reach it -- so an
// Owner vouching for more than one person is not left guessing which
// code is which.
func vouchedCodeCopy(subjectName, code string) (subject, text string) {
	return "Doula Cloud: account recovery code", "Hello,\n\n" +
		"You approved an account-recovery request for " + subjectName + ".\n\n" +
		"Recovery code: " + code + "\n\n" +
		"Give this code to " + subjectName + " directly -- it lets her sign back in and set up a new authenticator. It expires in 24 hours and can be used once.\n\n" +
		"If you didn't approve this, reply to this email right away.\n"
}
