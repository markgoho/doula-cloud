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
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
)

// CodeLifetime is #605's §4.2.1.3 expiry for an issued recovery code.
const CodeLifetime = 24 * time.Hour

func wrapOutboxErr(err error) error {
	if err == nil {
		return nil
	}
	// coverage:ignore reason: only reached by a DB failure inside the outbox package, not exercised by unit tests
	return fmt.Errorf("mfarecoverymail: %w", err)
}

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

// Worker sends due staff_mfa_recovery_outbox rows. Accounts resolves the
// recipient Owner's *current* address at send time, mirroring
// authmail.TokenMailWorker's own reasoning (#614: staff.email can drift
// from the account Identity Platform actually holds).
type Worker struct {
	Sender   mail.Sender
	Accounts authn.AccountManager
	Now      func() time.Time
	From     string
	ReplyTo  string
}

func (w Worker) inner() outbox.Worker {
	return outbox.Worker{
		Sender: w.Sender, Now: w.Now, From: w.From, ReplyTo: w.ReplyTo,
		Table:           "staff_mfa_recovery_outbox",
		ClearOnTerminal: []string{"token"},
	}
}

type pendingRow struct {
	id                   string
	attemptCount         int
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

func scanRow(rows *sql.Rows) (pendingRow, error) {
	var r pendingRow
	err := rows.Scan(&r.id, &r.attemptCount, &r.recipientIdentityUID, &r.subjectStaffID, &r.token)
	return r, wrapOutboxErr(err)
}

// ProcessPending sends every due staff_mfa_recovery_outbox row within tx.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), claimQuery, scanRow, w.send))
}

func (w Worker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingRow, now time.Time) error {
	account, err := w.Accounts.GetAccount(ctx, r.recipientIdentityUID)
	if errors.Is(err, authn.ErrAccountNotFound) {
		return wrapOutboxErr(inner.MarkDeadLetteredNow(ctx, tx, r.id, "no Identity Platform account for this recipient"))
	}
	if err != nil {
		return wrapOutboxErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, err, now))
	}

	if !r.token.Valid || r.token.String == "" {
		// coverage:ignore reason: every row this package queues carries a token; unreachable without writing to the table outside Queue
		return wrapOutboxErr(inner.MarkDeadLetteredNow(ctx, tx, r.id, "outbox row carries no token"))
	}

	name, err := subjectName(ctx, tx, r.subjectStaffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return wrapOutboxErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, err, now))
	}

	subject, text := vouchedCodeCopy(name, r.token.String)
	sendErr := w.Sender.Send(ctx, mail.Message{To: account.Email, From: w.From, ReplyTo: w.ReplyTo, Subject: subject, Text: text})
	if sendErr == nil {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
	}
	return wrapOutboxErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, sendErr, now))
}

// subjectName reads the name of the person a vouched code is for, so the
// mail can say who to hand it to. staff_mfa_recovery_outbox.subject_staff_id
// carries a plain (non-cascading) foreign key to staff(id) with nothing in
// this codebase that ever deletes a staff row (RemoveMembershipHandler
// deletes the Membership, not the person), so the row this queries always
// exists -- there is no offboarded-in-between-vouch-and-delivery case to
// fall back for.
func subjectName(ctx context.Context, tx *sql.Tx, staffID string) (string, error) {
	var name string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM staff WHERE id = $1`, staffID).Scan(&name); err != nil {
		// coverage:ignore reason: the FK guarantees a matching staff row; only a DB failure could reach here, not exercised by unit tests
		return "", fmt.Errorf("mfarecoverymail: read subject name: %w", err)
	}
	return name, nil
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
