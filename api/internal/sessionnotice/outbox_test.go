package sessionnotice_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	testStatusSent = "sent"
	kindNewSignIn  = "new_signin"
)

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
	return testdb.Scalar[string](t, db.Admin,
		`INSERT INTO session_notice_outbox (identity_uid, kind, next_attempt_at, created_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		identityUID, kind, nextAttemptAt, createdAt)
}

// runWorker begins a tx on db.App with the trusted session var
// outbox.ProcessHandler would otherwise set after its own secret check,
// runs ProcessPending, and commits.
func runWorker(t *testing.T, db *testdb.DB, w sessionnotice.Worker) {
	t.Helper()
	tx := db.BeginAs(t, testdb.SessionVar{Name: outbox.NotificationDoor, Value: "true"})
	if err := w.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// runPending seeds a due row of kind for uid, runs the worker once and
// returns the row's status and the mail it sent.
func runPending(t *testing.T, db *testdb.DB, uid, kind string) (status string, sent []mail.Message) {
	t.Helper()
	outboxID := seedSessionNoticeOutboxRow(t, db, uid, kind, time.Now().Add(-time.Minute), time.Now())
	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))
	return testdb.Scalar[string](t, db.Admin, `SELECT status FROM session_notice_outbox WHERE id = $1`, outboxID), sender.Sent()
}

func countOutboxRows(t *testing.T, db *testdb.DB, identityUID, kind string) int {
	t.Helper()
	return testdb.Scalar[int](t, db.Admin,
		`SELECT count(*) FROM session_notice_outbox WHERE identity_uid = $1 AND kind = $2`, identityUID, kind)
}

func TestQueueNewSignInIfDue_SkipsNonStaffIdentity(t *testing.T) {
	db := testdb.New(t)
	const clientUID = "client-portal-uid"

	if err := sessionnotice.QueueNewSignInIfDue(t.Context(), db.App, clientUID, time.Now(), &tasknudge.FakeEnqueuer{}); err != nil {
		t.Fatalf("QueueNewSignInIfDue: %v", err)
	}
	if got := countOutboxRows(t, db, clientUID, kindNewSignIn); got != 0 {
		t.Fatalf("outbox rows for a non-Staff identity = %d, want 0", got)
	}
}

// TestQueueNewSignInIfDue_QueuesOnlyOutsideTheIdleWindow covers the
// ticket's own example -- a phone and a laptop signing in the same day --
// as a notice queued within signinIdleWindow of a prior one for the same
// identity, and a genuine return after a gap, the case the idle window is
// meant to still flag.
func TestQueueNewSignInIfDue_QueuesOnlyOutsideTheIdleWindow(t *testing.T) {
	cases := []struct {
		name string
		// priorAge is how long ago the identity's earlier notice was
		// queued; zero means she has none.
		priorAge time.Duration
		want     int
	}{
		{"first sign-in", 0, 1},
		{"second device the same day", time.Hour, 1},
		{"return after an 8-day gap", 8 * 24 * time.Hour, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			const uid = "staff-signs-in"
			testdb.SeedStaff(t, db, uid)
			now := time.Now()
			if tc.priorAge > 0 {
				seedSessionNoticeOutboxRow(t, db, uid, kindNewSignIn, now, now.Add(-tc.priorAge))
			}

			if err := sessionnotice.QueueNewSignInIfDue(t.Context(), db.App, uid, now, &tasknudge.FakeEnqueuer{}); err != nil {
				t.Fatalf("QueueNewSignInIfDue: %v", err)
			}
			if got := countOutboxRows(t, db, uid, kindNewSignIn); got != tc.want {
				t.Fatalf("outbox rows = %d, want %d", got, tc.want)
			}
		})
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

// queuers are the three notices a transaction queues for an identity,
// each by its own function.
var queuers = []struct {
	kind  string
	queue func(context.Context, *sql.Tx, string) error
}{
	{"session_revoked", sessionnotice.QueueSessionRevoked},
	{"mfa_recovery_cleared", sessionnotice.QueueMFARecoveryCleared},
	{"session_evicted", func(ctx context.Context, tx *sql.Tx, uid string) error {
		_, err := sessionnotice.QueueSessionEvicted(ctx, tx, authn.Eviction{IdentityUID: uid, Tier: authn.TierStaff})
		return err
	}},
}

// TestQueue_InsertsOnePendingRowAndALaterCallIsANoop covers a notice
// queued once, and the conflict on an existing pending row being a no-op:
// two rapid Owner clicks of "end sessions", or a recovery code spent twice
// in quick succession before the worker runs (should never happen,
// single-use codes), queue only one notice -- the table's one-pending
// index is the arbiter this exercises.
func TestQueue_InsertsOnePendingRowAndALaterCallIsANoop(t *testing.T) {
	for _, q := range queuers {
		t.Run(q.kind, func(t *testing.T) {
			db := testdb.New(t)
			const uid = "staff-queued-twice"

			tx := db.BeginAs(t)
			if err := q.queue(t.Context(), tx, uid); err != nil {
				t.Fatalf("queue (first): %v", err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatalf("commit: %v", err)
			}
			if got := countOutboxRows(t, db, uid, q.kind); got != 1 {
				t.Fatalf("outbox rows = %d, want 1", got)
			}

			tx = db.BeginAs(t)
			if err := q.queue(t.Context(), tx, uid); err != nil {
				t.Fatalf("queue (second): %v", err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatalf("commit: %v", err)
			}
			if got := countOutboxRows(t, db, uid, q.kind); got != 1 {
				t.Fatalf("outbox rows after a second call = %d, want 1", got)
			}
		})
	}
}

// TestWorker_ProcessPending_MailsEachNoticeAndMarksItSent holds each
// notice kind to its own subject -- an eviction in particular has its own
// subject and body, not session_revoked's, since it ends one browser's
// session and her other devices stay signed in.
func TestWorker_ProcessPending_MailsEachNoticeAndMarksItSent(t *testing.T) {
	cases := []struct{ kind, subject string }{
		{kindNewSignIn, "DoulaCloud: new sign-in to your account"},
		{"session_revoked", "DoulaCloud: your sessions were signed out"},
		{"mfa_recovery_cleared", "DoulaCloud: your two-factor authentication was reset"},
		{"session_evicted", "DoulaCloud: you were signed out in one browser"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			db := testdb.New(t)
			const uid = "staff-worker-notice"
			testdb.SeedStaff(t, db, uid)

			status, sent := runPending(t, db, uid, tc.kind)

			if status != testStatusSent {
				t.Fatalf("status = %q, want %s", status, testStatusSent)
			}
			if len(sent) != 1 {
				t.Fatalf("sent %d messages, want 1", len(sent))
			}
			if sent[0].To != uid+"@example.com" {
				t.Fatalf("To = %q", sent[0].To)
			}
			if sent[0].Subject != tc.subject {
				t.Fatalf("subject = %q, want %q", sent[0].Subject, tc.subject)
			}
		})
	}
}

// TestWorker_ProcessPending_MarksSentWithNoMailWhenThereIsNobodyToNotify
// covers both skips: an identity that queued a notice and was then
// removed from staff before send, and #892's skip-at-send recheck -- a
// notice queued while she still had a login, claimed after she deleted
// it. Her Identity Platform account is gone and her staff row names
// nobody, so the row is marked sent with no mail, never retried and
// never dead-lettered.
func TestWorker_ProcessPending_MarksSentWithNoMailWhenThereIsNobodyToNotify(t *testing.T) {
	cases := []struct {
		name, kind string
		seed       func(*testing.T, *testdb.DB, string)
	}{
		{"no staff row", kindNewSignIn, func(*testing.T, *testdb.DB, string) {}},
		{"deleted login", "session_revoked", func(t *testing.T, db *testdb.DB, uid string) {
			testdb.RedactDeletedLogin(t, db, testdb.SeedStaff(t, db, uid))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			const uid = "staff-with-nobody-to-notify"
			tc.seed(t, db, uid)

			status, sent := runPending(t, db, uid, tc.kind)

			if status != testStatusSent {
				t.Fatalf("status = %q, want %s", status, testStatusSent)
			}
			if len(sent) != 0 {
				t.Fatalf("sent %d messages, want none", len(sent))
			}
		})
	}
}
