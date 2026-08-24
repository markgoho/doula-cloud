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
	testPaymentOutboxStatusPending = "pending"
	testPaymentOutboxStatusSent    = "sent"
	testPaymentAppBaseURL          = "https://app.example.test" //nolint:gosec // test fixture URL, not a credential
)

// newTestPaymentWorker builds a payments.PaymentReceivedWorker around
// sender with this file's stand-in AppBaseURL/From/ReplyTo -- every
// outbox test needs one, only the injected Sender and (occasionally) Now
// vary.
func newTestPaymentWorker(sender mail.Sender) payments.PaymentReceivedWorker {
	return payments.PaymentReceivedWorker{Sender: sender, Now: time.Now, AppBaseURL: testPaymentAppBaseURL, From: "a@b.test", ReplyTo: "support@b.test"}
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
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_outbox_fixture_"+practiceID+suffix, invoiceStatusOpen, seedPaymentOutboxAmountCents, time.Now())
	var paymentID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at) VALUES ($1, 'pi_fixture', $2, now()) RETURNING id`,
		invoiceID, seedPaymentOutboxAmountCents,
	).Scan(&paymentID); err != nil {
		t.Fatalf("seed payments row: %v", err)
	}
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO payment_received_outbox (payment_id, practice_id, attempt_count, next_attempt_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		paymentID, practiceID, attemptCount, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed payment_received_outbox row: %v", err)
	}
	return id
}

// runPaymentWorker begins a tx on db.App, sets the trusted session var
// payments.ProcessPaymentOutboxHandler would otherwise set after its own
// secret check, runs ProcessPending, and commits.
func runPaymentWorker(t *testing.T, db *testdb.DB, w payments.PaymentReceivedWorker) {
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

func paymentOutboxRowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count FROM payment_received_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount); err != nil {
		t.Fatalf("query payment_received_outbox row: %v", err)
	}
	return status, attemptCount
}

func TestQueuePaymentReceivedNotification_InsertsPendingRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Queue Payment Insert")
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_queue_insert", invoiceStatusOpen, 5000, time.Now())
	var paymentID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at) VALUES ($1, 'pi_queue_insert', 5000, now()) RETURNING id`,
		invoiceID,
	).Scan(&paymentID); err != nil {
		t.Fatalf("seed payments row: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

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
	practiceID := seedOwner(t, db, "payment-owner-one")
	adminStaffID := seedStaff(t, db, "payment-admin-one")
	seedMembership(t, db, practiceID, adminStaffID, "{admin}")
	// A Doula at the same Practice should never be mailed -- ADR-0006's
	// read table gives Contract money/Invoice history to Owner and Admin
	// only.
	doulaStaffID := seedStaff(t, db, "payment-doula-bystander")
	seedMembership(t, db, practiceID, doulaStaffID, "{doula}")
	outboxID := seedPaymentOutboxRow(t, db, practiceID, "", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runPaymentWorker(t, db, newTestPaymentWorker(sender))

	status, _ := paymentOutboxRowState(t, db, outboxID)
	if status != testPaymentOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testPaymentOutboxStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("sent %d messages, want 2 (one per Owner/Admin)", len(sent))
	}
	wantLink := testPaymentAppBaseURL + "/practices/" + practiceID
	for _, msg := range sent {
		if msg.Subject != "Doula Cloud: a Payment arrived" {
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

// TestPaymentWorker_ProcessPending_TwoPaymentsForSamePracticeBothMailed
// pins 00035's dropped-dedup design: unlike payout_outbox and
// low_credit_outbox, this table carries no "one pending row per
// Practice" index, because two payments landing before a Scheduler tick
// must both be mailed rather than one silently swallowing the other.
func TestPaymentWorker_ProcessPending_TwoPaymentsForSamePracticeBothMailed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "payment-owner-two-payments")
	firstID := seedPaymentOutboxRow(t, db, practiceID, "-first", 0, time.Now().Add(-time.Minute))
	secondID := seedPaymentOutboxRow(t, db, practiceID, "-second", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runPaymentWorker(t, db, newTestPaymentWorker(sender))

	firstStatus, _ := paymentOutboxRowState(t, db, firstID)
	secondStatus, _ := paymentOutboxRowState(t, db, secondID)
	if firstStatus != testPaymentOutboxStatusSent || secondStatus != testPaymentOutboxStatusSent {
		t.Fatalf("statuses = %q/%q, want both %s", firstStatus, secondStatus, testPaymentOutboxStatusSent)
	}
	if len(sender.Sent()) != 2 {
		t.Fatalf("sent %d messages, want 2 -- one per Payment, both to the same Owner", len(sender.Sent()))
	}
}

func TestPaymentWorker_ProcessPending_SkipsRowNotYetDue(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "payment-owner-not-due")
	outboxID := seedPaymentOutboxRow(t, db, practiceID, "", 0, time.Now().Add(time.Hour))

	sender := &mail.FakeSender{}
	runPaymentWorker(t, db, newTestPaymentWorker(sender))

	status, _ := paymentOutboxRowState(t, db, outboxID)
	if status != testPaymentOutboxStatusPending {
		t.Fatalf("status = %q, want %s (not due yet)", status, testPaymentOutboxStatusPending)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no send for a not-yet-due row")
	}
}

func TestPaymentWorker_ProcessPending_ZeroOwnersOrAdminsMarksSentWithNoMail(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Payment Ownerless Practice")
	outboxID := seedPaymentOutboxRow(t, db, practiceID, "", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runPaymentWorker(t, db, newTestPaymentWorker(sender))

	status, _ := paymentOutboxRowState(t, db, outboxID)
	if status != testPaymentOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testPaymentOutboxStatusSent)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent when the Practice has no Owner or Admin")
	}
}

func TestPaymentWorker_ProcessPending_RetriesOnSendFailure(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "payment-owner-retry")
	outboxID := seedPaymentOutboxRow(t, db, practiceID, "", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runPaymentWorker(t, db, newTestPaymentWorker(sender))

	status, attemptCount := paymentOutboxRowState(t, db, outboxID)
	if status != testPaymentOutboxStatusPending || attemptCount != 1 {
		t.Fatalf("status/attempt_count = %q/%d, want %s/1", status, attemptCount, testPaymentOutboxStatusPending)
	}
}

func TestPaymentWorker_ProcessPending_DeadLettersAfterFinalAttempt(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "payment-owner-dead-letter")
	// One attempt short of the schedule's length -- this failure is the
	// last one before dead-letter.
	outboxID := seedPaymentOutboxRow(t, db, practiceID, "", 4, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runPaymentWorker(t, db, newTestPaymentWorker(sender))

	status, attemptCount := paymentOutboxRowState(t, db, outboxID)
	if status != "dead_lettered" || attemptCount != 5 {
		t.Fatalf("status/attempt_count = %q/%d, want dead_lettered/5", status, attemptCount)
	}
}
