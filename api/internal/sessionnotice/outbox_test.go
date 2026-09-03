package sessionnotice_test

import (
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	testStatusPending = "pending"
	testStatusSent    = "sent"
)

// newTestWorker builds a sessionnotice.Worker around sender with this
// file's stand-in From/ReplyTo -- every outbox test needs one, only the
// injected Sender and (occasionally) Now vary.
func newTestWorker(sender mail.Sender) sessionnotice.Worker {
	return sessionnotice.Worker{Sender: sender, Now: time.Now, From: "a@b.test", ReplyTo: "support@b.test"}
}

// seedStaff inserts a Staff row for identityUID with a fixed email, using
// the superuser Admin connection. sessionnotice needs no Practice or
// membership -- the recipient is the Staff member alone.
func seedStaff(t *testing.T, db *testdb.DB, identityUID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, 'Test Staff', 'staff@example.com', 'NY')`,
		identityUID,
	); err != nil {
		t.Fatalf("seed staff %q: %v", identityUID, err)
	}
}

// seedOutboxRow inserts an outbox row of kind for identityUID directly
// (bypassing Queue*), for tests that need to control status,
// attempt_count, next_attempt_at, or created_at precisely.
func seedOutboxRow(t *testing.T, db *testdb.DB, identityUID, kind string, attemptCount int, nextAttemptAt, createdAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO session_notice_outbox (identity_uid, kind, attempt_count, next_attempt_at, created_at)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		identityUID, kind, attemptCount, nextAttemptAt, createdAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed session_notice_outbox row: %v", err)
	}
	return id
}

// runWorker begins a tx on db.App, sets the trusted session var
// sessionnotice.ProcessOutboxHandler would otherwise set after its own
// secret check, runs ProcessPending, and commits.
func runWorker(t *testing.T, db *testdb.DB, w sessionnotice.Worker) {
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

func outboxRowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count FROM session_notice_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount); err != nil {
		t.Fatalf("query session_notice_outbox row: %v", err)
	}
	return status, attemptCount
}

func countOutboxRows(t *testing.T, db *testdb.DB, identityUID, kind string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM session_notice_outbox WHERE identity_uid = $1 AND kind = $2`,
		identityUID, kind,
	).Scan(&count); err != nil {
		t.Fatalf("count session_notice_outbox rows: %v", err)
	}
	return count
}

func TestQueueNewSignInIfDue_SkipsNonStaffIdentity(t *testing.T) {
	db := testdb.New(t)
	const clientUID = "client-portal-uid"

	if err := sessionnotice.QueueNewSignInIfDue(t.Context(), db.App, clientUID, time.Now(), &tasknudge.FakeEnqueuer{}); err != nil {
		t.Fatalf("QueueNewSignInIfDue: %v", err)
	}
	if got := countOutboxRows(t, db, clientUID, "new_signin"); got != 0 {
		t.Fatalf("outbox rows for a non-Staff identity = %d, want 0", got)
	}
}

func TestQueueNewSignInIfDue_QueuesFirstSignIn(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-first-signin"
	seedStaff(t, db, uid)

	if err := sessionnotice.QueueNewSignInIfDue(t.Context(), db.App, uid, time.Now(), &tasknudge.FakeEnqueuer{}); err != nil {
		t.Fatalf("QueueNewSignInIfDue: %v", err)
	}
	if got := countOutboxRows(t, db, uid, "new_signin"); got != 1 {
		t.Fatalf("outbox rows after first sign-in = %d, want 1", got)
	}
}

// TestQueueNewSignInIfDue_SkipsWithinIdleWindow covers the ticket's own
// example -- a phone and a laptop signing in the same day -- as a notice
// queued within signinIdleWindow of a prior one for the same identity.
func TestQueueNewSignInIfDue_SkipsWithinIdleWindow(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-second-device-same-day"
	seedStaff(t, db, uid)
	now := time.Now()
	seedOutboxRow(t, db, uid, "new_signin", 0, now, now.Add(-time.Hour))

	if err := sessionnotice.QueueNewSignInIfDue(t.Context(), db.App, uid, now, &tasknudge.FakeEnqueuer{}); err != nil {
		t.Fatalf("QueueNewSignInIfDue: %v", err)
	}
	if got := countOutboxRows(t, db, uid, "new_signin"); got != 1 {
		t.Fatalf("outbox rows for a same-day second device = %d, want 1 (deduped)", got)
	}
}

