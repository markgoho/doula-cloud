package payments_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

const (
	payoutOutboxTable  = "payout_outbox"
	paymentOutboxTable = "payment_received_outbox"

	testPayoutStatusPending = "pending"
	testPayoutStatusSent    = "sent"
	testOutboxStatusSkipped = "skipped"
	testPayoutAppBaseURL    = "https://app.example.test" //nolint:gosec // test fixture URL, not a credential
	// testRequirementDOB is a real Stripe requirement path, standing in
	// across this package's payout-outbox tests for "something is
	// outstanding" -- its content never matters beyond that.
	testRequirementDOB = "individual.dob"
	// The Platform-voice sending identity every outbox worker in this
	// package's tests is built with -- named once now that three workers
	// (payout, payment-received and #917's connect nudge) each need it.
	testOutboxFrom    = "a@b.test"
	testOutboxReplyTo = "support@b.test"
)

// newTestPayoutWorker builds a payments.Worker around sender with this
// file's stand-in AppBaseURL/From/ReplyTo -- every outbox test needs
// one, only the injected Sender and (occasionally) Now vary.
func newTestPayoutWorker(sender mail.Sender) payments.Worker {
	return payments.Worker{Sender: sender, Now: time.Now, AppBaseURL: testPayoutAppBaseURL, From: testOutboxFrom, ReplyTo: testOutboxReplyTo}
}

// seedPayoutOutboxRow inserts a pending payout_outbox row for
// practiceID with the given attempt_count/next_attempt_at, using the
// superuser Admin connection -- the table carries no RLS (00034).
func seedPayoutOutboxRow(t *testing.T, db *testdb.DB, practiceID string, attemptCount int, nextAttemptAt time.Time) string {
	t.Helper()
	id := testdb.Scalar[string](t, db.Admin, `INSERT INTO payout_outbox (practice_id, attempt_count, next_attempt_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		practiceID, attemptCount, nextAttemptAt)
	return id
}

// worker is what an outbox worker in this package offers the test.
type worker interface {
	ProcessPending(ctx context.Context, tx *sql.Tx) error
}

// runWorker begins a tx on db.App, sets the trusted session var
// outbox.ProcessHandler would otherwise set after its own secret check,
// runs ProcessPending, and commits.
func runWorker(t *testing.T, db *testdb.DB, w worker) {
	t.Helper()
	db.RunAsWorker(t, w.ProcessPending)
}

// outboxRowState reads a row of one of the outbox tables directly.
func outboxRowState(t *testing.T, db *testdb.DB, table, id string) (status string, attemptCount int) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count FROM `+table+` WHERE id = $1`, id,
	).Scan(&status, &attemptCount); err != nil {
		t.Fatalf("query %s row: %v", table, err)
	}
	return status, attemptCount
}

// setRequirementsDue seeds practiceID's live stripe_connect_requirements_due
// directly, standing in for what handleCapabilityStatusUpdated would have
// written from a real webhook delivery.
func setRequirementsDue(t *testing.T, db *testdb.DB, practiceID string, requirements []string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_requirements_due = $1 WHERE id = $2`, requirements, practiceID,
	); err != nil {
		t.Fatalf("set requirements due: %v", err)
	}
}

func TestQueuePayoutIncompleteNotification_InsertsPendingRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Queue Insert")

	tx := db.BeginAs(t)

	// The grace window is counted from clock.Now(ctx) (#1992), so a fixed
	// clock pins next_attempt_at exactly.
	queuedAt := time.Now().Add(-time.Hour).Truncate(time.Microsecond).UTC()
	ctx := clock.Into(t.Context(), func() time.Time { return queuedAt })
	if err := payments.QueuePayoutIncompleteNotification(ctx, tx, practiceID); err != nil {
		t.Fatalf("QueuePayoutIncompleteNotification: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	var status string
	var nextAttemptAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(status), max(next_attempt_at) FROM payout_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&count, &status, &nextAttemptAt); err != nil {
		t.Fatalf("query payout_outbox: %v", err)
	}
	if count != 1 || status != testPayoutStatusPending {
		t.Fatalf("count/status = %d/%q, want 1/%s", count, status, testPayoutStatusPending)
	}
	if want := queuedAt.Add(48 * time.Hour); !nextAttemptAt.Equal(want) {
		t.Fatalf("next_attempt_at = %v, want the 48-hour grace window after the clock, %v", nextAttemptAt, want)
	}
}

// TestQueuePayoutIncompleteNotification_GraceWindowDefersTheSend pins the
// behavioral consequence of that grace window: a row queued this instant is
// not due yet, so running the worker immediately after must mail no one
// and leave the row pending -- the whole point of the grace window (see
// 00034_payout_outbox.sql's reasoning) is that an Owner mid-onboarding is
// never mailed the moment requirements first appear.
func TestQueuePayoutIncompleteNotification_GraceWindowDefersTheSend(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "owner-grace-window", []string{ownerRole}, "employee")
	setRequirementsDue(t, db, practiceID, []string{testRequirementDOB})

	queuePayoutIncomplete(t, db, practiceID)

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestPayoutWorker(sender))

	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent immediately after queuing, got %d", len(sender.Sent()))
	}
	status := testdb.Scalar[string](t, db.Admin, `SELECT status FROM payout_outbox WHERE practice_id = $1`, practiceID)
	if status != testPayoutStatusPending {
		t.Fatalf("status = %q, want %s (not yet due)", status, testPayoutStatusPending)
	}
}

func TestQueuePayoutIncompleteNotification_ConflictOnExistingPendingRowIsNoop(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Queue Conflict")
	seedPayoutOutboxRow(t, db, practiceID, 0, time.Now())

	queuePayoutIncomplete(t, db, practiceID)

	count := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM payout_outbox WHERE practice_id = $1`, practiceID)
	if count != 1 {
		t.Fatalf("count = %d, want 1 (the existing pending row, untouched by DO NOTHING)", count)
	}
}

