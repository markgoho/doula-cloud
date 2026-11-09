package outbox_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
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
	testDeadReason    = "no address on file"

	rowID      = "row-1"
	secretA    = "secret_a"
	secretB    = "secret_b"
	recipient  = "recipient@example.test"
	mailgunErr = "mailgun down"
)

// The three scratch recipient addresses SendAll's multi-recipient tests
// share.
const (
	addrA = "a@example.test"
	addrB = "b@example.test"
	addrC = "c@example.test"
)

// errTransient stands in for an ordinary Mailgun failure -- the kind
// ADR-0010's backoff schedule exists for.
var errTransient = errors.New("transient send failure")

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
			dead_letter_reason outbox_dead_letter_reason,
			secret_a text,
			secret_b text,
			CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL))
		)`); err != nil {
		t.Fatalf("create test table: %v", err)
	}
}

// insertTestRow seeds one row whose due-time is an offset from the
// *database's* clock: dueIn of 0 means "due now", a negative value
// "overdue", a positive one "not yet due". Rounded to whole microseconds,
// which is Postgres timestamp resolution anyway.
//
// It deliberately takes no host-side time.Time. testClaimQuery decides
// dueness against Postgres's own now(), so a due-time written from the
// host's clock is a comparison of two clocks nothing keeps in step, and
// under a VM-backed container engine it decided these tests by the sign
// of the drift -- see #987, and "A due-time fixture must not compare two
// clocks" in docs/testing/api.md for the rule this follows.
func insertTestRow(t *testing.T, db *testdb.DB, id string, attemptCount int, dueIn time.Duration) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO outbox_test_rows (id, attempt_count, next_attempt_at, secret_a, secret_b)
		 VALUES ($1, $2, now() + $3 * interval '1 microsecond', 'a-secret', 'b-secret')`,
		id, attemptCount, dueIn.Microseconds(),
	); err != nil {
		t.Fatalf("insert test row: %v", err)
	}
}

type testRowState struct {
	status       string
	attemptCount int
	lastError    sql.NullString
	reason       sql.NullString
	secretA      sql.NullString
	secretB      sql.NullString
}

func readTestRow(t *testing.T, db *testdb.DB, id string) testRowState {
	t.Helper()
	var s testRowState
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count, last_error, dead_letter_reason, secret_a, secret_b FROM outbox_test_rows WHERE id = $1`, id,
	).Scan(&s.status, &s.attemptCount, &s.lastError, &s.reason, &s.secretA, &s.secretB); err != nil {
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

// rowEnv is what a worker test starts from: a database holding the
// scratch outbox table with one due row, rowID, that has already been
// attempted attempts times.
type rowEnv struct {
	T  *testing.T
	DB *testdb.DB
}

func newRowEnv(t *testing.T, attempts int) *rowEnv {
	t.Helper()
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, rowID, attempts, 0)
	return &rowEnv{T: t, DB: db}
}

// inTx runs fn in a transaction and commits it, the way a drain does.
func (e *rowEnv) inTx(fn func(tx *sql.Tx) error) {
	e.T.Helper()
	tx, err := e.DB.Admin.BeginTx(e.T.Context(), nil)
	if err != nil {
		e.T.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		e.T.Fatalf("worker: %v", err)
	}
	if err := tx.Commit(); err != nil {
		e.T.Fatalf("commit: %v", err)
	}
}

// rowWant is what a worker leaves on rowID. cleared names the secret
// columns the worker nulled; the other of the two keeps its seeded value.
type rowWant struct {
	status   string
	attempts int
	reason   outbox.DeadLetterReason
	// lastError, when set, must appear in the row's last_error.
	lastError string
	cleared   []string
}

func (e *rowEnv) expect(want rowWant) {
	e.T.Helper()
	got := readTestRow(e.T, e.DB, rowID)
	if got.status != want.status {
		e.T.Errorf("status = %q, want %q", got.status, want.status)
	}
	if got.attemptCount != want.attempts {
		e.T.Errorf("attempt_count = %d, want %d", got.attemptCount, want.attempts)
	}
	if got.reason.String != string(want.reason) {
		e.T.Errorf("dead_letter_reason = %+v, want %q", got.reason, want.reason)
	}
	if want.lastError != "" && !strings.Contains(got.lastError.String, want.lastError) {
		e.T.Errorf("last_error = %q, want it to contain %q", got.lastError.String, want.lastError)
	}
	for col, got := range map[string]sql.NullString{secretA: got.secretA, secretB: got.secretB} {
		if cleared := slices.Contains(want.cleared, col); got.Valid == cleared {
			e.T.Errorf("%s = %+v, want cleared = %t", col, got, cleared)
		}
	}
}

// scriptSender stands in for the Sender a worker mails through. It refuses
// every address in refuse with mail.ErrSuppressed, the shape
// mailsuppress.Sender presents without this package depending on it or on
// the email_suppressions table, and records every other message it is
// asked to send. That call fails with errTransient on the failAt'th of
// them (1-indexed), or on every one when failAll is set.
type scriptSender struct {
	mu      sync.Mutex
	sent    []mail.Message
	refuse  map[string]bool
	failAt  int
	failAll bool
}

func (s *scriptSender) Send(_ context.Context, msg mail.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refuse[msg.To] {
		return fmt.Errorf("%w: %s", mail.ErrSuppressed, msg.To)
	}
	s.sent = append(s.sent, msg)
	if s.failAll || (s.failAt != 0 && len(s.sent) == s.failAt) {
		return errTransient
	}
	return nil
}

func refusing(addresses ...string) map[string]bool {
	refuse := make(map[string]bool, len(addresses))
	for _, a := range addresses {
		refuse[a] = true
	}
	return refuse
}
