package welcomecredits_test

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/welcomecredits"
)

const (
	appBaseURL = "https://app.example.test"
	ownerRole  = "owner"
	doulaRole  = "doula"
	employee   = "employee"
	statusSent = "sent"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

func newWorker(sender mail.Sender) welcomecredits.Worker {
	return welcomecredits.Worker{
		Sender: sender, Now: time.Now, AppBaseURL: appBaseURL, From: "hello@mg.doula.test", ReplyTo: "support@doula.test",
	}
}

// seedDueRow writes the row signup would have written, already past its
// fire time by Postgres's own clock.
func seedDueRow(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO welcome_credits_outbox (practice_id, next_attempt_at, created_at)
		 VALUES ($1, now() - interval '1 minute', now() - interval '14 days') RETURNING id`,
		practiceID,
	).Scan(&id); err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	return id
}

func runWorker(t *testing.T, db *testdb.DB, w welcomecredits.Worker) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("open door: %v", err)
	}
	if err := w.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

type rowState struct {
	status    string
	sentAt    sql.NullTime
	skip      sql.NullString
	notified  sql.NullString
	attempts  int
	lastError sql.NullString
}

func readRow(t *testing.T, db *testdb.DB, id string) rowState {
	t.Helper()
	var r rowState
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, sent_at, skip_reason, notified_owner_staff_ids::text, attempt_count, last_error
		 FROM welcome_credits_outbox WHERE id = $1`, id,
	).Scan(&r.status, &r.sentAt, &r.skip, &r.notified, &r.attempts, &r.lastError); err != nil {
		t.Fatalf("read outbox row: %v", err)
	}
	return r
}

