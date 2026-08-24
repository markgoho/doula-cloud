// Package sessionnotice sends the two security-notice Platform
// Notifications ADR-0004 orphaned when session ownership moved off
// Identity Platform to the BFF's own `sessions` table (#345, map #213):
// "new sign-in" and "session revoked". Both are Platform voice (ADR-0009)
// with a single recipient -- the Staff member whose session it is,
// resolved from identity_uid via `staff` at send time, mirroring how
// billing.Worker resolves Owner emails rather than storing them on the
// row.
package sessionnotice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/tasknudge"
)

// backoffSchedule mirrors every other outbox in this codebase (ADR-0010):
// attempt N (1-indexed) waits backoffSchedule[N-1] before retrying, and a
// row whose attempt_count reaches len(backoffSchedule) is dead-lettered.
var backoffSchedule = []time.Duration{
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	6 * time.Hour,
	18 * time.Hour,
}

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

// maxOutboxBatch bounds how many rows one ProcessPending call sends, so a
// large backlog can't turn a single Scheduler tick into an unbounded
// transaction. Mirrors billing.Worker/payments.Worker.
const maxOutboxBatch = 100

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

// Worker sends due session_notice_outbox rows -- the Cloud-Scheduler-
// driven half of ADR-0010's outbox, mirroring billing.Worker/
// payments.Worker's shape. No AppBaseURL: unlike its siblings, neither
// notice's body links anywhere -- ADR-0004 built no "active sessions"
// screen to link to, and a security notice needs no more than "reply if
// this wasn't you" to be actionable.
type Worker struct {
	Sender  mail.Sender
	Now     func() time.Time
	From    string
	ReplyTo string
}

type pendingRow struct {
	id           string
	identityUID  string
	kind         string
	attemptCount int
}

// ProcessPending sends every due session_notice_outbox row within tx,
// resolving the target Staff member's current email at send time (not
// stored on the row, same reasoning as billing.Worker.ownerEmails) and
// skipping a row whose identity no longer names a Staff member -- an
// offboarded account deleted between queuing and send has no address
// left to notify.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	now := w.Now()

	// The due-check compares against Postgres's own now(), not w.Now(),
	// mirroring every other Worker.ProcessPending in this codebase --
	// next_attempt_at's default is also Postgres's clock, and clock skew
	// against the Go process's could make a freshly-queued row look not
	// yet due.
	rows, err := tx.QueryContext(ctx,
		`SELECT id, identity_uid, kind, attempt_count
		 FROM session_notice_outbox
		 WHERE status = 'pending' AND next_attempt_at <= now()
		 ORDER BY next_attempt_at
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`,
		maxOutboxBatch,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("sessionnotice: query pending outbox rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pending []pendingRow
	for rows.Next() {
		var r pendingRow
		if err := rows.Scan(&r.id, &r.identityUID, &r.kind, &r.attemptCount); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return fmt.Errorf("sessionnotice: scan outbox row: %w", err)
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return fmt.Errorf("sessionnotice: iterate outbox rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		// coverage:ignore reason: DB row close failure, not exercised by unit tests
		return fmt.Errorf("sessionnotice: close outbox rows: %w", err)
	}

	for _, r := range pending {
		email, found, err := staffEmail(ctx, tx, r.identityUID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
		if !found {
			if err := w.markSent(ctx, tx, r.id, now); err != nil {
				// coverage:ignore reason: DB update failure, not exercised by unit tests
				return err
			}
			continue
		}

		var subject, text string
		if r.kind == "session_revoked" {
			subject, text = sessionRevokedSubject, sessionRevokedText()
		} else {
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
			if err := w.markSent(ctx, tx, r.id, now); err != nil {
				// coverage:ignore reason: DB update failure, not exercised by unit tests
				return err
			}
			continue
		}
		if err := w.markFailed(ctx, tx, r.id, r.attemptCount, sendErr, now); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return err
		}
	}
	return nil
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

func (w Worker) markSent(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE session_notice_outbox SET status = 'sent', sent_at = $1, last_error = NULL WHERE id = $2`,
		now, id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("sessionnotice: mark outbox row sent: %w", err)
	}
	return nil
}

func (w Worker) markFailed(ctx context.Context, tx *sql.Tx, id string, attemptCount int, sendErr error, now time.Time) error {
	nextAttempt := attemptCount + 1
	if nextAttempt >= len(backoffSchedule) {
		if _, err := tx.ExecContext(ctx,
			`UPDATE session_notice_outbox SET status = 'dead_lettered', attempt_count = $1, last_error = $2 WHERE id = $3`,
			nextAttempt, sendErr.Error(), id,
		); err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return fmt.Errorf("sessionnotice: dead-letter outbox row: %w", err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE session_notice_outbox SET attempt_count = $1, next_attempt_at = $2, last_error = $3 WHERE id = $4`,
		nextAttempt, now.Add(backoffSchedule[nextAttempt-1]), sendErr.Error(), id,
	); err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("sessionnotice: schedule outbox retry: %w", err)
	}
	return nil
}
