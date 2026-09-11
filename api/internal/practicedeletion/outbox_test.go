package practicedeletion_test

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/practicedeletion"
	"doula-cloud/api/internal/testdb"
)

const testAppBaseURL = "https://app.example.test" //nolint:gosec // test fixture URL, not a credential
const statusSent = "sent"

func newTestWorker(sender mail.Sender) practicedeletion.Worker {
	return practicedeletion.Worker{Mailer: outbox.Mailer{
		Sender: sender, Now: time.Now, AppBaseURL: testAppBaseURL, From: "a@b.test", ReplyTo: "support@b.test",
	}}
}

// seedOutboxRow inserts a practice_deletion_outbox row directly, the
// same shape billing's own seedLowCreditOutboxRow uses for that table.
//
// runWorker's claim query reads next_attempt_at against Postgres's own
// now(), so a caller that means "due" must pass a margin far larger than
// any plausible host-vs-container clock skew -- time.Now().Add(-time.Minute)
// throughout this file -- rather than a bare time.Now(), which a VM-backed
// container engine (Podman/Docker Desktop on macOS) can read as not yet
// due. See "A due-time fixture must not compare two clocks" in
// docs/testing.md.
func seedOutboxRow(t *testing.T, db *testdb.DB, practiceID, act string, nextAttemptAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practice_deletion_outbox (practice_id, act, next_attempt_at) VALUES ($1, $2, $3) RETURNING id`,
		practiceID, act, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed practice_deletion_outbox row: %v", err)
	}
	return id
}

// markPendingDeletion seeds the practices columns InitiateHandler would
// have set, without going through the handler -- these tests exercise
// the worker in isolation from the request that enqueues its rows.
func markPendingDeletion(t *testing.T, db *testdb.DB, practiceID string, finalizeAt time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET deletion_requested_at = now(), deletion_finalize_at = $2 WHERE id = $1`,
		practiceID, finalizeAt,
	); err != nil {
		t.Fatalf("mark pending deletion: %v", err)
	}
}

func runWorker(t *testing.T, db *testdb.DB, w practicedeletion.Worker) {
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

func outboxRowStatus(t *testing.T, db *testdb.DB, id string) string {
	t.Helper()
	var status string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT status FROM practice_deletion_outbox WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("query outbox row: %v", err)
	}
	return status
}

