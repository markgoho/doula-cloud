package outbox_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/testdb"
)

// suppressingSender refuses every address in refuse with
// mail.ErrSuppressed and delivers the rest -- the shape
// mailsuppress.Sender presents to an outbox worker, without this
// package having to depend on it or on the email_suppressions table.
type suppressingSender struct {
	mu     sync.Mutex
	sent   []mail.Message
	refuse map[string]bool
}

func (s *suppressingSender) Send(_ context.Context, msg mail.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refuse[msg.To] {
		return fmt.Errorf("%w: %s", mail.ErrSuppressed, msg.To)
	}
	s.sent = append(s.sent, msg)
	return nil
}

func newSuppressingSender(refused ...string) *suppressingSender {
	refuse := make(map[string]bool, len(refused))
	for _, a := range refused {
		refuse[a] = true
	}
	return &suppressingSender{refuse: refuse}
}

// A suppressed address can never stop being suppressed by waiting, so
// ADR-0029 skips the backoff schedule entirely rather than spending five
// attempts over about a day rediscovering the same refusal.
func TestMarkFailed_SuppressedDeadLettersWithoutRetrying(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	w := newTestWorker(&mail.FakeSender{})

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	sendErr := fmt.Errorf("%w: someone@example.test", mail.ErrSuppressed)
	if err := w.MarkFailed(t.Context(), tx, "row-1", 0, sendErr, time.Now()); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got := readTestRow(t, db, "row-1")
	if got.status != testStatusDead {
		t.Fatalf("status = %q, want %s (dead-lettered on the spot)", got.status, testStatusDead)
	}
	if got.attemptCount != 0 {
		t.Fatalf("attempt_count = %d, want 0 (no retry was scheduled)", got.attemptCount)
	}
	if got.lastError.String != sendErr.Error() {
		t.Fatalf("last_error = %q, want %q", got.lastError.String, sendErr.Error())
	}
}

// The suppressed-address branch clears the same terminal columns every
// other dead-letter path does -- a plaintext invite token must not
// survive on a row that will never send.
func TestMarkFailed_SuppressedClearsConfiguredColumns(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	w := newTestWorker(&mail.FakeSender{}, "secret_a", "secret_b")

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := w.MarkFailed(t.Context(), tx, "row-1", 0, mail.ErrSuppressed, time.Now()); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	got := readTestRow(t, db, "row-1")
	if got.secretA.Valid || got.secretB.Valid {
		t.Fatalf("secrets survived a suppressed dead-letter: a=%v b=%v", got.secretA, got.secretB)
	}
}

// One Practice owner who complained must not cost the other owners a
// Notification they never objected to.
func TestSendAll_SkipsSuppressedAddressAndStillMarksSent(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := newSuppressingSender(addrB)
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
		t.Fatalf("delivered %d messages, want 2 (the suppressed one skipped, not a stop)", len(sender.sent))
	}
	got := readTestRow(t, db, "row-1")
	if got.status != testStatusSent {
		t.Fatalf("status = %q, want %s", got.status, testStatusSent)
	}
}

// With nobody left to mail, the row records why rather than looking like
// a successful send that delivered nothing.
func TestSendAll_EverySuppressedAddressDeadLetters(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := newSuppressingSender(addrA, addrB)
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

	if len(sender.sent) != 0 {
		t.Fatalf("delivered %d messages, want 0", len(sender.sent))
	}
	got := readTestRow(t, db, "row-1")
	if got.status != testStatusDead {
		t.Fatalf("status = %q, want %s", got.status, testStatusDead)
	}
	if got.lastError.String == "" {
		t.Fatal("last_error is empty, want the suppression reason")
	}
}

// A real send failure alongside a suppressed address still retries: the
// suppression must not turn a transient Mailgun outage into a permanent
// dead-letter.
func TestSendAll_RealFailureAfterSuppressedStillRetries(t *testing.T) {
	db := testdb.New(t)
	createTestTable(t, db)
	insertTestRow(t, db, "row-1", 0, time.Now())
	sender := newSuppressingSender(addrA)
	w := newTestWorker(failAfterSuppressed{inner: sender})

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

	got := readTestRow(t, db, "row-1")
	if got.status != testStatusPending {
		t.Fatalf("status = %q, want %s (scheduled for retry)", got.status, testStatusPending)
	}
	if got.attemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1", got.attemptCount)
	}
}

// failAfterSuppressed refuses whatever inner refuses, and fails every
// other address with an ordinary transient error.
type failAfterSuppressed struct{ inner *suppressingSender }

func (s failAfterSuppressed) Send(ctx context.Context, msg mail.Message) error {
	if err := s.inner.Send(ctx, msg); err != nil {
		return err
	}
	return errTransient
}

// errTransient stands in for an ordinary Mailgun failure -- the kind
// ADR-0010's backoff schedule exists for.
var errTransient = errors.New("transient send failure")

// The three scratch recipient addresses SendAll's multi-recipient tests
// share.
const (
	addrA = "a@example.test"
	addrB = "b@example.test"
	addrC = "c@example.test"
)
