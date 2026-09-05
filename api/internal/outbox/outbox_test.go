package outbox_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

const (
	testTable         = "outbox_test_rows"
	testStatusPending = "pending"
	testStatusSent    = "sent"
	testStatusDead    = "dead_lettered"
	testFrom          = "sender@example.test"
	testReplyTo       = "reply@example.test"
)

// createTestTable makes a scratch outbox-shaped table in db's own cloned
// database -- one per test, via testdb.New -- rather than a session-scoped
// CREATE TEMP TABLE, which a *sql.DB connection pool could hand a later
// call a different connection than the one that created it. Run against
// db.Admin throughout: a table this package invents has no grants for the
// low-privilege app_test role db.App connects as, and this package's own
// Worker methods carry no RLS concern to prove.
func createTestTable(t *testing.T, db *testdb.DB) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), `
		CREATE TABLE outbox_test_rows (
			id text PRIMARY KEY,
			status text NOT NULL DEFAULT 'pending',
			attempt_count int NOT NULL DEFAULT 0,
			next_attempt_at timestamptz NOT NULL DEFAULT now(),
			sent_at timestamptz,
			last_error text,
			secret_a text,
			secret_b text
		)`); err != nil {
		t.Fatalf("create test table: %v", err)
	}
}

func insertTestRow(t *testing.T, db *testdb.DB, id string, attemptCount int, nextAttemptAt time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO outbox_test_rows (id, attempt_count, next_attempt_at, secret_a, secret_b)
		 VALUES ($1, $2, $3, 'a-secret', 'b-secret')`,
		id, attemptCount, nextAttemptAt,
	); err != nil {
		t.Fatalf("insert test row: %v", err)
	}
}

type testRowState struct {
	status       string
	attemptCount int
	lastError    sql.NullString
	secretA      sql.NullString
	secretB      sql.NullString
}

func readTestRow(t *testing.T, db *testdb.DB, id string) testRowState {
	t.Helper()
	var s testRowState
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count, last_error, secret_a, secret_b FROM outbox_test_rows WHERE id = $1`, id,
	).Scan(&s.status, &s.attemptCount, &s.lastError, &s.secretA, &s.secretB); err != nil {
		t.Fatalf("read test row: %v", err)
	}
	return s
}

func newTestWorker(sender mail.Sender, clearOnTerminal ...string) outbox.Worker {
	return outbox.Worker{
		Sender: sender, Now: time.Now, From: testFrom, ReplyTo: testReplyTo,
		Table: testTable, ClearOnTerminal: clearOnTerminal,
	}
}

// countingSender fails on its failAt'th call (1-indexed), or never if
// failAt is zero -- lets SendAll's stop-at-first-failure behavior be
// proven with more than one recipient, which mail.FakeSender's single
// fixed Err can't do.
type countingSender struct {
	mu     sync.Mutex
	sent   []mail.Message
	failAt int
}

func (s *countingSender) Send(_ context.Context, msg mail.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, msg)
	if s.failAt != 0 && len(s.sent) == s.failAt {
		return errors.New("send failed")
	}
	return nil
}

func TestMarkSent_NoClearOnTerminal(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	w := newTestWorker(&mail.FakeSender{})

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := w.MarkSent(t.Context(), tx, "row-1", time.Now()); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got := readTestRow(t, db, "row-1")
	if got.status != testStatusSent {
		t.Fatalf("status = %q, want %s", got.status, testStatusSent)
	}
	if !got.secretA.Valid || got.secretA.String != "a-secret" {
		t.Fatalf("secret_a = %+v, want unchanged", got.secretA)
	}
}

func TestMarkSent_ClearsConfiguredColumns(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	w := newTestWorker(&mail.FakeSender{}, "secret_a", "secret_b")

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := w.MarkSent(t.Context(), tx, "row-1", time.Now()); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got := readTestRow(t, db, "row-1")
	if got.secretA.Valid || got.secretB.Valid {
		t.Fatalf("secrets = %+v/%+v, want both cleared", got.secretA, got.secretB)
	}
}

func TestMarkFailed_SchedulesRetryBeforeScheduleExhausted(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	w := newTestWorker(&mail.FakeSender{})

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := w.MarkFailed(t.Context(), tx, "row-1", 0, errors.New("mailgun down"), time.Now()); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got := readTestRow(t, db, "row-1")
	if got.status != testStatusPending {
		t.Fatalf("status = %q, want %s", got.status, testStatusPending)
	}
	if got.attemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1", got.attemptCount)
	}
	if !got.lastError.Valid || got.lastError.String != "mailgun down" {
		t.Fatalf("last_error = %+v, want %q", got.lastError, "mailgun down")
	}
}

func TestMarkFailed_DeadLettersAfterScheduleExhausted(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", len(outbox.BackoffSchedule)-1, time.Now())
	w := newTestWorker(&mail.FakeSender{}, "secret_a")

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := w.MarkFailed(t.Context(), tx, "row-1", len(outbox.BackoffSchedule)-1, errors.New("still down"), time.Now()); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got := readTestRow(t, db, "row-1")
	if got.status != testStatusDead {
		t.Fatalf("status = %q, want %s", got.status, testStatusDead)
	}
	if got.secretA.Valid {
		t.Fatalf("secret_a = %+v, want cleared on dead-letter too", got.secretA)
	}
}

