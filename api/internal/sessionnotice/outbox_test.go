package sessionnotice_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const testStatusSent = "sent"

// newTestWorker builds a sessionnotice.Worker around sender with this
// file's stand-in From/ReplyTo. The Worker tests below are kept (rather
// than moved to compose_test.go) because compose needs a live tx for its
// own staffEmail lookup; skip/retry/dead-letter/suppression/
// terminal-clear belong to outbox.MailWorker's own suite.
func newTestWorker(sender mail.Sender) sessionnotice.Worker {
	return sessionnotice.NewWorker(outbox.Mailer{Sender: sender, Now: time.Now, From: "a@b.test", ReplyTo: "support@b.test"})
}

// seedSessionNoticeOutboxRow inserts an outbox row of kind for identityUID directly
// (bypassing Queue*), for tests that need to control status,
// attempt_count, next_attempt_at, or created_at precisely.
func seedSessionNoticeOutboxRow(t *testing.T, db *testdb.DB, identityUID, kind string, nextAttemptAt, createdAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO session_notice_outbox (identity_uid, kind, next_attempt_at, created_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		identityUID, kind, nextAttemptAt, createdAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed session_notice_outbox row: %v", err)
	}
	return id
}

// runWorker begins a tx on db.App, sets the trusted session var
// outbox.ProcessHandler would otherwise set after its own
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

func outboxRowState(t *testing.T, db *testdb.DB, id string) (status string) {
	t.Helper()
	var attemptCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count FROM session_notice_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount); err != nil {
		t.Fatalf("query session_notice_outbox row: %v", err)
	}
	return status
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
	testdb.SeedStaff(t, db, uid)

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
	testdb.SeedStaff(t, db, uid)
	now := time.Now()
	seedSessionNoticeOutboxRow(t, db, uid, "new_signin", now, now.Add(-time.Hour))

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
	testdb.SeedStaff(t, db, uid)
	now := time.Now()
	seedSessionNoticeOutboxRow(t, db, uid, "new_signin", now, now.Add(-8*24*time.Hour))

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
	testdb.SeedStaff(t, db, uid)
	outboxID := seedSessionNoticeOutboxRow(t, db, uid, "new_signin", time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status := outboxRowState(t, db, outboxID)
	if status != testStatusSent {
		t.Fatalf("status = %q, want %s", status, testStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	if sent[0].To != uid+"@example.com" {
		t.Fatalf("To = %q", sent[0].To)
	}
	if sent[0].Subject != "Doula Cloud: new sign-in to your account" {
		t.Fatalf("subject = %q", sent[0].Subject)
	}
}

func TestWorker_ProcessPending_MailsSessionRevoked(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-worker-revoked"
	testdb.SeedStaff(t, db, uid)
	outboxID := seedSessionNoticeOutboxRow(t, db, uid, "session_revoked", time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status := outboxRowState(t, db, outboxID)
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

// TestWorker_ProcessPending_NoStaffMarksSentWithNoMail covers an
// identity that queued a notice and was then removed from staff before
// send -- there is nobody left to notify.
func TestWorker_ProcessPending_NoStaffMarksSentWithNoMail(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-since-removed"
	outboxID := seedSessionNoticeOutboxRow(t, db, uid, "new_signin", time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status := outboxRowState(t, db, outboxID)
	if status != testStatusSent {
		t.Fatalf("status = %q, want %s", status, testStatusSent)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("expected no mail sent for an identity with no staff row")
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
	testdb.SeedStaff(t, db, uid)
	outboxID := seedSessionNoticeOutboxRow(t, db, uid, "mfa_recovery_cleared", time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status := outboxRowState(t, db, outboxID)
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