func TestPayoutWorker_ProcessPending_MailsEveryOwnerAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "owner-one", []string{ownerRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, "owner-two", []string{ownerRole}, "employee")
	// A non-Owner Staff member at the same Practice should never be mailed.
	testdb.SeedStaffAtPractice(t, db, practiceID, "doula-bystander", []string{doulaRole}, "employee")
	setRequirementsDue(t, db, practiceID, []string{testRequirementDOB})
	outboxID := seedPayoutOutboxRow(t, db, practiceID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestPayoutWorker(sender))

	status, _ := outboxRowState(t, db, payoutOutboxTable, outboxID)
	if status != testPayoutStatusSent {
		t.Fatalf("status = %q, want %s", status, testPayoutStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("sent %d messages, want 2 (one per Owner)", len(sent))
	}
	wantLink := testPayoutAppBaseURL + "/practices/" + practiceID + "/settings/payments"
	wantRecipients := map[string]bool{"owner-one@example.com": false, "owner-two@example.com": false}
	for _, msg := range sent {
		if msg.Subject != "DoulaCloud: your Practice's payout account needs more information" {
			t.Fatalf("subject = %q", msg.Subject)
		}
		if _, ok := wantRecipients[msg.To]; !ok {
			t.Fatalf("To = %q, want one of the two Owners", msg.To)
		}
		wantRecipients[msg.To] = true
		if !strings.Contains(msg.Text, wantLink) {
			t.Fatalf("body %q does not contain link %q", msg.Text, wantLink)
		}
	}
	for to, got := range wantRecipients {
		if !got {
			t.Fatalf("Owner %q was never mailed", to)
		}
	}
}

// TestPayoutWorker_ProcessPending_Outcomes proves what the worker does to
// a row it finds: it leaves one not yet due; marks one sent with no mail
// when the Practice has no Owner, or when the requirements cleared before
// the row came due (the grace-window recheck: an Owner who finishes
// onboarding is never mailed at all); retries on a send failure; and
// dead-letters on the final attempt.
func TestPayoutWorker_ProcessPending_Outcomes(t *testing.T) {
	errMailgun := errors.New("mailgun unavailable")
	for _, tc := range []struct {
		name         string
		noOwner      bool
		requirements []string
		attemptCount int
		due          time.Duration
		sendErr      error
		wantStatus   string
		wantAttempts int
	}{
		{name: "a row not yet due is skipped", due: time.Hour, requirements: []string{testRequirementDOB}, wantStatus: testPayoutStatusPending},
		{name: "a Practice with no Owner is marked skipped with no mail", noOwner: true, requirements: []string{testRequirementDOB}, due: -time.Minute, wantStatus: testOutboxStatusSkipped},
		{name: "requirements already cleared skip the mail", requirements: []string{}, due: -time.Minute, wantStatus: testOutboxStatusSkipped},
		{name: "a send failure retries", requirements: []string{testRequirementDOB}, due: -time.Minute, sendErr: errMailgun, wantStatus: testPayoutStatusPending, wantAttempts: 1},
		// One attempt short of the schedule's length -- this failure is the
		// last one before dead-letter.
		{name: "the final attempt dead-letters", requirements: []string{testRequirementDOB}, attemptCount: 4, due: -time.Minute, sendErr: errMailgun, wantStatus: statusDeadLettered, wantAttempts: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			staff := fixture.Owner()
			if tc.noOwner {
				staff = fixture.NoRole()
			}
			e := newEnv(t, signedInAs(staff))
			setRequirementsDue(t, e.DB, e.Practice.ID, tc.requirements)
			outboxID := seedPayoutOutboxRow(t, e.DB, e.Practice.ID, tc.attemptCount, time.Now().Add(tc.due))

			sender := &mail.FakeSender{Err: tc.sendErr}
			runWorker(t, e.DB, newTestPayoutWorker(sender))

			status, attemptCount := outboxRowState(t, e.DB, payoutOutboxTable, outboxID)
			if status != tc.wantStatus || attemptCount != tc.wantAttempts {
				t.Fatalf("status/attempt_count = %q/%d, want %s/%d", status, attemptCount, tc.wantStatus, tc.wantAttempts)
			}
			if len(sender.Sent()) != 0 {
				t.Fatalf("sent %d messages, want none", len(sender.Sent()))
			}
			if tc.wantStatus == statusDeadLettered {
				if got := testdb.DeadLetterReason(t, e.DB, "payout_outbox", outboxID); got != "retries_exhausted" {
					t.Fatalf("dead_letter_reason = %q, want retries_exhausted", got)
				}
			}
		})
	}
}

// queuePayoutIncomplete queues the payout-incomplete notice for practiceID
// in a transaction of its own and commits it.
func queuePayoutIncomplete(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	tx := db.BeginAs(t)
	if err := payments.QueuePayoutIncompleteNotification(t.Context(), tx, practiceID); err != nil {
		t.Fatalf("QueuePayoutIncompleteNotification: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
