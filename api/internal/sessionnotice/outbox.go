// Package sessionnotice sends the security-notice Platform
// Notifications ADR-0004 orphaned when session ownership moved off
// Identity Platform to the BFF's own `sessions` table (#345, map #213):
// "new sign-in" and "session revoked", joined since by
// "two-factor reset" (#615) and "signed out in one browser" (#610).
// All are Platform voice (ADR-0009) with a single recipient -- the Staff
// member whose session it is, resolved from identity_uid via `staff` at
// send time, mirroring how billing.Worker resolves Owner emails rather
// than storing them on the row. That resolver is Staff-only, which is
// half of why an evicted *portal* session gets no mail at all -- see
// QueueSessionEvicted for the whole decision.
package sessionnotice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/tasknudge"
)

// signinIdleWindow is the "new sign-in" trigger condition this ticket's
// AC asks to be decided, with reasoning: a sign-in is worth a notice only
// if no sign-in for the same identity has been notice-worthy in the last
// week. ADR-0004 deliberately stores no device or IP on a session row --
// "an active sessions screen is out of scope because the design
// deliberately keeps no session state to list" -- so "new device" and
// "new IP range" both need schema this ticket has no reason to add.
// "First sign-in after N days idle" needs none: it is answerable purely
// from this package's own outbox history. Seven days rather than one
// comfortably absorbs the ticket's own example (a phone and a laptop
// signing in the same day) and any ordinary day-to-day gap in a Staff
// member's schedule, while still catching the case worth flagging -- a
// return after a real gap, which is when a stale, possibly compromised
// credential is most likely to be the one still working.
const signinIdleWindow = 7 * 24 * time.Hour

const newSignInSubject = "Doula Cloud: new sign-in to your account"

func newSignInText() string {
	return "Hello,\n\n" +
		"Your Doula Cloud account was just signed in to.\n\n" +
		"If this was you, no action is needed. If it wasn't, reply to this email right away.\n"
}

const sessionRevokedSubject = "Doula Cloud: your sessions were signed out"

func sessionRevokedText() string {
	return "Hello,\n\n" +
		"All of your Doula Cloud sessions were signed out. You'll need to sign in again on every device.\n\n" +
		"If you didn't expect this, reply to this email.\n"
}

const sessionEvictedSubject = "Doula Cloud: you were signed out in one browser"

// Deliberately not sessionRevokedText: an eviction ends exactly one
// session -- the one the browser she is standing at held -- and every
// other device she is signed in from is untouched, which "all of your
// sessions ... on every device" would misreport. See 00077 for the same
// reasoning at the schema.
func sessionEvictedText() string {
	return "Hello,\n\n" +
		"You were signed out of Doula Cloud in one browser, because that browser was used to sign in to the client portal. Your other devices are still signed in.\n\n" +
		"If you didn't expect this, reply to this email.\n"
}

const mfaRecoveryClearedSubject = "Doula Cloud: your two-factor authentication was reset"

func mfaRecoveryClearedText() string {
	return "Hello,\n\n" +
		"The two-factor authenticator on your Doula Cloud account was removed as part of an account-recovery request, and every one of your sessions was signed out.\n\n" +
		"You'll need to sign in with your password and set up a new authenticator. If you didn't request this, reply to this email right away.\n"
}

