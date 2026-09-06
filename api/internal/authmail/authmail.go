// Package authmail is ADR-0010's outbox for #613's three Staff auth
// mail kinds -- email verification, password reset, and the email-change
// notice -- built on #169's decision to keep Identity Platform as the
// credential store while Doula Cloud's own outbox becomes the post
// office.
//
// Two tables (00061), because the two kinds minted from auth_tokens
// share a recipient-resolution shape a third one does not:
// staff_token_mail_outbox's worker reads the account's *current* address
// from Identity Platform via the Admin SDK at send time -- staff.email
// can drift from it (#614) -- while staff_email_change_outbox notifies
// the address a change moved *away* from, which has to be captured at
// request time because it is gone from the account by the time the
// worker runs.
//
// Neither table has a tasknudge.OutboxType: #613's AC is explicit that no
// ADR-0010 exception is carved for this mail -- "delay accepted" -- so
// Cloud Scheduler's own cadence is the whole delivery guarantee here,
// unlike the eight kinds ADR-0013's nudge speeds up.
package authmail

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/outbox"
)

// TokenMailKind mirrors staff_token_mail_kind (00061).
type TokenMailKind string

const (
	// KindEmailVerification is a mint of PurposeStaffEmailVerification.
	KindEmailVerification TokenMailKind = "email_verification"
	// KindPasswordReset is a mint of PurposeStaffPasswordReset.
	KindPasswordReset TokenMailKind = "password_reset"
)

// VerificationLinkLifetime and ResetLinkLifetime are #613's per-purpose
// expiries: verification grants nothing but a flag, so it can be
// generous; reset grants a credential change, matching Identity
// Platform's own reset default.
const (
	VerificationLinkLifetime = 24 * time.Hour
	ResetLinkLifetime        = time.Hour
)

// TokenMailWorker sends due staff_token_mail_outbox rows through
// outbox.MailWorker's shared claim-scan-compose-send-mark loop -- this
// kind's own share is only its table, claim query, row shape and Compose
// below. Accounts resolves the *current* recipient address at send time,
// per this package's own doc comment.
type TokenMailWorker = outbox.MailWorker[tokenMailRow]

// NewTokenMailWorker builds the Staff verification/reset outbox worker
// around mailer and accounts.
func NewTokenMailWorker(mailer outbox.Mailer, accounts authn.AccountManager) TokenMailWorker {
	return TokenMailWorker{
		Mailer:          mailer,
		Table:           "staff_token_mail_outbox",
		ClearOnTerminal: []string{"token"},
		ClaimQuery:      tokenMailClaimQuery,
		Scan:            scanTokenMailRow,
		Compose:         composeTokenMail(mailer, accounts),
	}
}

type tokenMailRow struct {
	identityUID string
	kind        TokenMailKind
	token       sql.NullString
}