func activityCount(t *testing.T, db *testdb.DB, practiceID string) (count int, actorKind string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), coalesce(max(actor_kind::text), '') FROM activity
		 WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'welcome_credits_email_sent'`,
		practiceID,
	).Scan(&count, &actorKind); err != nil {
		t.Fatalf("read activity: %v", err)
	}
	return count, actorKind
}

func exec(t *testing.T, db *testdb.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func TestWorker_MailsEveryCurrentOwnerAndRecordsTheSend(t *testing.T) {
	db := testdb.New(t)
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "wc-owner", []string{ownerRole, doulaRole}, employee)
	secondOwnerID := testdb.SeedStaffAtPractice(t, db, practiceID, "wc-second-owner", []string{ownerRole}, employee)
	testdb.SeedStaffAtPractice(t, db, practiceID, "wc-doula", []string{doulaRole}, employee)
	id := seedDueRow(t, db, practiceID)

	sender := &mail.FakeSender{}
	runWorker(t, db, newWorker(sender))

	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("mails sent = %d, want 2 (one per Owner, none to the Doula)", len(sent))
	}
	to := map[string]bool{}
	for _, m := range sent {
		to[m.To] = true
		if m.From != "hello@mg.doula.test" {
			t.Errorf("From = %q, want the Platform identity", m.From)
		}
		if m.Subject != "DoulaCloud: your 3 Welcome credits are still there" {
			t.Errorf("Subject = %q", m.Subject)
		}
		link := appBaseURL + "/practices/" + practiceID + "\n"
		if !strings.Contains(m.Text, "Start with a Client here:\n"+link) {
			t.Errorf("Text does not link the Practice overview: %q", m.Text)
		}
		for _, banned := range []string{"/billing", "Test Practice", "unsubscribe"} {
			if strings.Contains(m.Text, banned) || strings.Contains(m.Subject, banned) {
				t.Errorf("message carries %q: %q", banned, m.Text)
			}
		}
	}
	if !to["wc-owner@example.com"] || !to["wc-second-owner@example.com"] {
		t.Errorf("recipients = %v, want both Owners", to)
	}

	row := readRow(t, db, id)
	if row.status != statusSent || !row.sentAt.Valid || row.skip.Valid {
		t.Errorf("row = %+v, want sent with no skip reason", row)
	}
	want := map[string]bool{"{" + ownerID + "," + secondOwnerID + "}": true, "{" + secondOwnerID + "," + ownerID + "}": true}
	if !want[row.notified.String] {
		t.Errorf("notified_owner_staff_ids = %q, want both Owners", row.notified.String)
	}

	if n, actor := activityCount(t, db, practiceID); n != 1 || actor != "system" {
		t.Errorf("activity rows = %d actor %q, want 1 by system", n, actor)
	}

	runWorker(t, db, newWorker(sender))
	if len(sender.Sent()) != 2 {
		t.Errorf("a second drain mailed again: %d mails", len(sender.Sent()))
	}
}

func TestWorker_MailsTheOwnerWhoseAddressIsNotSuppressed(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "wc-half-a", []string{ownerRole}, employee)
	testdb.SeedStaffAtPractice(t, db, practiceID, "wc-half-b", []string{ownerRole}, employee)
	exec(t, db, `INSERT INTO email_suppressions (address, cause) VALUES ('wc-half-a@example.com', 'complaint')`)
	id := seedDueRow(t, db, practiceID)

	sender := &mail.FakeSender{}
	runWorker(t, db, newWorker(sender))

	if row := readRow(t, db, id); row.status != statusSent || row.skip.Valid {
		t.Errorf("row = %+v, want sent and not skipped while one Owner is reachable", row)
	}
	if n, _ := activityCount(t, db, practiceID); n != 1 {
		t.Errorf("activity rows = %d, want 1", n)
	}
}

func TestWorker_SkipsWithNothingMailed(t *testing.T) {
	cases := []struct {
		name    string
		reason  string
		arrange func(t *testing.T, db *testdb.DB, practiceID string)
	}{
		{
			name:   "an Engagement exists",
			reason: "engagement_started",
			arrange: func(t *testing.T, db *testdb.DB, practiceID string) {
				t.Helper()
				testdb.SeedEngagement(t, db, practiceID)
			},
		},
		{
			name:   "a founding grant is held",
			reason: "founding_grant",
			arrange: func(t *testing.T, db *testdb.DB, practiceID string) {
				t.Helper()
				testdb.SeedFoundingGrant(t, db, practiceID, 3, time.Now())
			},
		},
		{
			name:   "deletion is pending",
			reason: "practice_deleting",
			arrange: func(t *testing.T, db *testdb.DB, practiceID string) {
				t.Helper()
				exec(t, db, `UPDATE practices SET deletion_requested_at = now(), deletion_finalize_at = now() + interval '30 days' WHERE id = $1`, practiceID)
			},
		},
		{
			name:   "the Practice is deleted",
			reason: "practice_deleting",
			arrange: func(t *testing.T, db *testdb.DB, practiceID string) {
				t.Helper()
				exec(t, db, `UPDATE practices SET deleted_at = now() WHERE id = $1`, practiceID)
			},
		},
		{
			name:   "every Owner address is suppressed",
			reason: "owners_suppressed",
			arrange: func(t *testing.T, db *testdb.DB, _ string) {
				t.Helper()
				exec(t, db, `INSERT INTO email_suppressions (address, cause) VALUES ('wc-skip-owner@example.com', 'bounce')`)
			},
		},
		{
			name:   "no Owner is left",
			reason: "no_owners",
			arrange: func(t *testing.T, db *testdb.DB, practiceID string) {
				t.Helper()
				exec(t, db, `UPDATE practice_memberships SET roles = '{doula}' WHERE practice_id = $1`, practiceID)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "wc-skip-owner", []string{ownerRole}, employee)
			tc.arrange(t, db, practiceID)
			id := seedDueRow(t, db, practiceID)

			sender := &mail.FakeSender{}
			runWorker(t, db, newWorker(sender))

			if len(sender.Sent()) != 0 {
				t.Errorf("mails sent = %d, want 0", len(sender.Sent()))
			}
			row := readRow(t, db, id)
			if row.status != statusSent || !row.sentAt.Valid || row.skip.String != tc.reason {
				t.Errorf("row = %+v, want sent with skip_reason %q", row, tc.reason)
			}
			if row.notified.Valid {
				t.Errorf("notified_owner_staff_ids = %q, want NULL on a skip", row.notified.String)
			}
			if n, _ := activityCount(t, db, practiceID); n != 0 {
				t.Errorf("activity rows = %d, want 0 on a skip", n)
			}
		})
	}
}

func TestWorker_AFailedSendRetriesAndRecordsNothing(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "wc-fail-owner", []string{ownerRole}, employee)
	id := seedDueRow(t, db, practiceID)

	runWorker(t, db, newWorker(&mail.FakeSender{Err: errors.New("mailgun down")}))

	row := readRow(t, db, id)
	if row.status != "pending" || row.attempts != 1 || !row.lastError.Valid {
		t.Errorf("row = %+v, want pending for a retry", row)
	}
	if n, _ := activityCount(t, db, practiceID); n != 0 {
		t.Errorf("activity rows = %d, want 0 before a send succeeds", n)
	}
}

func TestEnqueue_SetsTheFireTimeFourteenDaysOut(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "wc-enqueue-owner", []string{ownerRole}, employee)
	now := time.Date(2027, time.February, 1, 9, 0, 0, 0, time.UTC)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := welcomecredits.Enqueue(t.Context(), tx, practiceID, now); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var due, created time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT next_attempt_at, created_at FROM welcome_credits_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&due, &created); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if !due.Equal(now.Add(14*24*time.Hour)) || !created.Equal(now) {
		t.Errorf("next_attempt_at = %v created_at = %v, want %v and %v", due, created, now.Add(14*24*time.Hour), now)
	}
}

func TestWorker_LeavesARowThatIsNotDueYet(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "wc-early-owner", []string{ownerRole}, employee)
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO welcome_credits_outbox (practice_id, next_attempt_at, created_at)
		 VALUES ($1, now() + interval '13 days', now()) RETURNING id`, practiceID,
	).Scan(&id); err != nil {
		t.Fatalf("seed: %v", err)
	}

	sender := &mail.FakeSender{}
	runWorker(t, db, newWorker(sender))

	if len(sender.Sent()) != 0 || readRow(t, db, id).status != "pending" {
		t.Errorf("a row not yet due was acted on")
	}
}
