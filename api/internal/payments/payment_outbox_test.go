package payments_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

const (
	testPaymentOutboxStatusPending = "pending"
	testPaymentOutboxStatusSent    = "sent"
	testPaymentAppBaseURL          = "https://app.example.test" //nolint:gosec // test fixture URL, not a credential
)

// newTestPaymentWorker builds a payments.PaymentReceivedWorker around
// sender with this file's stand-in AppBaseURL/From/ReplyTo -- every
// outbox test needs one, only the injected Sender and (occasionally) Now
// vary.
func newTestPaymentWorker(sender mail.Sender) payments.PaymentReceivedWorker {
	return payments.PaymentReceivedWorker{Sender: sender, Now: time.Now, AppBaseURL: testPaymentAppBaseURL, From: testOutboxFrom, ReplyTo: testOutboxReplyTo}
}

// seedPaymentOutboxAmountCents is every seedPaymentOutboxRow fixture's
// amount -- no test in this file asserts on a varying amount, so it is
// not a parameter (only TestPaymentWorker_ProcessPending_MailsEveryOwnerAndAdminMarksSent
// checks the formatted "$50.00" this produces).
const seedPaymentOutboxAmountCents = 5000

// seedPaymentOutboxRow inserts a pending payment_received_outbox row for
// practiceID (with a fresh payments row backing it, per the table's
// payment_id FK) using the superuser Admin connection -- the table
// carries no RLS (00035). suffix disambiguates the fixture Stripe invoice
// id (UNIQUE per 00024) when a test seeds more than one row for the same
// Practice.
func seedPaymentOutboxRow(t *testing.T, db *testdb.DB, practiceID, suffix string, attemptCount int, nextAttemptAt time.Time) string {
	t.Helper()
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_outbox_fixture_"+practiceID+suffix, invoiceStatusOpen, seedPaymentOutboxAmountCents, time.Now())
	paymentID := testdb.Scalar[string](t, db.Admin, `INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at, kind) VALUES ($1, 'pi_fixture', $2, now(), 'stripe') RETURNING id`,
		invoiceID, seedPaymentOutboxAmountCents)
	id := testdb.Scalar[string](t, db.Admin, `INSERT INTO payment_received_outbox (payment_id, practice_id, attempt_count, next_attempt_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		paymentID, practiceID, attemptCount, nextAttemptAt)
	return id
}

func TestQueuePaymentReceivedNotification_InsertsPendingRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Queue Payment Insert")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_queue_insert", invoiceStatusOpen, 5000, time.Now())
	paymentID := testdb.Scalar[string](t, db.Admin, `INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at, kind) VALUES ($1, 'pi_queue_insert', 5000, now(), 'stripe') RETURNING id`,
		invoiceID)

	tx := db.BeginAs(t)

	if err := payments.QueuePaymentReceivedNotification(t.Context(), tx, paymentID, practiceID); err != nil {
		t.Fatalf("QueuePaymentReceivedNotification: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	var status string
	var gotPaymentID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(status), max(payment_id::text) FROM payment_received_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&count, &status, &gotPaymentID); err != nil {
		t.Fatalf("query payment_received_outbox: %v", err)
	}
	if count != 1 || status != testPaymentOutboxStatusPending || gotPaymentID != paymentID {
		t.Fatalf("count/status/payment_id = %d/%q/%q, want 1/%s/%s", count, status, gotPaymentID, testPaymentOutboxStatusPending, paymentID)
	}
}

func TestPaymentWorker_ProcessPending_MailsEveryOwnerAndAdminMarksSent(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "payment-owner-one", []string{ownerRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, "payment-admin-one", []string{adminRole}, "employee")
	// A Doula at the same Practice should never be mailed -- ADR-0006's
	// read table gives Contract money/Invoice history to Owner and Admin
	// only.
	testdb.SeedStaffAtPractice(t, db, practiceID, "payment-doula-bystander", []string{doulaRole}, "employee")
	outboxID := seedPaymentOutboxRow(t, db, practiceID, "", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestPaymentWorker(sender))

	status, _ := outboxRowState(t, db, paymentOutboxTable, outboxID)
	if status != testPaymentOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testPaymentOutboxStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("sent %d messages, want 2 (one per Owner/Admin)", len(sent))
	}
	wantLink := testPaymentAppBaseURL + "/practices/" + practiceID
	wantRecipients := map[string]bool{"payment-owner-one@example.com": false, "payment-admin-one@example.com": false}
	for _, msg := range sent {
		if msg.Subject != "DoulaCloud: a Payment arrived" {
			t.Fatalf("subject = %q", msg.Subject)
		}
		if _, ok := wantRecipients[msg.To]; !ok {
			t.Fatalf("To = %q, want the Owner or the Admin", msg.To)
		}
		wantRecipients[msg.To] = true
		if !strings.Contains(msg.Text, wantLink) {
			t.Fatalf("body %q does not contain link %q", msg.Text, wantLink)
		}
	}
	for to, got := range wantRecipients {
		if !got {
			t.Fatalf("recipient %q was never mailed", to)
		}
	}
}

// TestPaymentWorker_ProcessPending_TwoPaymentsForSamePracticeBothMailed
// pins 00035's dropped-dedup design: unlike payout_outbox and
// low_credit_outbox, this table carries no "one pending row per
// Practice" index, because two payments landing before a Scheduler tick
// must both be mailed rather than one silently swallowing the other.
func TestPaymentWorker_ProcessPending_TwoPaymentsForSamePracticeBothMailed(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "payment-owner-two-payments", []string{ownerRole}, "employee")
	firstID := seedPaymentOutboxRow(t, db, practiceID, "-first", 0, time.Now().Add(-time.Minute))
	secondID := seedPaymentOutboxRow(t, db, practiceID, "-second", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestPaymentWorker(sender))

	firstStatus, _ := outboxRowState(t, db, paymentOutboxTable, firstID)
	secondStatus, _ := outboxRowState(t, db, paymentOutboxTable, secondID)
	if firstStatus != testPaymentOutboxStatusSent || secondStatus != testPaymentOutboxStatusSent {
		t.Fatalf("statuses = %q/%q, want both %s", firstStatus, secondStatus, testPaymentOutboxStatusSent)
	}
	if len(sender.Sent()) != 2 {
		t.Fatalf("sent %d messages, want 2 -- one per Payment, both to the same Owner", len(sender.Sent()))
	}
}

// TestPaymentWorker_ProcessPending_Outcomes proves what the worker does to
// a row it finds: it leaves one not yet due; marks one sent with no mail
// when the Practice has no Owner or Admin; retries on a send failure; and
// dead-letters on the final attempt.
func TestPaymentWorker_ProcessPending_Outcomes(t *testing.T) {
	errMailgun := errors.New("mailgun unavailable")
	for _, tc := range []struct {
		name         string
		noOwner      bool
		attemptCount int
		due          time.Duration
		sendErr      error
		wantStatus   string
		wantAttempts int
	}{
		{name: "a row not yet due is skipped", due: time.Hour, wantStatus: testPaymentOutboxStatusPending},
		{name: "a Practice with no Owner or Admin is marked sent with no mail", noOwner: true, due: -time.Minute, wantStatus: testPaymentOutboxStatusSent},
		{name: "a send failure retries", due: -time.Minute, sendErr: errMailgun, wantStatus: testPaymentOutboxStatusPending, wantAttempts: 1},
		// One attempt short of the schedule's length -- this failure is the
		// last one before dead-letter.
		{name: "the final attempt dead-letters", attemptCount: 4, due: -time.Minute, sendErr: errMailgun, wantStatus: statusDeadLettered, wantAttempts: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			staff := fixture.Owner()
			if tc.noOwner {
				staff = fixture.NoRole()
			}
			e := newEnv(t, signedInAs(staff))
			outboxID := seedPaymentOutboxRow(t, e.DB, e.Practice.ID, "", tc.attemptCount, time.Now().Add(tc.due))

			sender := &mail.FakeSender{Err: tc.sendErr}
			runWorker(t, e.DB, newTestPaymentWorker(sender))

			status, attemptCount := outboxRowState(t, e.DB, paymentOutboxTable, outboxID)
			if status != tc.wantStatus || attemptCount != tc.wantAttempts {
				t.Fatalf("status/attempt_count = %q/%d, want %s/%d", status, attemptCount, tc.wantStatus, tc.wantAttempts)
			}
			if len(sender.Sent()) != 0 {
				t.Fatalf("sent %d messages, want none", len(sender.Sent()))
			}
			if tc.wantStatus == statusDeadLettered {
				if got := testdb.DeadLetterReason(t, e.DB, "payment_received_outbox", outboxID); got != "retries_exhausted" {
					t.Fatalf("dead_letter_reason = %q, want retries_exhausted", got)
				}
			}
		})
	}
}
