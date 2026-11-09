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
		{"just before the end the notice is mailed", time.Microsecond, true},
		{"at the end nothing is mailed", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newNoticeEnv(t)
			endsAt := testdb.Scalar[time.Time](t, e.DB.Admin, `SELECT ends_at FROM engagement_coverage_gaps WHERE id = $1`, e.GapID)
			sender := &mail.FakeSender{}
			now := endsAt.Add(-c.beforeEnd)

			e.runWorkerWith(oncall.GapNoticeWorker{
				Sender: sender, Now: func() time.Time { return now },
				AppBaseURL: testAppBaseURL, From: "from@example.test", ReplyTo: "reply@example.test",
			})

			if mailed := len(sender.Sent()) > 0; mailed != c.wantMailed {
				t.Fatalf("mailed = %v (%d messages), want %v", mailed, len(sender.Sent()), c.wantMailed)
			}
			if got := e.noticeStatus(); got != statusSent {
				t.Fatalf("status = %q, want sent", got)
			}
		})
	}
}

// runWorkerWith runs one pass of a worker the caller built, so a test can
// fix its clock.
func (e noticeEnv) runWorkerWith(w oncall.GapNoticeWorker) {
	e.T.Helper()
	tx, err := e.DB.App.BeginTx(e.T.Context(), nil)
	if err != nil {
		e.T.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(e.T.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		e.T.Fatalf("set trusted door: %v", err)
	}
	if err := w.ProcessPending(e.T.Context(), tx); err != nil {
		e.T.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		e.T.Fatalf("commit: %v", err)
	}
}