// TestQueueNewSignInIfDue_QueuesAgainAfterIdleWindow covers a genuine
// return after a gap -- the case the idle window is meant to still flag.
func TestQueueNewSignInIfDue_QueuesAgainAfterIdleWindow(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-returns-after-gap"
	seedStaff(t, db, uid)
	now := time.Now()
	seedOutboxRow(t, db, uid, "new_signin", 0, now, now.Add(-8*24*time.Hour))

	if err := sessionnotice.QueueNewSignInIfDue(t.Context(), db.App, uid, now, &tasknudge.FakeEnqueuer{}); err != nil {
		t.Fatalf("QueueNewSignInIfDue: %v", err)
	}
	if got := countOutboxRows(t, db, uid, "new_signin"); got != 2 {
		t.Fatalf("outbox rows after an 8-day gap = %d, want 2 (a fresh notice)", got)
	}
}

// TestQueueNewSignInIfDue_DBFailureRollsBack covers the tx's rollback
// path: with `staff` gone, the staff-membership check fails, the error
// propagates, and the deferred rollback runs instead of a commit.
// Mirrors session.TestCreateHandler_SessionStoreFailure's own
// drop-the-table approach to forcing a write/read failure.
func TestQueueNewSignInIfDue_DBFailureRollsBack(t *testing.T) {
	db := testdb.New(t)
	if _, err := db.Admin.ExecContext(t.Context(), `DROP TABLE staff CASCADE`); err != nil {
		t.Fatalf("drop staff: %v", err)
	}

	err := sessionnotice.QueueNewSignInIfDue(t.Context(), db.App, "some-uid", time.Now(), &tasknudge.FakeEnqueuer{})
	if err == nil {
		t.Fatal("expected an error with the staff table gone")
	}
}