func seedOwnerEmail(t *testing.T, db *testdb.DB, staffID, email string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE staff SET email = $2 WHERE id = $1`, staffID, email); err != nil {
		t.Fatalf("seed owner email: %v", err)
	}
}

func TestWorker_ReminderMailsCurrentOwners(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "owner-reminder-mails", []string{ownerRole}, "employee")
	seedOwnerEmail(t, db, staffID, "owner@example.test")
	finalizeAt := time.Now().Add(7 * 24 * time.Hour)
	markPendingDeletion(t, db, practiceID, finalizeAt)
	rowID := seedOutboxRow(t, db, practiceID, "reminder", time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	if status := outboxRowStatus(t, db, rowID); status != statusSent {
		t.Fatalf("status = %s, want sent", status)
	}
	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("mails sent = %d, want 1", len(sent))
	}
	if sent[0].To != "owner@example.test" {
		t.Fatalf("To = %s, want owner@example.test", sent[0].To)
	}
}

func TestWorker_ReminderSkipsWhenNoLongerPending(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "owner-reminder-restored", []string{ownerRole}, "employee")
	seedOwnerEmail(t, db, staffID, "owner@example.test")
	// No markPendingDeletion call: the Practice was restored (or never
	// pending), so deletion_requested_at is null -- the skip-at-send
	// recheck's own condition.
	rowID := seedOutboxRow(t, db, practiceID, "reminder", time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	if status := outboxRowStatus(t, db, rowID); status != statusSent {
		t.Fatalf("status = %s, want sent", status)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("mails sent = %d, want 0", len(sender.Sent()))
	}
}

func TestWorker_FinalizeErasesClientsForfeitsCreditsAndStampsDeletedAt(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "owner-finalize-happy", []string{ownerRole}, "employee")
	markPendingDeletion(t, db, practiceID, time.Now())

	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Ada', 'ada@example.test') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 3)`, practiceID,
	); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	rowID := seedOutboxRow(t, db, practiceID, "finalize", time.Now().Add(-time.Minute))
	runWorker(t, db, newTestWorker(&mail.FakeSender{}))

	if status := outboxRowStatus(t, db, rowID); status != statusSent {
		t.Fatalf("status = %s, want sent", status)
	}

	var erasedAt sql.NullTime
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT erased_at FROM clients WHERE id = $1`, clientID).Scan(&erasedAt); err != nil {
		t.Fatalf("query client: %v", err)
	}
	if !erasedAt.Valid {
		t.Fatal("client not erased by finalization")
	}

	var deletedAt sql.NullTime
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT deleted_at FROM practices WHERE id = $1`, practiceID).Scan(&deletedAt); err != nil {
		t.Fatalf("query practice: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatal("practice not stamped deleted_at")
	}

	var balance int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT COALESCE(SUM(quantity), 0) FROM credit_ledger WHERE practice_id = $1`, practiceID).Scan(&balance); err != nil {
		t.Fatalf("sum ledger: %v", err)
	}
	if balance != 0 {
		t.Fatalf("balance after forfeiture = %d, want 0", balance)
	}

	var action, actorKind string
	var actorStaffID sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_kind::text, actor_staff_id FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_deletion_finalized'`,
		practiceID,
	).Scan(&action, &actorKind, &actorStaffID); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if actorKind != "system" {
		t.Fatalf("actor_kind = %s, want system", actorKind)
	}
	if actorStaffID.Valid {
		t.Fatalf("actor_staff_id = %v, want null for a system actor", actorStaffID.String)
	}
}

func TestWorker_FinalizeSkipsWhenNoLongerPending(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "owner-finalize-restored", []string{ownerRole}, "employee")
	// No markPendingDeletion call: restored before finalization ran.
	rowID := seedOutboxRow(t, db, practiceID, "finalize", time.Now().Add(-time.Minute))

	runWorker(t, db, newTestWorker(&mail.FakeSender{}))

	if status := outboxRowStatus(t, db, rowID); status != statusSent {
		t.Fatalf("status = %s, want sent", status)
	}
	var deletedAt sql.NullTime
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT deleted_at FROM practices WHERE id = $1`, practiceID).Scan(&deletedAt); err != nil {
		t.Fatalf("query practice: %v", err)
	}
	if deletedAt.Valid {
		t.Fatal("deleted_at set despite the Practice no longer being pending")
	}
}

func TestWorker_FinalizeDoesNotDoubleEraseAnAlreadyErasedClient(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "owner-finalize-already-erased", []string{ownerRole}, "employee")
	markPendingDeletion(t, db, practiceID, time.Now())

	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, erased_at) VALUES ($1, 'Erased Client', now()) RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed already-erased client: %v", err)
	}

	rowID := seedOutboxRow(t, db, practiceID, "finalize", time.Now().Add(-time.Minute))
	runWorker(t, db, newTestWorker(&mail.FakeSender{}))

	if status := outboxRowStatus(t, db, rowID); status != statusSent {
		t.Fatalf("status = %s, want sent", status)
	}

	var diff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_deletion_finalized'`,
		practiceID,
	).Scan(&diff); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	var scope struct {
		ClientsErased    int `json:"clientsErased"`
		CreditsForfeited int `json:"creditsForfeited"`
	}
	if err := json.Unmarshal(diff, &scope); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if scope.ClientsErased != 0 {
		t.Fatalf("ClientsErased = %d, want 0 (the already-erased Client was not re-erased)", scope.ClientsErased)
	}
}
