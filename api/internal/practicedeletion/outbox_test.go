package practicedeletion_test

import (
	"encoding/json/v2"
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
	return practicedeletion.Worker{
		Sender: sender, Now: time.Now, AppBaseURL: testAppBaseURL, From: "a@b.test", ReplyTo: "support@b.test"}
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
// docs/testing/api.md.
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
	tx := db.BeginAs(t, testdb.SessionVar{Name: outbox.NotificationDoor, Value: "true"})
	if err := w.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func seedOwnerEmail(t *testing.T, db *testdb.DB, staffID, email string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE staff SET email = $2 WHERE id = $1`, staffID, email); err != nil {
		t.Fatalf("seed owner email: %v", err)
	}
}

// runAct seeds a due act row for practiceID, runs the worker once and
// requires the row to be settled as sent, whether or not it acted.
func runAct(t *testing.T, db *testdb.DB, practiceID, act string, sender mail.Sender) {
	t.Helper()
	rowID := seedOutboxRow(t, db, practiceID, act, time.Now().Add(-time.Minute))
	runWorker(t, db, newTestWorker(sender))
	if status := testdb.Scalar[string](t, db.Admin, `SELECT status FROM practice_deletion_outbox WHERE id = $1`, rowID); status != statusSent {
		t.Fatalf("status = %s, want sent", status)
	}
}

// newOwnedPractice is a Practice with one Owner at the given identity.
func newOwnedPractice(t *testing.T, db *testdb.DB, uid string) (practiceID, staffID string) {
	t.Helper()
	return testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
}

func seedClient(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	return testdb.Scalar[string](t, db.Admin,
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Ada', 'ada@example.test') RETURNING id`, practiceID)
}

func practiceDeletedAt(t *testing.T, db *testdb.DB, practiceID string) *time.Time {
	t.Helper()
	return testdb.Scalar[*time.Time](t, db.Admin, `SELECT deleted_at FROM practices WHERE id = $1`, practiceID)
}

func finalizedActivity(t *testing.T, db *testdb.DB, column, practiceID string) string {
	t.Helper()
	return testdb.Scalar[string](t, db.Admin,
		`SELECT `+column+` FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_deletion_finalized'`,
		practiceID)
}

func TestWorker_ReminderMailsCurrentOwners(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := newOwnedPractice(t, db, "owner-reminder-mails")
	seedOwnerEmail(t, db, staffID, "owner@example.test")
	markPendingDeletion(t, db, practiceID, time.Now().Add(7*24*time.Hour))

	sender := &mail.FakeSender{}
	runAct(t, db, practiceID, "reminder", sender)

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
	practiceID, staffID := newOwnedPractice(t, db, "owner-reminder-restored")
	seedOwnerEmail(t, db, staffID, "owner@example.test")
	// No markPendingDeletion call: the Practice was restored (or never
	// pending), so deletion_requested_at is null -- the skip-at-send
	// recheck's own condition.
	sender := &mail.FakeSender{}
	runAct(t, db, practiceID, "reminder", sender)

	if len(sender.Sent()) != 0 {
		t.Fatalf("mails sent = %d, want 0", len(sender.Sent()))
	}
}

func TestWorker_FinalizeErasesClientsForfeitsCreditsAndStampsDeletedAt(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := newOwnedPractice(t, db, "owner-finalize-happy")
	markPendingDeletion(t, db, practiceID, time.Now())
	clientID := seedClient(t, db, practiceID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 3)`, practiceID,
	); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	runAct(t, db, practiceID, "finalize", &mail.FakeSender{})

	if erasedAt := testdb.Scalar[*time.Time](t, db.Admin, `SELECT erased_at FROM clients WHERE id = $1`, clientID); erasedAt == nil {
		t.Fatal("client not erased by finalization")
	}
	if practiceDeletedAt(t, db, practiceID) == nil {
		t.Fatal("practice not stamped deleted_at")
	}
	if balance := testdb.Scalar[int](t, db.Admin, `SELECT COALESCE(SUM(quantity), 0) FROM credit_ledger WHERE practice_id = $1`, practiceID); balance != 0 {
		t.Fatalf("balance after forfeiture = %d, want 0", balance)
	}
	if actorKind := finalizedActivity(t, db, "actor_kind::text", practiceID); actorKind != "system" {
		t.Fatalf("actor_kind = %s, want system", actorKind)
	}
	if actorStaffID := testdb.Scalar[*string](t, db.Admin,
		`SELECT actor_staff_id FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_deletion_finalized'`,
		practiceID); actorStaffID != nil {
		t.Fatalf("actor_staff_id = %v, want null for a system actor", *actorStaffID)
	}
}

