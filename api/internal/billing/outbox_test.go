package billing_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	testLowCreditStatusPending = "pending"
	testLowCreditStatusSent    = "sent"
	testLowCreditAppBaseURL    = "https://app.example.test" //nolint:gosec // test fixture URL, not a credential
	testLowCreditSenderAddr    = "a@b.test"
)

// newTestWorker builds a billing.Worker around sender with this file's
// stand-in AppBaseURL/From/ReplyTo -- every outbox test needs one, only
// the injected Sender and (occasionally) Now vary.
func newTestWorker(sender mail.Sender) billing.Worker {
	return billing.Worker{Sender: sender, Now: time.Now, AppBaseURL: testLowCreditAppBaseURL, From: testLowCreditSenderAddr, ReplyTo: "support@b.test"}
}

// seedLowCreditOutboxRow inserts a pending low_credit_outbox row for
// practiceID with the given attempt_count/next_attempt_at, using the
// superuser Admin connection -- the table carries no RLS (00033).
func seedLowCreditOutboxRow(t *testing.T, db *testdb.DB, practiceID string, attemptCount int, nextAttemptAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO low_credit_outbox (practice_id, attempt_count, next_attempt_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		practiceID, attemptCount, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed low_credit_outbox row: %v", err)
	}
	return id
}

// runLowCreditWorker begins a tx on db.App, sets the trusted session var
// billing.ProcessOutboxHandler would otherwise set after its own secret
// check, runs ProcessPending, and commits.
func runLowCreditWorker(t *testing.T, db *testdb.DB, w billing.Worker) {
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

func lowCreditOutboxRowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count FROM low_credit_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount); err != nil {
		t.Fatalf("query low_credit_outbox row: %v", err)
	}
	return status, attemptCount
}

func TestShouldQueueOutOfCreditsNotification_TrueWithNoPriorRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Dedupe First Episode")
	tx := beginAsPractice(t, db, practiceID)
	defer tx.Rollback()

	should, err := billing.ShouldQueueOutOfCreditsNotification(t.Context(), tx, practiceID)
	if err != nil {
		t.Fatalf("ShouldQueueOutOfCreditsNotification: %v", err)
	}
	if !should {
		t.Fatal("expected true with no prior low_credit_outbox row")
	}
}

func TestShouldQueueOutOfCreditsNotification_FalseWithRowFromSameEpisode(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Dedupe Same Episode")
	seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now())
	tx := beginAsPractice(t, db, practiceID)
	defer tx.Rollback()

	should, err := billing.ShouldQueueOutOfCreditsNotification(t.Context(), tx, practiceID)
	if err != nil {
		t.Fatalf("ShouldQueueOutOfCreditsNotification: %v", err)
	}
	if should {
		t.Fatal("expected false: a row from this episode already exists and no purchase followed it")
	}
}

func TestShouldQueueOutOfCreditsNotification_TrueAfterPurchaseFollowsPriorRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Dedupe Reset By Purchase")
	seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now())
	seedLedgerRow(t, db, practiceID, "purchase", 1)
	tx := beginAsPractice(t, db, practiceID)
	defer tx.Rollback()

	should, err := billing.ShouldQueueOutOfCreditsNotification(t.Context(), tx, practiceID)
	if err != nil {
		t.Fatalf("ShouldQueueOutOfCreditsNotification: %v", err)
	}
	if !should {
		t.Fatal("expected true: a purchase after the last row re-arms the notification")
	}
}

func TestQueueOutOfCreditsNotification_InsertsPendingRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Queue Insert")

	if err := billing.QueueOutOfCreditsNotification(t.Context(), db.App, practiceID, &tasknudge.FakeEnqueuer{}); err != nil {
		t.Fatalf("QueueOutOfCreditsNotification: %v", err)
	}

	var count int
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(status) FROM low_credit_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&count, &status); err != nil {
		t.Fatalf("query low_credit_outbox: %v", err)
	}
	if count != 1 || status != "pending" {
		t.Fatalf("count/status = %d/%q, want 1/pending", count, status)
	}
}

func TestQueueOutOfCreditsNotification_ConflictOnExistingPendingRowIsNoop(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Queue Conflict")
	seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now())

	if err := billing.QueueOutOfCreditsNotification(t.Context(), db.App, practiceID, &tasknudge.FakeEnqueuer{}); err != nil {
		t.Fatalf("QueueOutOfCreditsNotification: %v", err)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM low_credit_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("query low_credit_outbox: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (the existing pending row, untouched by DO NOTHING)", count)
	}
}

func TestLowCreditWorker_ProcessPending_MailsEveryOwnerAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-one")
	secondOwnerStaffID := seedStaff(t, db, "owner-two")
	seedMembership(t, db, practiceID, secondOwnerStaffID, "{owner}")
	// A non-Owner Staff member at the same Practice should never be mailed.
	doulaStaffID := seedStaff(t, db, "doula-bystander")
	seedMembership(t, db, practiceID, doulaStaffID, "{doula}")
	outboxID := seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runLowCreditWorker(t, db, newTestWorker(sender))

	status, _ := lowCreditOutboxRowState(t, db, outboxID)
	if status != testLowCreditStatusSent {
		t.Fatalf("status = %q, want %s", status, testLowCreditStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("sent %d messages, want 2 (one per Owner)", len(sent))
	}
	wantLink := testLowCreditAppBaseURL + "/practices/" + practiceID + "/billing"
	for _, msg := range sent {
		if msg.Subject != "Doula Cloud: your Practice is out of Credits" {
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

func TestLowCreditWorker_ProcessPending_SkipsRowNotYetDue(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-not-due")
	outboxID := seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now().Add(time.Hour))

	sender := &mail.FakeSender{}
	runLowCreditWorker(t, db, newTestWorker(sender))

	status, _ := lowCreditOutboxRowState(t, db, outboxID)
	if status != testLowCreditStatusPending {
		t.Fatalf("status = %q, want %s (not due yet)", status, testLowCreditStatusPending)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no send for a not-yet-due row")
	}
}

func TestLowCreditWorker_ProcessPending_ZeroOwnersMarksSentWithNoMail(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Ownerless Practice")
	outboxID := seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runLowCreditWorker(t, db, newTestWorker(sender))

	status, _ := lowCreditOutboxRowState(t, db, outboxID)
	if status != testLowCreditStatusSent {
		t.Fatalf("status = %q, want %s", status, testLowCreditStatusSent)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent when the Practice has no Owner")
	}
}

func TestLowCreditWorker_ProcessPending_RetriesOnSendFailure(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-retry")
	outboxID := seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runLowCreditWorker(t, db, newTestWorker(sender))

	status, attemptCount := lowCreditOutboxRowState(t, db, outboxID)
	if status != testLowCreditStatusPending || attemptCount != 1 {
		t.Fatalf("status/attempt_count = %q/%d, want %s/1", status, attemptCount, testLowCreditStatusPending)
	}
}

func TestLowCreditWorker_ProcessPending_DeadLettersAfterFinalAttempt(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-dead-letter")
	// One attempt short of the schedule's length -- this failure is the
	// last one before dead-letter.
	outboxID := seedLowCreditOutboxRow(t, db, practiceID, 4, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runLowCreditWorker(t, db, newTestWorker(sender))

	status, attemptCount := lowCreditOutboxRowState(t, db, outboxID)
	if status != "dead_lettered" || attemptCount != 5 {
		t.Fatalf("status/attempt_count = %q/%d, want dead_lettered/5", status, attemptCount)
	}
}