func TestMarkDeadLetteredNow(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	w := newTestWorker(&mail.FakeSender{})

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := w.MarkDeadLetteredNow(t.Context(), tx, "row-1", "no address on file"); err != nil {
		t.Fatalf("MarkDeadLetteredNow: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got := readTestRow(t, db, "row-1")
	if got.status != testStatusDead {
		t.Fatalf("status = %q, want %s", got.status, testStatusDead)
	}
	if got.attemptCount != 0 {
		t.Fatalf("attempt_count = %d, want unchanged 0 (outright dead-letter, not a retry)", got.attemptCount)
	}
	if !got.lastError.Valid || got.lastError.String != "no address on file" {
		t.Fatalf("last_error = %+v, want %q", got.lastError, "no address on file")
	}
}

func TestSendAll_EmptyAddressesMarksSentWithNothingToMail(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := &countingSender{}
	w := newTestWorker(sender)

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := w.SendAll(t.Context(), tx, "row-1", 0, time.Now(), nil, "subject", "text"); err != nil {
		t.Fatalf("SendAll: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if len(sender.sent) != 0 {
		t.Fatalf("sent %d messages, want 0", len(sender.sent))
	}
	got := readTestRow(t, db, "row-1")
	if got.status != testStatusSent {
		t.Fatalf("status = %q, want %s", got.status, testStatusSent)
	}
}

func TestSendAll_MailsEveryAddressAndMarksSent(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := &countingSender{}
	w := newTestWorker(sender)

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	addrs := []string{addrA, addrB}
	if err := w.SendAll(t.Context(), tx, "row-1", 0, time.Now(), addrs, "subject", "text"); err != nil {
		t.Fatalf("SendAll: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if len(sender.sent) != 2 {
		t.Fatalf("sent %d messages, want 2", len(sender.sent))
	}
	got := readTestRow(t, db, "row-1")
	if got.status != testStatusSent {
		t.Fatalf("status = %q, want %s", got.status, testStatusSent)
	}
}

func TestSendAll_StopsAtFirstFailureAndMarksFailed(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := &countingSender{failAt: 2}
	w := newTestWorker(sender)

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	addrs := []string{addrA, addrB, addrC}
	if err := w.SendAll(t.Context(), tx, "row-1", 0, time.Now(), addrs, "subject", "text"); err != nil {
		t.Fatalf("SendAll: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if len(sender.sent) != 2 {
		t.Fatalf("attempted %d sends, want 2 (stop at first failure)", len(sender.sent))
	}
	got := readTestRow(t, db, "row-1")
	if got.status != testStatusPending {
		t.Fatalf("status = %q, want %s (scheduled for retry)", got.status, testStatusPending)
	}
	if got.attemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1", got.attemptCount)
	}
}

type testRow struct {
	id           string
	attemptCount int
}

const testClaimQuery = `SELECT id, attempt_count FROM outbox_test_rows
	 WHERE status = 'pending' AND next_attempt_at <= now()
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
	insertTestRow(t, db, "due-1", 0, time.Now().Add(-time.Minute))
	insertTestRow(t, db, "not-due", 0, time.Now().Add(time.Hour))
	sender := &mail.FakeSender{}
	w := newTestWorker(sender)

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	var handled []string
	handle := func(ctx context.Context, tx *sql.Tx, w outbox.Worker, r testRow, now time.Time) error {
		handled = append(handled, r.id)
		return w.MarkSent(ctx, tx, r.id, now)
	}
	if err := outbox.ProcessPending(t.Context(), tx, w, testClaimQuery, scanTestRow, handle); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if len(handled) != 1 || handled[0] != "due-1" {
		t.Fatalf("handled = %v, want [due-1]", handled)
	}
	if got := readTestRow(t, db, "not-due"); got.status != testStatusPending {
		t.Fatalf("not-due status = %q, want unchanged %s", got.status, testStatusPending)
	}
}

func TestProcessPending_HandleErrorStopsAndPropagates(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	w := newTestWorker(&mail.FakeSender{})

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	wantErr := errors.New("handle boom")
	handle := func(_ context.Context, _ *sql.Tx, _ outbox.Worker, _ testRow, _ time.Time) error {
		return wantErr
	}
	if err := outbox.ProcessPending(t.Context(), tx, w, testClaimQuery, scanTestRow, handle); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestProcessPending_ScanErrorPropagates(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	w := newTestWorker(&mail.FakeSender{})

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Selects one column fewer than scanTestRow expects, so rows.Scan
	// fails deterministically -- proves ProcessPending's scan-error path
	// without needing a real DB failure.
	const badQuery = `SELECT id FROM outbox_test_rows WHERE status = 'pending' LIMIT $1 FOR UPDATE SKIP LOCKED`
	handle := func(_ context.Context, _ *sql.Tx, _ outbox.Worker, _ testRow, _ time.Time) error {
		t.Fatal("handle should not run when scan fails")
		return nil
	}
	if err := outbox.ProcessPending(t.Context(), tx, w, badQuery, scanTestRow, handle); err == nil {
		t.Fatal("want a scan error, got nil")
	}
}

func TestProcessPending_NoPendingRowsCallsHandleZeroTimes(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	w := newTestWorker(&mail.FakeSender{})

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	calls := 0
	handle := func(_ context.Context, _ *sql.Tx, _ outbox.Worker, _ testRow, _ time.Time) error {
		calls++
		return nil
	}
	if err := outbox.ProcessPending(t.Context(), tx, w, testClaimQuery, scanTestRow, handle); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if calls != 0 {
		t.Fatalf("handle called %d times, want 0", calls)
	}
}
