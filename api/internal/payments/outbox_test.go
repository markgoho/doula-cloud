package payments_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

const (
	testPayoutStatusPending = "pending"
	testPayoutStatusSent    = "sent"
	testPayoutAppBaseURL    = "https://app.example.test" //nolint:gosec // test fixture URL, not a credential
	// testRequirementDOB is a real Stripe requirement path, standing in
	// across this package's payout-outbox tests for "something is
	// outstanding" -- its content never matters beyond that.
	testRequirementDOB = "individual.dob"
)

// newTestPayoutWorker builds a payments.Worker around sender with this
// file's stand-in AppBaseURL/From/ReplyTo -- every outbox test needs
// one, only the injected Sender and (occasionally) Now vary.
func newTestPayoutWorker(sender mail.Sender) payments.Worker {
	return payments.Worker{Sender: sender, Now: time.Now, AppBaseURL: testPayoutAppBaseURL, From: "a@b.test", ReplyTo: "support@b.test"}
}

// seedPayoutOutboxRow inserts a pending payout_outbox row for
// practiceID with the given attempt_count/next_attempt_at, using the
// superuser Admin connection -- the table carries no RLS (00034).
func seedPayoutOutboxRow(t *testing.T, db *testdb.DB, practiceID string, attemptCount int, nextAttemptAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO payout_outbox (practice_id, attempt_count, next_attempt_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		practiceID, attemptCount, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed payout_outbox row: %v", err)
	}
	return id
}

