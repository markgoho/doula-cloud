package oncall_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/testdb"
)

// TestGapNoticeWorker_GapStillOpenUntilEndsAtOnTheWorkerClock is #1992's
// edge for the send-time recheck: with the worker's clock one microsecond
// before the gap's ends_at the gap still needs cover and the notice is
// mailed, and at ends_at it has ended and nothing is mailed. The worker's
// Now is fixed, so only a recheck read from it can pass.
func TestGapNoticeWorker_GapStillOpenUntilEndsAtOnTheWorkerClock(t *testing.T) {
	cases := []struct {
		name       string
		beforeEnd  time.Duration
		wantMailed bool
	}{
		{"just-before-end", time.Microsecond, true},
		{"at-end", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := testdb.New(t)
			f := newNoticeFixture(t, db, "notice-edge-"+c.name)
			var endsAt time.Time
			if err := db.Admin.QueryRowContext(t.Context(),
				`SELECT ends_at FROM engagement_coverage_gaps WHERE id = $1`, f.gapID).Scan(&endsAt); err != nil {
				t.Fatalf("read ends_at: %v", err)
			}

			sender := &mail.FakeSender{}
			now := endsAt.Add(-c.beforeEnd)
			runWorkerWith(t, db, oncall.GapNoticeWorker{
				Sender: sender, Now: func() time.Time { return now },
				AppBaseURL: testAppBaseURL, From: "from@example.test", ReplyTo: "reply@example.test",
			})
			if mailed := len(sender.Sent()) > 0; mailed != c.wantMailed {
				t.Fatalf("mailed = %v (%d messages), want %v", mailed, len(sender.Sent()), c.wantMailed)
			}
			if got := noticeStatus(t, db, f.gapID); got != statusSent {
				t.Fatalf("status = %q, want sent", got)
			}
		})
	}
}

// runWorkerWith is runWorker for a worker the caller built, so a test can
// fix its clock.
func runWorkerWith(t *testing.T, db *testdb.DB, w oncall.GapNoticeWorker) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("set trusted door: %v", err)
	}
	if err := w.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
