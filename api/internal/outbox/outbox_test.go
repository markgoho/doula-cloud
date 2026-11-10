package outbox_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

func TestWorker_MarkingARow(t *testing.T) {
	exhausted := len(outbox.BackoffSchedule) - 1
	suppressedErr := fmt.Errorf("%w: someone@example.test", mail.ErrSuppressed)
	cases := []struct {
		name     string
		attempts int
		clear    []string
		mark     func(w outbox.Worker, tx *sql.Tx) error
		want     rowWant
	}{
		{
			name: "sent leaves the secrets when none are configured",
			mark: func(w outbox.Worker, tx *sql.Tx) error {
				return w.MarkSent(context.Background(), tx, rowID, time.Now())
			},
			want: rowWant{status: testStatusSent},
		},
		{
			name:  "sent clears the configured columns",
			clear: []string{secretA, secretB},
			mark: func(w outbox.Worker, tx *sql.Tx) error {
				return w.MarkSent(context.Background(), tx, rowID, time.Now())
			},
			want: rowWant{status: testStatusSent, cleared: []string{secretA, secretB}},
		},
		{
			// A skip is a way out of pending like any other, so the
			// plaintext the row held goes with it -- the duty the login
			// deletion's own hand-written UPDATE once skipped.
			name:  "skipped records why and clears the configured columns",
			clear: []string{secretA},
			mark: func(w outbox.Worker, tx *sql.Tx) error {
				return w.MarkSkipped(context.Background(), tx, rowID, outbox.SkipLoginDeleted, time.Now())
			},
			want: rowWant{status: testStatusSkipped, skip: outbox.SkipLoginDeleted, cleared: []string{secretA}},
		},
		{
			name: "a failure before the schedule is spent schedules a retry",
			mark: func(w outbox.Worker, tx *sql.Tx) error {
				return w.MarkFailed(context.Background(), tx, rowID, 0, errors.New(mailgunErr), time.Now())
			},
			want: rowWant{status: testStatusPending, attempts: 1, lastError: mailgunErr},
		},
		{
			name:     "a failure after the schedule is spent dead-letters and clears the secrets too",
			attempts: exhausted,
			clear:    []string{secretA},
			mark: func(w outbox.Worker, tx *sql.Tx) error {
				return w.MarkFailed(context.Background(), tx, rowID, exhausted, errors.New("still down"), time.Now())
			},
			want: rowWant{status: testStatusDead, attempts: exhausted + 1, reason: outbox.RetriesExhausted, lastError: "still down", cleared: []string{secretA}},
		},
		{
			name: "dead-lettering now is outright, not a retry",
			mark: func(w outbox.Worker, tx *sql.Tx) error {
				return w.MarkDeadLetteredNow(context.Background(), tx, rowID, testDeadReason)
			},
			want: rowWant{status: testStatusDead, reason: outbox.OwnData, lastError: testDeadReason},
		},
		{
			// A suppressed address can never stop being suppressed by
			// waiting, so ADR-0029 skips the backoff schedule rather than
			// spending five attempts rediscovering the same refusal. The
			// plaintext invite token must not survive on a row that will
			// never send.
			name:  "a suppressed address dead-letters without retrying and clears the secrets",
			clear: []string{secretA, secretB},
			mark: func(w outbox.Worker, tx *sql.Tx) error {
				return w.MarkFailed(context.Background(), tx, rowID, 0, suppressedErr, time.Now())
			},
			want: rowWant{status: testStatusDead, reason: outbox.OwnData, lastError: suppressedErr.Error(), cleared: []string{secretA, secretB}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := newRowEnv(t, c.attempts)
			w := newTestWorker(&mail.FakeSender{}, c.clear...)

			env.inTx(func(tx *sql.Tx) error { return c.mark(w, tx) })

			env.expect(c.want)
		})
	}
}