// runPayoutWorker begins a tx on db.App, sets the trusted session var
// payments.ProcessOutboxHandler would otherwise set after its own secret
// check, runs ProcessPending, and commits.
func runPayoutWorker(t *testing.T, db *testdb.DB, w payments.Worker) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("set trusted session var: %v", err)
	}
	if err := w.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func payoutOutboxRowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count FROM payout_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount); err != nil {
		t.Fatalf("query payout_outbox row: %v", err)
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
	practiceID := seedPractice(t, db, "Queue Insert")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if err := payments.QueuePayoutIncompleteNotification(t.Context(), tx, practiceID); err != nil {
		t.Fatalf("QueuePayoutIncompleteNotification: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	var status string
	var inGraceWindow bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(status), bool_and(next_attempt_at BETWEEN now() + interval '47 hours' AND now() + interval '49 hours')
		 FROM payout_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&count, &status, &inGraceWindow); err != nil {
		t.Fatalf("query payout_outbox: %v", err)
	}
	if count != 1 || status != testPayoutStatusPending {
		t.Fatalf("count/status = %d/%q, want 1/%s", count, status, testPayoutStatusPending)
	}
	if !inGraceWindow {
		t.Fatal("next_attempt_at is not ~48 hours out -- the grace window (00034's column default) did not apply")
	}
}

// TestQueuePayoutIncompleteNotification_GraceWindowDefersTheSend pins the
// behavioral consequence of that default: a row queued this instant is
// not due yet, so running the worker immediately after must mail no one
// and leave the row pending -- the whole point of the grace window (see
// 00034_payout_outbox.sql's reasoning) is that an Owner mid-onboarding is
// never mailed the moment requirements first appear.
func TestQueuePayoutIncompleteNotification_GraceWindowDefersTheSend(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-grace-window")
	setRequirementsDue(t, db, practiceID, []string{testRequirementDOB})

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := payments.QueuePayoutIncompleteNotification(t.Context(), tx, practiceID); err != nil {
		t.Fatalf("QueuePayoutIncompleteNotification: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	sender := &mail.FakeSender{}
	runPayoutWorker(t, db, newTestPayoutWorker(sender))

	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent immediately after queuing, got %d", len(sender.Sent()))
	}
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status FROM payout_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&status); err != nil {
		t.Fatalf("query payout_outbox: %v", err)
	}
	if status != testPayoutStatusPending {
		t.Fatalf("status = %q, want %s (not yet due)", status, testPayoutStatusPending)
	}
}

func TestQueuePayoutIncompleteNotification_ConflictOnExistingPendingRowIsNoop(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Queue Conflict")
	seedPayoutOutboxRow(t, db, practiceID, 0, time.Now())

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if err := payments.QueuePayoutIncompleteNotification(t.Context(), tx, practiceID); err != nil {
		t.Fatalf("QueuePayoutIncompleteNotification: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM payout_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("query payout_outbox: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (the existing pending row, untouched by DO NOTHING)", count)
	}
}

func TestPayoutWorker_ProcessPending_MailsEveryOwnerAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-one")
	secondOwnerStaffID := seedStaff(t, db, "owner-two")
	seedMembership(t, db, practiceID, secondOwnerStaffID, "{owner}")
	// A non-Owner Staff member at the same Practice should never be mailed.
	doulaStaffID := seedStaff(t, db, "doula-bystander")
	seedMembership(t, db, practiceID, doulaStaffID, "{doula}")
	setRequirementsDue(t, db, practiceID, []string{testRequirementDOB})
	outboxID := seedPayoutOutboxRow(t, db, practiceID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runPayoutWorker(t, db, newTestPayoutWorker(sender))

	status, _ := payoutOutboxRowState(t, db, outboxID)
	if status != testPayoutStatusSent {
		t.Fatalf("status = %q, want %s", status, testPayoutStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("sent %d messages, want 2 (one per Owner)", len(sent))
	}
	wantLink := testPayoutAppBaseURL + "/practices/" + practiceID + "/settings/payments"
	for _, msg := range sent {
		if msg.Subject != "Doula Cloud: your Practice's payout account needs more information" {
			t.Fatalf("subject = %q", msg.Subject)
		}
		if msg.To != "staff@example.com" {
			t.Fatalf("To = %q", msg.To)
		}
		if !strings.Contains(msg.Text, wantLink) {
			t.Fatalf("body %q does not contain link %q", msg.Text, wantLink)
		}
	}
}

func TestPayoutWorker_ProcessPending_SkipsRowNotYetDue(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-not-due")
	setRequirementsDue(t, db, practiceID, []string{testRequirementDOB})
	outboxID := seedPayoutOutboxRow(t, db, practiceID, 0, time.Now().Add(time.Hour))

	sender := &mail.FakeSender{}
	runPayoutWorker(t, db, newTestPayoutWorker(sender))

	status, _ := payoutOutboxRowState(t, db, outboxID)
	if status != testPayoutStatusPending {
		t.Fatalf("status = %q, want %s (not due yet)", status, testPayoutStatusPending)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no send for a not-yet-due row")
	}
}

func TestPayoutWorker_ProcessPending_ZeroOwnersMarksSentWithNoMail(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Ownerless Practice")
	setRequirementsDue(t, db, practiceID, []string{testRequirementDOB})
	outboxID := seedPayoutOutboxRow(t, db, practiceID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runPayoutWorker(t, db, newTestPayoutWorker(sender))

	status, _ := payoutOutboxRowState(t, db, outboxID)
	if status != testPayoutStatusSent {
		t.Fatalf("status = %q, want %s", status, testPayoutStatusSent)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent when the Practice has no Owner")
	}
}

// TestPayoutWorker_ProcessPending_RequirementsAlreadyClearedSkipsMail
// proves the grace-window recheck: an Owner who finishes onboarding
// before the row comes due is never mailed at all, even though a row is
// still sitting there pending.
func TestPayoutWorker_ProcessPending_RequirementsAlreadyClearedSkipsMail(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-cleared")
	setRequirementsDue(t, db, practiceID, []string{})
	outboxID := seedPayoutOutboxRow(t, db, practiceID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runPayoutWorker(t, db, newTestPayoutWorker(sender))

	status, _ := payoutOutboxRowState(t, db, outboxID)
	if status != testPayoutStatusSent {
		t.Fatalf("status = %q, want %s", status, testPayoutStatusSent)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent once requirements cleared before the row came due")
	}
}

func TestPayoutWorker_ProcessPending_RetriesOnSendFailure(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-retry")
	setRequirementsDue(t, db, practiceID, []string{testRequirementDOB})
	outboxID := seedPayoutOutboxRow(t, db, practiceID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runPayoutWorker(t, db, newTestPayoutWorker(sender))

	status, attemptCount := payoutOutboxRowState(t, db, outboxID)
	if status != testPayoutStatusPending || attemptCount != 1 {
		t.Fatalf("status/attempt_count = %q/%d, want %s/1", status, attemptCount, testPayoutStatusPending)
	}
}

func TestPayoutWorker_ProcessPending_DeadLettersAfterFinalAttempt(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-dead-letter")
	setRequirementsDue(t, db, practiceID, []string{testRequirementDOB})
	// One attempt short of the schedule's length -- this failure is the
	// last one before dead-letter.
	outboxID := seedPayoutOutboxRow(t, db, practiceID, 4, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runPayoutWorker(t, db, newTestPayoutWorker(sender))

	status, attemptCount := payoutOutboxRowState(t, db, outboxID)
	if status != "dead_lettered" || attemptCount != 5 {
		t.Fatalf("status/attempt_count = %q/%d, want dead_lettered/5", status, attemptCount)
	}
}