// QueueNewSignInIfDue records identityUID's sign-in at now as a "new
// sign-in" notice, unless identityUID names no Staff member (a Client
// Portal sign-in -- this is Platform voice, Staff only) or a notice for
// the same identity was already queued within signinIdleWindow. Runs on
// its own transaction rather than the caller's: session.CreateHandler
// mints the session on db directly, with no ambient tx, and this check
// needs app.notification_worker_trusted to read `staff` (00033) the same
// way the Worker does at send time.
//
// Errors are meant to be swallowed by the caller, the same way
// session.EndHandler swallows a failed EndSession: a notice is a
// best-effort side channel, and failing to queue one must never turn a
// legitimate sign-in into a 500.
//
// enq is ADR-0013's Cloud Tasks nudge; like QueueOutOfCreditsNotification,
// this function commits its own transaction, so the nudge fires
// immediately after that commit succeeds rather than through
// tasknudge.Register/Drain.
func QueueNewSignInIfDue(ctx context.Context, db *sql.DB, identityUID string, now time.Time, enq tasknudge.Enqueuer) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		return fmt.Errorf("sessionnotice: begin queue new-sign-in tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	queued := false

	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("sessionnotice: set trusted flag: %w", err)
	}

	var isStaff bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM staff WHERE identity_uid = $1)`, identityUID).Scan(&isStaff); err != nil {
		return fmt.Errorf("sessionnotice: check staff membership: %w", err)
	}
	if isStaff {
		var dueRecently bool
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM session_notice_outbox
				WHERE identity_uid = $1 AND kind = 'new_signin' AND created_at > $2
			)`,
			identityUID, now.Add(-signinIdleWindow),
		).Scan(&dueRecently); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("sessionnotice: check new-sign-in dedupe: %w", err)
		}
		if !dueRecently {
			// No ON CONFLICT here: new_signin carries no matching unique
			// index (see 00036_session_notice_outbox.sql) -- the
			// dueRecently check above is this row's only dedupe.
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO session_notice_outbox (identity_uid, kind) VALUES ($1, 'new_signin')`,
				identityUID,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				return fmt.Errorf("sessionnotice: queue new-sign-in notice: %w", err)
			}
			queued = true
		}
	}

	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB commit failure, not exercised by unit tests
		return fmt.Errorf("sessionnotice: commit queue new-sign-in tx: %w", err)
	}
	committed = true
	if queued {
		tasknudge.Fire(enq, tasknudge.SessionNotice)(ctx)
	}
	return nil
}

// QueueSessionRevoked records that every session identityUID held was
// just ended. Unlike ordinary sign-out (authn.EndSession, the same actor
// ending their own single browser session -- never notice-worthy), this
// is authn.EndAllSessions: today reached only through
// staffauth.EndSessionsHandler, an Owner ending a Staff member's
// sessions everywhere (offboarding, a lost phone). Every call is its own
// notice-worthy event, so there is no idle-window dedupe here, only the
// same one-pending-row race guard QueueNewSignInIfDue uses. Runs on the
// caller's own tx (mirrors payments.QueuePaymentReceivedNotification):
// EndSessionsHandler already holds one, and this insert has no rollback
// of its own to survive.
func QueueSessionRevoked(ctx context.Context, tx *sql.Tx, identityUID string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO session_notice_outbox (identity_uid, kind) VALUES ($1, 'session_revoked')
		 ON CONFLICT (identity_uid) WHERE status = 'pending' AND kind = 'session_revoked' DO NOTHING`,
		identityUID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("sessionnotice: queue session-revoked notice: %w", err)
	}
	return nil
}

// QueueSessionEvicted records that ev's session was just evicted by a
// sign-in to the other population (#610), and reports whether it queued
// anything.
//
// It queues only for an evicted Staff session, and the "false" it
// returns for a portal one is #610's own recorded decision, not an
// omission. Two reasons, either sufficient. This package resolves its
// recipient from identity_uid via `staff` (see the package comment and
// staffEmail), so a Portal Account identifier has no address to send to
// here at all. And an eviction is self-inflicted and immediate: she
// pressed through a warning that said this would happen, in the browser
// it happened in, seconds ago -- there is nothing an email would tell
// her that the screen in front of her did not.
//
// Runs on the caller's own tx, like QueueSessionRevoked: the mint seam
// that evicts already holds one, and the delete and this insert must
// land or roll back together.
func QueueSessionEvicted(ctx context.Context, tx *sql.Tx, ev authn.Eviction) (queued bool, err error) {
	if ev.Tier != authn.TierStaff {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO session_notice_outbox (identity_uid, kind) VALUES ($1, 'session_evicted')
		 ON CONFLICT (identity_uid) WHERE status = 'pending' AND kind = 'session_evicted' DO NOTHING`,
		ev.IdentityUID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("sessionnotice: queue session-evicted notice: %w", err)
	}
	return true, nil
}

// QueueMFARecoveryCleared records that identityUID's TOTP enrolment was
// just cleared by one of #615's three recovery paths -- a distinct
// notice from QueueSessionRevoked's "sessions ended", queued alongside
// it rather than instead of it, because the two facts (sessions ended,
// enrolment cleared) are both individually notice-worthy per #615's AC.
// Same one-pending-row race guard as QueueSessionRevoked.
func QueueMFARecoveryCleared(ctx context.Context, tx *sql.Tx, identityUID string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO session_notice_outbox (identity_uid, kind) VALUES ($1, 'mfa_recovery_cleared')
		 ON CONFLICT (identity_uid) WHERE status = 'pending' AND kind = 'mfa_recovery_cleared' DO NOTHING`,
		identityUID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("sessionnotice: queue mfa-recovery-cleared notice: %w", err)
	}
	return nil
}

