package payments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// ConnectNudgeCooldownDays bounds how often a Practice's Owners can be
// asked to connect Stripe (#917, ADR-0035): one nudge per Practice per
// week, counted across every sender rather than per sender, because the
// thing being protected is the Owner's inbox and eleven Admins asking
// once each is eleven emails.
//
// A week, and not #343's own "one per episode": that bound is re-armed
// by a webhook, and a Practice with no Stripe account produces no
// webhooks at all, so there is no episode here to re-arm against. A
// bound with no re-arming rule would be "once, ever", which turns a
// second ask a fortnight later -- an entirely reasonable thing for a
// colleague to want -- into a refusal with no way out. A week is long
// enough that the second ask reads as "this is still not done" rather
// than as nagging, and short enough that a Practice that cannot invoice
// is not stuck for a month.
//
// Counted in days against Postgres's own clock rather than as a Go
// duration, so a sandbox running compressed time (simclock) moves the
// cooldown with everything else instead of freezing it at wall-clock
// speed.
const ConnectNudgeCooldownDays = 7

// MsgConnectNudgeAlreadyConnected is the refusal when a Practice's
// Stripe account already exists. Written for the person who pressed the
// control, per docs/api-design.md section 7: it says what changed rather
// than naming a rule she broke, because the ordinary way to meet it is
// an Owner finishing onboarding in another tab while the screen still
// showed the older answer.
const MsgConnectNudgeAlreadyConnected = "This Practice has already connected Stripe. Check the status again to see where it stands."

// MsgConnectNudgeTooSoon is the refusal inside the cooldown. It names
// the bound in the same breath as the refusal, so the reader learns the
// rule from the sentence rather than having to discover it by trying
// again tomorrow.
const MsgConnectNudgeTooSoon = "Every Practice Owner was emailed about this in the last week. Doula Cloud sends this reminder at most once a week, so there is nothing more to send right now."

// actionConnectNudgeSent records a Staff member asking every Owner to
// connect Stripe -- Practice-scoped, plain-string, the same shape
// actionBillingModeChanged and actionPaymentTermsChanged already use.
// This is the audit entry #917 asks for, alongside
// connect_nudge_outbox.requested_by_staff_id on the row itself: the
// ledger answers "who asked, and when" where a Practice already reads
// its own history, and the row answers it where the worker can see it.
const actionConnectNudgeSent = "stripe_connect_nudge_sent"

// connectNudgeSubject and connectNudgeText are the nudge's fixed copy.
// ADR-0009's content rule is unconditional and ADR-0011 removed its one
// exception: no Practice name, no Client detail, nothing identifying, in
// From, subject or body.
//
// The sender is not named either, and that is deliberate rather than an
// oversight of the same rule. Who asked is exactly the kind of
// identifying detail the rule keeps out of a Notification, and it has
// somewhere better to live: the Activity entry this send writes, which
// the Practice reads behind its own permission gate. "Someone at your
// Practice" is the whole of what the email says about it.
const connectNudgeSubject = "Doula Cloud: your Practice still has to connect Stripe"

func connectNudgeText(link string) string {
	return "Hello,\n\n" +
		"Someone at your Practice asked us to let you know that Stripe is not connected yet, so Clients cannot pay their invoices.\n\n" +
		"Connect it here:\n" +
		link + "\n\n" +
		"If you have questions, reply to this email.\n"
}

// errConnectNudgeAlreadyConnected and errConnectNudgeTooSoon are
// queueConnectNudge's two refusals, mapped to a response by the handler
// -- the same split payments already uses between a package-internal
// error and the sentence a person reads.
var (
	errConnectNudgeAlreadyConnected = errors.New("payments: practice has already connected stripe")
	errConnectNudgeTooSoon          = errors.New("payments: connect nudge is inside its cooldown")
)

// PostConnectNudgeHandler queues the "your Practice still has to connect
// Stripe" Platform Notification to every Owner the Practice currently
// has (#917, ADR-0035).
//
// Mounted for an Owner or an Admin -- the same pair ADR-0008 gives
// Stripe Connect state, since being allowed to see the account is
// unfinished is what makes a person able to say so. The mount carries no
// rule against an Owner sending it: an Owner nudging herself is a
// pointless errand rather than a permission violation, the screen never
// offers her the control, and inventing a role-inversion gate for it
// would be a new gate shape in this codebase earning nothing. What the
// boundary does enforce is the condition the nudge is about -- a
// Practice with a Stripe account is refused whatever the screen shows --
// and the bound on repeats.
//
// Must be mounted behind staffauth.Middleware.
func PostConnectNudgeHandler(enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		err := queueConnectNudge(r.Context(), tx, practiceID, staffID)
		switch {
		case errors.Is(err, errConnectNudgeAlreadyConnected):
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgConnectNudgeAlreadyConnected, nil)
			return
		case errors.Is(err, errConnectNudgeTooSoon):
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, MsgConnectNudgeTooSoon, nil)
			return
		case err != nil:
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.ConnectNudge))
		w.WriteHeader(http.StatusAccepted)
	})
}