//nolint:gosec // G101 flags the "token" column name; this is SQL query text, not a credential
const tokenMailClaimQuery = `SELECT id, attempt_count, identity_uid, kind, token
	 FROM staff_token_mail_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanTokenMailRow(rows *sql.Rows) (outbox.RowMeta, tokenMailRow, error) {
	var meta outbox.RowMeta
	var r tokenMailRow
	if err := rows.Scan(&meta.ID, &meta.AttemptCount, &r.identityUID, &r.kind, &r.token); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return meta, r, fmt.Errorf("authmail: scan token mail row: %w", err)
	}
	return meta, r, nil
}

// composeTokenMail resolves the recipient's current Identity Platform
// account, which is what makes this Compose need ctx (accounts.GetAccount
// is a network call). ErrAccountNotFound is terminal (dead-lettered, no
// account left to notify); any other AccountManager error retries per
// BackoffSchedule, same as a Mailgun failure would.
func composeTokenMail(mailer outbox.Mailer, accounts authn.AccountManager) func(context.Context, *sql.Tx, tokenMailRow, time.Time) (string, string, string, error) {
	return func(ctx context.Context, _ *sql.Tx, r tokenMailRow, _ time.Time) (string, string, string, error) {
		account, err := accounts.GetAccount(ctx, r.identityUID)
		if errors.Is(err, authn.ErrAccountNotFound) {
			return "", "", "", &outbox.DeadLetterError{Reason: "no Identity Platform account for this identity"}
		}
		if err != nil {
			return "", "", "", fmt.Errorf("authmail: resolve account: %w", err)
		}

		if r.kind == KindEmailVerification && account.EmailVerified {
			// Already verified through some other path -- another link
			// sent before this row's own re-request superseded it, or a
			// provider that reports addresses pre-verified -- before this
			// row got sent. Nothing to deliver.
			return "", "", "", outbox.ErrAlreadyDone
		}

		if !r.token.Valid || r.token.String == "" {
			// coverage:ignore reason: every row this package queues carries a token; unreachable without writing to the table outside Queue
			return "", "", "", &outbox.DeadLetterError{Reason: "outbox row carries no token"}
		}

		subject, text := tokenMailCopy(r.kind, mailer.AppBaseURL, r.token.String)
		return account.Email, subject, text, nil
	}
}

// tokenMailCopy is verification and reset mail's fixed, content-free
// copy (ADR-0009): no Staff name, no Practice name, only the link.
func tokenMailCopy(kind TokenMailKind, appBaseURL, token string) (subject, text string) {
	switch kind {
	case KindPasswordReset:
		link := appBaseURL + "/reset-password?token=" + token
		return "Reset your Doula Cloud password", "Hello,\n\n" +
			"We received a request to reset your Doula Cloud password.\n\n" +
			link + "\n\n" +
			"This link expires in one hour. If you didn't request this, you can safely ignore this email -- your password has not been changed.\n"
	case KindEmailVerification:
		link := appBaseURL + "/verify-email?token=" + token
		return "Verify your email address", "Hello,\n\n" +
			"Please verify your email address for Doula Cloud.\n\n" +
			link + "\n\n" +
			"This link expires in 24 hours. If you didn't request this, you can safely ignore this email.\n"
	}
	// coverage:ignore reason: TokenMailKind is a closed two-value type (00061's staff_token_mail_kind enum); every value this package ever constructs matches one of the two cases above
	return "", ""
}

// QueueTokenMail inserts a pending staff_token_mail_outbox row for
// identityUID and kind carrying token, or -- if one is already pending
// (a re-request) -- resets it via ON CONFLICT so the worker retries
// immediately with the fresh token rather than mailing a superseded one.
func QueueTokenMail(ctx context.Context, tx *sql.Tx, identityUID string, kind TokenMailKind, token string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO staff_token_mail_outbox (identity_uid, kind, token) VALUES ($1, $2, $3)
		 ON CONFLICT (identity_uid, kind) WHERE status = 'pending'
		 DO UPDATE SET token = $3, attempt_count = 0, next_attempt_at = now(), last_error = NULL`,
		identityUID, kind, token,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("authmail: queue token mail: %w", err)
	}
	return nil
}

// EmailChangeWorker sends due staff_email_change_outbox rows -- the
// notice a Staff email change mails to the address it moved away from --
// through the same outbox.MailWorker loop as TokenMailWorker above.
type EmailChangeWorker = outbox.MailWorker[emailChangeRow]

// NewEmailChangeWorker builds the Staff email-change-notice outbox
// worker around mailer.
func NewEmailChangeWorker(mailer outbox.Mailer) EmailChangeWorker {
	return EmailChangeWorker{
		Mailer:     mailer,
		Table:      "staff_email_change_outbox",
		ClaimQuery: emailChangeClaimQuery,
		Scan:       scanEmailChangeRow,
		Compose:    composeEmailChange,
	}
}

type emailChangeRow struct {
	oldEmail string
}

const emailChangeClaimQuery = `SELECT id, attempt_count, old_email
	 FROM staff_email_change_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanEmailChangeRow(rows *sql.Rows) (outbox.RowMeta, emailChangeRow, error) {
	var meta outbox.RowMeta
	var r emailChangeRow
	if err := rows.Scan(&meta.ID, &meta.AttemptCount, &r.oldEmail); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return meta, r, fmt.Errorf("authmail: scan email change row: %w", err)
	}
	return meta, r, nil
}

// emailChangeSubject and emailChangeText are the notice's fixed,
// content-free copy. The new address is deliberately not named here:
// the old address's owner only needs to know a change happened and
// whether she recognizes it, not what it changed to.
const emailChangeSubject = "The email on your Doula Cloud account was changed"

const emailChangeText = "Hello,\n\n" +
	"The email address on your Doula Cloud account was changed.\n\n" +
	"If you made this change, no action is needed. If you did not, reply to this email right away.\n"

func composeEmailChange(_ context.Context, _ *sql.Tx, r emailChangeRow, _ time.Time) (string, string, string, error) {
	return r.oldEmail, emailChangeSubject, emailChangeText, nil
}

// QueueEmailChangeNotice inserts a pending staff_email_change_outbox row
// notifying oldEmail that identityUID's account address changed.
func QueueEmailChangeNotice(ctx context.Context, tx *sql.Tx, identityUID, oldEmail string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO staff_email_change_outbox (identity_uid, old_email) VALUES ($1, $2)`,
		identityUID, oldEmail,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("authmail: queue email change notice: %w", err)
	}
	return nil
}