// TestWorker_FinalizeDestroysClientFeedbackAndLeavesStaffFeedback is
// #1525's Practice Deletion AC, both halves in one finalization: a
// Client's Feedback is reached through the same client.Erase cascade
// that erases her, with a "close as erased" job for its issue, and a
// Staff member's Feedback from under the deleted Practice stays exactly
// as it was (#1501 Q3: "Staff items survive it"), with no close queued.
func TestWorker_FinalizeDestroysClientFeedbackAndLeavesStaffFeedback(t *testing.T) {
	db := testdb.New(t)
	const portalUID = "portal-finalize-feedback"
	practiceID, staffID := newOwnedPractice(t, db, "owner-finalize-feedback")
	markPendingDeletion(t, db, practiceID, time.Now())
	clientID := seedClient(t, db, practiceID)
	testdb.SeedPortalAccount(t, db, portalUID, portalUID+"@example.test")
	testdb.AttachPortalUser(t, db, portalUID, clientID)

	const feedbackColumns = `id, kind, text, page_url, route_id, app_build, screen_width, browser`
	const feedbackValues = `gen_random_uuid(), 'not_working', 'what she typed', '/x', '/x', 'abc1234', 390, 'Safari 18'`
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback (`+feedbackColumns+`, portal_account, practice_id, client_id, issue_number, sent_at)
		 VALUES (`+feedbackValues+`, $1, $2, $3, 61, now()),
		        (`+feedbackValues+`, $1, NULL, NULL, 62, now())`,
		portalUID, practiceID, clientID,
	); err != nil {
		t.Fatalf("seed client feedback: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback (`+feedbackColumns+`, staff_id, practice_id, roles, issue_number, sent_at)
		 VALUES (`+feedbackValues+`, $1, $2, '{owner}', 63, now())`,
		staffID, practiceID,
	); err != nil {
		t.Fatalf("seed staff feedback: %v", err)
	}

	runAct(t, db, practiceID, "finalize", &mail.FakeSender{})

	if n := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM feedback WHERE portal_account IS NOT NULL`); n != 0 {
		t.Fatalf("the Client's pieces of Feedback = %d, want 0 -- the Deletion reaches them through her Erasure", n)
	}
	if n := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM feedback WHERE staff_id = $1 AND issue_number = 63`, staffID); n != 1 {
		t.Fatalf("the Staff member's pieces of Feedback = %d, want 1 -- a Practice's Deletion leaves Staff Feedback in place", n)
	}
	closes := testdb.Column[int64](t, db.Admin,
		`SELECT issue_number FROM feedback_issue_outbox WHERE act = 'close_erased' AND status = 'pending' ORDER BY issue_number`)
	if len(closes) != 2 || closes[0] != 61 || closes[1] != 62 {
		t.Fatalf("close jobs = %v, want [61 62] -- one per Client piece, none for the Staff piece", closes)
	}
}

func TestWorker_FinalizeSkipsWhenNoLongerPending(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := newOwnedPractice(t, db, "owner-finalize-restored")
	// No markPendingDeletion call: restored before finalization ran.
	runAct(t, db, practiceID, "finalize", &mail.FakeSender{})

	if practiceDeletedAt(t, db, practiceID) != nil {
		t.Fatal("deleted_at set despite the Practice no longer being pending")
	}
}

func TestWorker_FinalizeDoesNotDoubleEraseAnAlreadyErasedClient(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := newOwnedPractice(t, db, "owner-finalize-already-erased")
	markPendingDeletion(t, db, practiceID, time.Now())
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, erased_at) VALUES ($1, 'Erased Client', now())`, practiceID,
	); err != nil {
		t.Fatalf("seed already-erased client: %v", err)
	}

	runAct(t, db, practiceID, "finalize", &mail.FakeSender{})

	var scope struct {
		ClientsErased    int `json:"clientsErased"`
		CreditsForfeited int `json:"creditsForfeited"`
	}
	if err := json.Unmarshal([]byte(finalizedActivity(t, db, "diff::text", practiceID)), &scope); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if scope.ClientsErased != 0 {
		t.Fatalf("ClientsErased = %d, want 0 (the already-erased Client was not re-erased)", scope.ClientsErased)
	}
}