// queueConnectNudge is the whole of the act: it confirms the Practice
// really has no Stripe account, confirms nobody has asked inside the
// cooldown, inserts the outbox row, and records who asked -- one path,
// on the caller's transaction, so the row and its Activity entry can
// never disagree about what happened.
//
// The practices row is locked for the rest of the transaction before
// either check, so two Admins pressing the control in the same second
// serialize here rather than both reading "no account, nobody asked" and
// both queueing. The partial unique index on the table is the second
// guard behind that one.
func queueConnectNudge(ctx context.Context, tx *sql.Tx, practiceID, staffID string) error {
	var accountID sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT stripe_connect_account_id FROM practices WHERE id = $1 FOR UPDATE`, practiceID,
	).Scan(&accountID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: read connect account for nudge: %w", err)
	}
	if accountID.Valid {
		return errConnectNudgeAlreadyConnected
	}

	var recentlyAsked bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM connect_nudge_outbox
			WHERE practice_id = $1 AND created_at > now() - make_interval(days => $2)
		 )`,
		practiceID, ConnectNudgeCooldownDays,
	).Scan(&recentlyAsked); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: check connect nudge cooldown: %w", err)
	}
	if recentlyAsked {
		return errConnectNudgeTooSoon
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO connect_nudge_outbox (practice_id, requested_by_staff_id) VALUES ($1, $2)`,
		practiceID, staffID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: queue connect nudge: %w", err)
	}

	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      actionConnectNudgeSent,
		Actor:       activity.StaffActor(staffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: record connect nudge: %w", err)
	}
	return nil
}

// ConnectNudgeWorker sends due connect_nudge_outbox rows -- the
// Cloud-Scheduler-driven half of ADR-0010's outbox for #917, and a third
// distinct type beside Worker (payout_outbox) and PaymentReceivedWorker
// for the reason those two are already distinct from each other: the
// table, the recipients, the send-time recheck and the copy are all its
// own.
//
// Hand-written rather than built on outbox.MailWorker[R] (#839) for the
// same reason Worker is: it mails every Owner the Practice currently has
// through the multi-recipient SendAll path, not one fixed address a
// Compose function could return.
type ConnectNudgeWorker struct {
	outbox.Mailer
}

func (w ConnectNudgeWorker) inner() outbox.Worker {
	return w.Worker("connect_nudge_outbox")
}

type connectNudgePendingRow struct {
	id           string
	practiceID   string
	attemptCount int
}

const connectNudgeClaimQuery = `SELECT id, practice_id, attempt_count
	 FROM connect_nudge_outbox
	 WHERE status = 'pending' AND next_attempt_at <= now()
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanConnectNudgeRow(rows *sql.Rows) (connectNudgePendingRow, error) {
	var r connectNudgePendingRow
	err := rows.Scan(&r.id, &r.practiceID, &r.attemptCount)
	return r, wrapOutboxErr(err)
}

// ProcessPending sends every due connect_nudge_outbox row within tx.
func (w ConnectNudgeWorker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	return wrapOutboxErr(outbox.ProcessPending(ctx, tx, w.inner(), connectNudgeClaimQuery, scanConnectNudgeRow, w.send))
}

// send mails one claimed row, rechecking the live condition first.
//
// The recheck is Worker.send's own shape (#343's requirementsStill-
// Outstanding) applied to this row's own condition: a Practice that
// connected Stripe between the press and the send -- a retry after a
// Mailgun failure can be up to a day later -- is marked sent with
// nothing mailed, rather than telling every Owner to do something that
// is already done.
func (w ConnectNudgeWorker) send(ctx context.Context, tx *sql.Tx, inner outbox.Worker, r connectNudgePendingRow, now time.Time) error {
	stillUnconnected, err := connectAccountMissing(ctx, tx, r.practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if !stillUnconnected {
		return wrapOutboxErr(inner.MarkSent(ctx, tx, r.id, now))
	}

	emails, err := ownerEmails(ctx, tx, r.practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	link := w.AppBaseURL + "/practices/" + r.practiceID + "/settings/payments"
	return wrapOutboxErr(inner.SendAll(ctx, tx, r.id, r.attemptCount, now, emails, connectNudgeSubject, connectNudgeText(link)))
}

// connectAccountMissing reports whether practiceID still has no Stripe
// Connect account right now -- the live read of exactly the condition
// GetConnectStatusHandler reports as not_connected.
func connectAccountMissing(ctx context.Context, tx *sql.Tx, practiceID string) (bool, error) {
	var accountID sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT stripe_connect_account_id FROM practices WHERE id = $1`, practiceID,
	).Scan(&accountID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("payments: recheck connect account: %w", err)
	}
	return !accountID.Valid, nil
}