func TestQueueSessionRevoked_InsertsPendingRow(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-revoked"

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if err := sessionnotice.QueueSessionRevoked(t.Context(), tx, uid); err != nil {
		t.Fatalf("QueueSessionRevoked: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if got := countOutboxRows(t, db, uid, "session_revoked"); got != 1 {
		t.Fatalf("outbox rows = %d, want 1", got)
	}
}

// TestQueueSessionRevoked_ConflictOnExistingPendingRowIsNoop covers two
// rapid Owner clicks of "end sessions" queuing only one notice.
func TestQueueSessionRevoked_ConflictOnExistingPendingRowIsNoop(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-double-click"

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if err := sessionnotice.QueueSessionRevoked(t.Context(), tx, uid); err != nil {
		t.Fatalf("QueueSessionRevoked (first): %v", err)
	}
	if err := sessionnotice.QueueSessionRevoked(t.Context(), tx, uid); err != nil {
		t.Fatalf("QueueSessionRevoked (second): %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if got := countOutboxRows(t, db, uid, "session_revoked"); got != 1 {
		t.Fatalf("outbox rows after two rapid calls = %d, want 1", got)
	}
}

func TestWorker_ProcessPending_MailsNewSignInAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-worker-signin"
	seedStaff(t, db, uid)
	outboxID := seedOutboxRow(t, db, uid, "new_signin", 0, time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _ := outboxRowState(t, db, outboxID)
	if status != testStatusSent {
		t.Fatalf("status = %q, want %s", status, testStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	if sent[0].To != "staff@example.com" {
		t.Fatalf("To = %q", sent[0].To)
	}
	if sent[0].Subject != "Doula Cloud: new sign-in to your account" {
		t.Fatalf("subject = %q", sent[0].Subject)
	}
}

func TestWorker_ProcessPending_MailsSessionRevoked(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-worker-revoked"
	seedStaff(t, db, uid)
	outboxID := seedOutboxRow(t, db, uid, "session_revoked", 0, time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _ := outboxRowState(t, db, outboxID)
	if status != testStatusSent {
		t.Fatalf("status = %q, want %s", status, testStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	if sent[0].Subject != "Doula Cloud: your sessions were signed out" {
		t.Fatalf("subject = %q", sent[0].Subject)
	}
}

func TestWorker_ProcessPending_SkipsRowNotYetDue(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-not-due"
	seedStaff(t, db, uid)
	outboxID := seedOutboxRow(t, db, uid, "new_signin", 0, time.Now().Add(time.Hour), time.Now())

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _ := outboxRowState(t, db, outboxID)
	if status != testStatusPending {
		t.Fatalf("status = %q, want %s (not due yet)", status, testStatusPending)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no send for a not-yet-due row")
	}
}

// TestWorker_ProcessPending_NoStaffMarksSentWithNoMail covers an
// identity that queued a notice and was then removed from staff before
// send -- there is nobody left to notify.
func TestWorker_ProcessPending_NoStaffMarksSentWithNoMail(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-since-removed"
	outboxID := seedOutboxRow(t, db, uid, "new_signin", 0, time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _ := outboxRowState(t, db, outboxID)
	if status != testStatusSent {
		t.Fatalf("status = %q, want %s", status, testStatusSent)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent for an identity with no staff row")
	}
}

func TestWorker_ProcessPending_RetriesOnSendFailure(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-retry"
	seedStaff(t, db, uid)
	outboxID := seedOutboxRow(t, db, uid, "new_signin", 0, time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runWorker(t, db, newTestWorker(sender))

	status, attemptCount := outboxRowState(t, db, outboxID)
	if status != testStatusPending || attemptCount != 1 {
		t.Fatalf("status/attempt_count = %q/%d, want %s/1", status, attemptCount, testStatusPending)
	}
}

func TestWorker_ProcessPending_DeadLettersAfterFinalAttempt(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-dead-letter"
	seedStaff(t, db, uid)
	// One attempt short of the schedule's length -- this failure is the
	// last one before dead-letter.
	outboxID := seedOutboxRow(t, db, uid, "new_signin", 4, time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{Err: errors.New("mailgun unavailable")}
	runWorker(t, db, newTestWorker(sender))

	status, attemptCount := outboxRowState(t, db, outboxID)
	if status != "dead_lettered" || attemptCount != 5 {
		t.Fatalf("status/attempt_count = %q/%d, want dead_lettered/5", status, attemptCount)
	}
}

func TestQueueMFARecoveryCleared_InsertsPendingRow(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-mfa-cleared"

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if err := sessionnotice.QueueMFARecoveryCleared(t.Context(), tx, uid); err != nil {
		t.Fatalf("QueueMFARecoveryCleared: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if got := countOutboxRows(t, db, uid, "mfa_recovery_cleared"); got != 1 {
		t.Fatalf("outbox rows = %d, want 1", got)
	}
}

// TestQueueMFARecoveryCleared_ConflictOnExistingPendingRowIsNoop mirrors
// TestQueueSessionRevoked_ConflictOnExistingPendingRowIsNoop: a code
// spent twice in quick succession before the worker runs (should never
// happen, single-use codes) must still queue only one notice --
// session_notice_outbox_mfa_recovery_cleared_one_pending (00063) is the
// arbiter this exercises.
func TestQueueMFARecoveryCleared_ConflictOnExistingPendingRowIsNoop(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-mfa-cleared-twice"

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if err := sessionnotice.QueueMFARecoveryCleared(t.Context(), tx, uid); err != nil {
		t.Fatalf("QueueMFARecoveryCleared (first): %v", err)
	}
	if err := sessionnotice.QueueMFARecoveryCleared(t.Context(), tx, uid); err != nil {
		t.Fatalf("QueueMFARecoveryCleared (second): %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if got := countOutboxRows(t, db, uid, "mfa_recovery_cleared"); got != 1 {
		t.Fatalf("outbox rows after two rapid calls = %d, want 1", got)
	}
}

func TestWorker_ProcessPending_MailsMFARecoveryCleared(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-worker-mfa-cleared"
	seedStaff(t, db, uid)
	outboxID := seedOutboxRow(t, db, uid, "mfa_recovery_cleared", 0, time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _ := outboxRowState(t, db, outboxID)
	if status != testStatusSent {
		t.Fatalf("status = %q, want %s", status, testStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	if sent[0].Subject != "Doula Cloud: your two-factor authentication was reset" {
		t.Fatalf("subject = %q", sent[0].Subject)
	}
}
