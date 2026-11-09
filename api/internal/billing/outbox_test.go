package billing_test

import (
	"errors"
	"slices"
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
	testLowCreditStatusDead    = "dead_lettered"
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
	id := testdb.Scalar[string](t, db.Admin, `INSERT INTO low_credit_outbox (practice_id, attempt_count, next_attempt_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		practiceID, attemptCount, nextAttemptAt)
	return id
}

// runLowCreditWorker runs ProcessPending the way outbox.ProcessHandler
// drives it, behind its trusted session var and without HTTP.
func runLowCreditWorker(t *testing.T, db *testdb.DB, w billing.Worker) {
	t.Helper()
	db.RunAsNotificationWorker(t, w.ProcessPending)
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

func TestShouldQueueOutOfCreditsNotification(t *testing.T) {
	cases := []struct {
		name string
		seed func(t *testing.T, db *testdb.DB, practiceID string)
		want bool
	}{
		{name: "true with no prior row", seed: func(*testing.T, *testdb.DB, string) {}, want: true},
		{
			name: "false with a row from the same episode and no purchase since",
			seed: func(t *testing.T, db *testdb.DB, practiceID string) {
				seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now())
			},
		},
		{
			name: "true again once a purchase follows the prior row",
			seed: func(t *testing.T, db *testdb.DB, practiceID string) {
				seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now())
				seedLedgerRow(t, db, practiceID, "purchase", 1)
			},
			want: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := testdb.New(t)
			practiceID := testdb.SeedPractice(t, db, "Dedupe")
			c.seed(t, db, practiceID)
			tx := beginAsPractice(t, db, practiceID)
			defer func() { _ = tx.Rollback() }()

			should, err := billing.ShouldQueueOutOfCreditsNotification(t.Context(), tx, practiceID)
			if err != nil {
				t.Fatalf("ShouldQueueOutOfCreditsNotification: %v", err)
			}
			if should != c.want {
				t.Fatalf("should queue = %v, want %v", should, c.want)
			}
		})
	}
}

func TestQueueOutOfCreditsNotification(t *testing.T) {
	cases := []struct {
		name string
		seed func(t *testing.T, db *testdb.DB, practiceID string)
	}{
		{name: "inserts a pending row", seed: func(*testing.T, *testdb.DB, string) {}},
		{
			name: "an existing pending row is left alone",
			seed: func(t *testing.T, db *testdb.DB, practiceID string) {
				seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now())
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := testdb.New(t)
			practiceID := testdb.SeedPractice(t, db, "Queue")
			c.seed(t, db, practiceID)

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
			if count != 1 || status != testLowCreditStatusPending {
				t.Fatalf("count/status = %d/%q, want 1/%s", count, status, testLowCreditStatusPending)
			}
		})
	}
}

func TestLowCreditWorker_ProcessPending_MailsEveryOwnerAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "owner-one", []string{ownerRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, "owner-two", []string{ownerRole}, "employee")
	// A non-Owner Staff member at the same Practice should never be mailed.
	testdb.SeedStaffAtPractice(t, db, practiceID, "doula-bystander", []string{doulaRole}, "employee")
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
	var recipients []string
	for _, msg := range sent {
		recipients = append(recipients, msg.To)
		if msg.Subject != "DoulaCloud: your Practice is out of Credits" || !strings.Contains(msg.Text, wantLink) {
			t.Fatalf("message to %q = %q / %q, want the out-of-Credits subject and %q", msg.To, msg.Subject, msg.Text, wantLink)
		}
	}
	slices.Sort(recipients)
	if want := []string{"owner-one@example.com", "owner-two@example.com"}; !slices.Equal(recipients, want) {
		t.Fatalf("mailed %v, want exactly the two Owners %v", recipients, want)
	}
}

// TestLowCreditWorker_ProcessPending_Outcomes is what one Scheduler tick
// leaves on a row, given who the Practice has to mail and how the send
// goes.
func TestLowCreditWorker_ProcessPending_Outcomes(t *testing.T) {
	mailgunDown := errors.New("mailgun unavailable")
	cases := []struct {
		name string
		// owners is how many Owners the Practice has.
		owners   int
		attempts int
		dueIn    time.Duration
		sendErr  error

		wantStatus   string
		wantAttempts int
		wantReason   string
	}{
		{name: "a row not yet due is skipped", owners: 1, dueIn: time.Hour, wantStatus: testLowCreditStatusPending},
		{name: "no Owners is marked sent with no mail", dueIn: -time.Minute, wantStatus: testLowCreditStatusSent},
		{
			name: "a send failure schedules a retry", owners: 1, dueIn: -time.Minute, sendErr: mailgunDown,
			wantStatus: testLowCreditStatusPending, wantAttempts: 1,
		},
		{
			// One attempt short of the schedule's length: this failure is
			// the last one before dead-letter.
			name: "a failure on the final attempt dead-letters", owners: 1, attempts: 4, dueIn: -time.Minute, sendErr: mailgunDown,
			wantStatus: testLowCreditStatusDead, wantAttempts: 5, wantReason: "retries_exhausted",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := testdb.New(t)
			practiceID := testdb.SeedPractice(t, db, "Worker Outcomes")
			for i := range c.owners {
				testdb.SeedStaffAtPractice(t, db, practiceID, "outcome-owner-"+string(rune('a'+i)), []string{ownerRole}, "employee")
			}
			outboxID := seedLowCreditOutboxRow(t, db, practiceID, c.attempts, time.Now().Add(c.dueIn))
			sender := &mail.FakeSender{Err: c.sendErr}

			runLowCreditWorker(t, db, newTestWorker(sender))

			status, attemptCount := lowCreditOutboxRowState(t, db, outboxID)
			if status != c.wantStatus || attemptCount != c.wantAttempts {
				t.Errorf("status/attempt_count = %q/%d, want %s/%d", status, attemptCount, c.wantStatus, c.wantAttempts)
			}
			if len(sender.Sent()) != 0 {
				t.Errorf("delivered %d messages, want none", len(sender.Sent()))
			}
			if got := testdb.DeadLetterReason(t, db, "low_credit_outbox", outboxID); got != c.wantReason {
				t.Errorf("dead_letter_reason = %q, want %q", got, c.wantReason)
			}
		})
	}
}