func TestWorker_SendAll(t *testing.T) {
	cases := []struct {
		name          string
		sender        *scriptSender
		addrs         []string
		wantDelivered int
		want          rowWant
	}{
		{
			name:   "no addresses is skipped with nothing to mail, and says so",
			sender: &scriptSender{},
			want:   rowWant{status: testStatusSkipped, skip: outbox.SkipNoRecipients},
		},
		{
			name:          "every address is mailed and the row is sent",
			sender:        &scriptSender{},
			addrs:         []string{addrA, addrB},
			wantDelivered: 2,
			want:          rowWant{status: testStatusSent},
		},
		{
			name:          "the first failure stops the send and schedules a retry",
			sender:        &scriptSender{failAt: 2},
			addrs:         []string{addrA, addrB, addrC},
			wantDelivered: 2,
			want:          rowWant{status: testStatusPending, attempts: 1},
		},
		{
			// One Practice owner who complained must not cost the other
			// owners a Notification they never objected to.
			name:          "a suppressed address is skipped, not a stop",
			sender:        &scriptSender{refuse: refusing(addrB)},
			addrs:         []string{addrA, addrB, addrC},
			wantDelivered: 2,
			want:          rowWant{status: testStatusSent},
		},
		{
			// With nobody left to mail, the row records why rather than
			// looking like a successful send that delivered nothing.
			name:   "every address suppressed dead-letters with the reason",
			sender: &scriptSender{refuse: refusing(addrA, addrB)},
			addrs:  []string{addrA, addrB},
			want:   rowWant{status: testStatusDead, reason: outbox.OwnData, lastError: mail.ErrSuppressed.Error()},
		},
		{
			// A suppression must not turn a transient Mailgun outage into
			// a permanent dead-letter.
			name:          "a real failure after a suppressed address still retries",
			sender:        &scriptSender{refuse: refusing(addrA), failAll: true},
			addrs:         []string{addrA, addrB},
			wantDelivered: 1,
			want:          rowWant{status: testStatusPending, attempts: 1},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := newRowEnv(t, 0)
			w := newTestWorker(c.sender)

			env.inTx(func(tx *sql.Tx) error {
				return w.SendAll(t.Context(), tx, rowID, 0, time.Now(), c.addrs, "subject", "text")
			})

			if len(c.sender.sent) != c.wantDelivered {
				t.Errorf("sender saw %d messages, want %d", len(c.sender.sent), c.wantDelivered)
			}
			env.expect(c.want)
		})
	}
}

type testRow struct {
	id           string
	attemptCount int
}

const testClaimQuery = `SELECT id, attempt_count FROM outbox_test_rows
	 WHERE ` + outbox.ClaimDue + `
	 ORDER BY next_attempt_at
	 LIMIT $1
	 FOR UPDATE SKIP LOCKED`

func scanTestRow(rows *sql.Rows) (testRow, error) {
	var r testRow
	if err := rows.Scan(&r.id, &r.attemptCount); err != nil {
		return r, fmt.Errorf("scan test row: %w", err)
	}
	return r, nil
}

func TestProcessPending_ClaimsDueRowsInOrderAndSkipsNotYetDue(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "due-1", 0, -time.Minute)
	insertTestRow(t, db, "not-due", 0, time.Hour)
	env := &rowEnv{T: t, DB: db}
	w := newTestWorker(&mail.FakeSender{})

	var handled []string
	handle := func(ctx context.Context, tx *sql.Tx, w outbox.Worker, r testRow, now time.Time) error {
		handled = append(handled, r.id)
		return w.MarkSent(ctx, tx, r.id, now)
	}
	env.inTx(func(tx *sql.Tx) error {
		return w.HandlePending(t.Context(), tx, testClaimQuery, scanTestRow, handle)
	})

	if len(handled) != 1 || handled[0] != "due-1" {
		t.Fatalf("handled = %v, want [due-1]", handled)
	}
	if got := readTestRow(t, db, "not-due"); got.status != testStatusPending {
		t.Fatalf("not-due status = %q, want unchanged %s", got.status, testStatusPending)
	}
}

// pendingRun is HandlePending over the claim query in a transaction left
// uncommitted, so a test can read what it returned.
func pendingRun(env *rowEnv, query string, handle func(context.Context, *sql.Tx, outbox.Worker, testRow, time.Time) error) error {
	env.T.Helper()
	tx := env.DB.BeginAdmin(env.T)
	err := newTestWorker(&mail.FakeSender{}).HandlePending(env.T.Context(), tx, query, scanTestRow, handle)
	if err != nil {
		return fmt.Errorf("handle pending: %w", err)
	}
	return nil
}

func TestProcessPending_HandleErrorStopsAndPropagates(t *testing.T) {
	env := newRowEnv(t, 0)
	wantErr := errors.New("handle boom")

	err := pendingRun(env, testClaimQuery, func(context.Context, *sql.Tx, outbox.Worker, testRow, time.Time) error {
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestProcessPending_ScanErrorPropagates(t *testing.T) {
	env := newRowEnv(t, 0)
	// Selects one column fewer than scanTestRow expects, so rows.Scan
	// fails deterministically -- proves ProcessPending's scan-error path
	// without needing a real DB failure.
	const badQuery = `SELECT id FROM outbox_test_rows WHERE status = 'pending' LIMIT $1 FOR UPDATE SKIP LOCKED`

	err := pendingRun(env, badQuery, func(context.Context, *sql.Tx, outbox.Worker, testRow, time.Time) error {
		t.Fatal("handle should not run when scan fails")
		return nil
	})

	if err == nil {
		t.Fatal("want a scan error, got nil")
	}
}

func TestProcessPending_NoPendingRowsCallsHandleZeroTimes(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	calls := 0

	err := pendingRun(&rowEnv{T: t, DB: db}, testClaimQuery, func(context.Context, *sql.Tx, outbox.Worker, testRow, time.Time) error {
		calls++
		return nil
	})

	if err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if calls != 0 {
		t.Fatalf("handle called %d times, want 0", calls)
	}
}
