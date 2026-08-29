package sitebuild_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/testdb"
)

// fixedNow is the clock every worker test runs on, so "aged past the
// window" is arithmetic rather than a sleep.
func fixedNow() time.Time { return time.Now() }

func buildHandler(db *testdb.DB, d sitebuild.Dispatcher) http.Handler {
	return sitebuild.ProcessOutboxHandler(db.App, sitebuild.Worker{Dispatcher: d, Now: fixedNow}, workerSecret)
}

func TestProcessOutbox_RefusesWithoutTheSecret(t *testing.T) {
	db := testdb.New(t)
	dispatcher := &fakeDispatcher{}

	for _, sent := range []string{"", "wrong-secret"} {
		rec := post(t, buildHandler(db, dispatcher), sent)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("secret %q: got %d, want 401", sent, rec.Code)
		}
	}
	if dispatcher.count() != 0 {
		t.Fatalf("dispatched %d times on an unauthorized call", dispatcher.count())
	}
}

// An empty configured secret must refuse everything rather than accept
// anything: a misconfigured deploy should be closed, not open.
func TestProcessOutbox_EmptyConfiguredSecretRefusesEveryone(t *testing.T) {
	db := testdb.New(t)
	h := sitebuild.ProcessOutboxHandler(db.App, sitebuild.Worker{Dispatcher: &fakeDispatcher{}, Now: fixedNow}, "")

	if rec := post(t, h, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

func TestProcessOutbox_NothingQueuedDispatchesNothing(t *testing.T) {
	db := testdb.New(t)
	dispatcher := &fakeDispatcher{}

	if rec := post(t, buildHandler(db, dispatcher), workerSecret); rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if dispatcher.count() != 0 {
		t.Fatalf("dispatched %d times with nothing queued", dispatcher.count())
	}
}

// The window's whole purpose: a rebuild queued a moment ago is left
// alone, so the rows that follow it in the next minute can join it.
func TestProcessOutbox_WaitsForTheCoalescingWindow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")
	queueRebuild(t, db, practiceID, 5)
	dispatcher := &fakeDispatcher{}

	post(t, buildHandler(db, dispatcher), workerSecret)

	if dispatcher.count() != 0 {
		t.Fatalf("dispatched %d times inside the window", dispatcher.count())
	}
	pending, _, _ := outboxCounts(t, db)
	if pending != 1 {
		t.Fatalf("pending rows = %d, want the row left queued", pending)
	}
}

// Two publishes in quick succession: one deploy, and neither row lost.
func TestProcessOutbox_CollapsesEveryPendingRowIntoOneDispatch(t *testing.T) {
	db := testdb.New(t)
	first := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")
	second := seedHostedPage(t, db, "Finger Lakes Birth", "finger-lakes-birth")
	// The oldest row is past the window; the second arrived while it was
	// still waiting, which is exactly the case the window exists for.
	queueRebuild(t, db, first, 120)
	queueRebuild(t, db, second, 30)
	dispatcher := &fakeDispatcher{}

	if rec := post(t, buildHandler(db, dispatcher), workerSecret); rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	if dispatcher.count() != 1 {
		t.Fatalf("dispatched %d times, want exactly one deploy for both", dispatcher.count())
	}
	pending, dispatched, _ := outboxCounts(t, db)
	if pending != 0 || dispatched != 2 {
		t.Fatalf("pending=%d dispatched=%d, want both rows claimed", pending, dispatched)
	}
}

// The nudge that arrives for the second publish finds the work already
// done, and must not deploy again for no reason.
func TestProcessOutbox_SecondCallDispatchesNothingMore(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")
	queueRebuild(t, db, practiceID, 120)
	dispatcher := &fakeDispatcher{}
	h := buildHandler(db, dispatcher)

	post(t, h, workerSecret)
	post(t, h, workerSecret)

	if dispatcher.count() != 1 {
		t.Fatalf("dispatched %d times, want one", dispatcher.count())
	}
}

// A dispatch that fails must leave the work queued: Cloud Scheduler's
// cadence is what retries it, and a row marked done would lose the
// deploy silently.
func TestProcessOutbox_FailedDispatchLeavesTheRowQueued(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")
	queueRebuild(t, db, practiceID, 120)
	dispatcher := &fakeDispatcher{err: errors.New("github returned 401")}

	if rec := post(t, buildHandler(db, dispatcher), workerSecret); rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 -- the failure is recorded, not raised", rec.Code)
	}

	pending, dispatched, _ := outboxCounts(t, db)
	if pending != 1 || dispatched != 0 {
		t.Fatalf("pending=%d dispatched=%d, want the row still queued", pending, dispatched)
	}
	var attempts int
	var lastError string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT attempt_count, last_error FROM site_build_outbox`,
	).Scan(&attempts, &lastError); err != nil {
		t.Fatalf("read attempt: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempt_count = %d, want 1", attempts)
	}
	if lastError == "" {
		t.Fatal("last_error is empty; the reason it failed was not recorded")
	}
}

// A credential that has lapsed fails identically forever. Dead-lettering
// stops the retry; the page stays pending, which is what tells her.
func TestProcessOutbox_DeadLettersAfterMaxAttempts(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")
	queueRebuild(t, db, practiceID, 120)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE site_build_outbox SET attempt_count = $1`, sitebuild.MaxAttempts-1,
	); err != nil {
		t.Fatalf("age the attempts: %v", err)
	}
	dispatcher := &fakeDispatcher{err: errors.New("github returned 401")}

	post(t, buildHandler(db, dispatcher), workerSecret)

	pending, _, dead := outboxCounts(t, db)
	if pending != 0 || dead != 1 {
		t.Fatalf("pending=%d dead=%d, want the row dead-lettered", pending, dead)
	}
}

// Queue is called inside the write site's own transaction (#443's
// write site is website.PutHandler), so it is tested the same way: a
// transaction, a row, and a commit.
func TestQueue_RecordsARebuild(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := sitebuild.Queue(t.Context(), tx, practiceID); err != nil {
		t.Fatalf("Queue: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	pending, _, _ := outboxCounts(t, db)
	if pending != 1 {
		t.Fatalf("pending rows = %d, want the rebuild queued", pending)
	}
}

// A write that rolls back must queue no deploy: the site is not stale
// if the change never happened.
func TestQueue_RollsBackWithTheWrite(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := sitebuild.Queue(t.Context(), tx, practiceID); err != nil {
		t.Fatalf("Queue: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	pending, _, _ := outboxCounts(t, db)
	if pending != 0 {
		t.Fatalf("pending rows = %d, want none after a rollback", pending)
	}
}