// Worker sends due session_notice_outbox rows -- the Cloud-Scheduler-
// driven half of ADR-0010's outbox (outbox.ProcessPending owns the claim/
// retry/dead-letter machinery every mail kind shares). No AppBaseURL:
// unlike its siblings, neither notice's body links anywhere -- ADR-0004
// built no "active sessions" screen to link to, and a security notice
// needs no more than "reply if this wasn't you" to be actionable.
type Worker struct {
	Sender  mail.Sender
	Now     func() time.Time
	From    string
	ReplyTo string
}

func (w Worker) inner() outbox.Worker {
	return outbox.Worker{Sender: w.Sender, Now: w.Now, From: w.From, ReplyTo: w.ReplyTo, Table: "session_notice_outbox"}
}

type pendingRow struct {
	id           string
	identityUID  string
	kind         string
	attemptCount int
}

const claimQuery = `SELECT id, identity_uid, kind, attempt_count
	 FROM session_notice_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanRow(rows *sql.Rows) (pendingRow, error) {
	var r pendingRow
	err := rows.Scan(&r.id, &r.identityUID, &r.kind, &r.attemptCount)
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
	return fmt.Errorf("sessionnotice: %w", err)
}

// ProcessPending sends every due session_notice_outbox row within tx,
// resolving the target Staff member's current email at send time (not
// stored on the row, same reasoning as billing.Worker.ownerEmails) and
// skipping a row whose identity no longer names a Staff member -- an
// offboarded account deleted between queuing and send has no address
// left to notify.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), claimQuery, scanRow, w.send))
}

func (w Worker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r pendingRow, now time.Time) error {
	email, found, err := staffEmail(ctx, tx, r.identityUID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if !found {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
	}

	var subject, text string
	switch r.kind {
	case "session_revoked":
		subject, text = sessionRevokedSubject, sessionRevokedText()
	case "session_evicted":
		subject, text = sessionEvictedSubject, sessionEvictedText()
	case "mfa_recovery_cleared":
		subject, text = mfaRecoveryClearedSubject, mfaRecoveryClearedText()
	default:
		subject, text = newSignInSubject, newSignInText()
	}

	sendErr := w.Sender.Send(ctx, mail.Message{
		To:      email,
		From:    w.From,
		ReplyTo: w.ReplyTo,
		Subject: subject,
		Text:    text,
	})
	if sendErr == nil {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
	}
	return wrapOutboxErr(inner.MarkFailed(ctx, tx, r.id, r.attemptCount, sendErr, now))
}

// staffEmail returns the email of the Staff member holding identityUID.
func staffEmail(ctx context.Context, tx *sql.Tx, identityUID string) (email string, found bool, err error) {
	err = tx.QueryRowContext(ctx, `SELECT email FROM staff WHERE identity_uid = $1`, identityUID).Scan(&email)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", false, fmt.Errorf("sessionnotice: resolve staff email: %w", err)
	}
	return email, true, nil
}
