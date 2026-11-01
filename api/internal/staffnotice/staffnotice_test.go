package staffnotice_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/staffnotice"
	"doula-cloud/api/internal/testdb"
)

const (
	workerSecret = "worker-secret-test"
	appBaseURL   = "https://app.example.test"
	doulaUID     = "uid-doula"
	doulaAddress = "uid-doula@example.com"
)

type fixture struct {
	db         *testdb.DB
	practiceID string
	ownerID    string
	doulaID    string
	offerID    string
	expiresAt  time.Time
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "uid-owner", []string{"owner"}, "employee")
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, doulaUID, []string{"doula"}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	expiresAt := time.Date(2030, time.March, 8, 19, 15, 0, 0, time.UTC)
	var offerID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area, due_date, offered_by, expires_at)
		 VALUES ($1, $2, 'contractor', 45000, 'A', 'North side', '2030-04-01', $3, $4)
		 RETURNING id`,
		engagementID, doulaID, ownerID, expiresAt,
	).Scan(&offerID); err != nil {
		t.Fatalf("seed offer: %v", err)
	}
	return fixture{db: db, practiceID: practiceID, ownerID: ownerID, doulaID: doulaID, offerID: offerID, expiresAt: expiresAt}
}

func (f fixture) notice() staffnotice.Notice {
	return staffnotice.Notice{
		PracticeID:       f.practiceID,
		RecipientStaffID: f.doulaID,
		Kind:             staffnotice.KindOfferReceived,
		SubjectID:        f.offerID,
		TriggeredBy:      f.ownerID,
	}
}

func (f fixture) queue(t *testing.T, n staffnotice.Notice) {
	t.Helper()
	tx, err := f.db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := staffnotice.Queue(t.Context(), tx, n); err != nil {
		_ = tx.Rollback()
		t.Fatalf("queue: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// run drives one pass through the endpoint the drain calls, with the
// notification door set as production sets it.
func (f fixture) run(t *testing.T, now time.Time) *mail.FakeSender {
	t.Helper()
	sender := &mail.FakeSender{}
	worker := staffnotice.NewWorker(outbox.Mailer{
		Sender: sender, Now: func() time.Time { return now }, AppBaseURL: appBaseURL,
		From: "DoulaCloud <notifications@mg.example.test>", ReplyTo: "support@mg.example.test",
	})
	srv := httptest.NewServer(outbox.ProcessHandler(f.db.App, worker, internalauth.FromSecret(workerSecret), outbox.NotificationDoor))
	t.Cleanup(srv.Close)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Internal-Secret", workerSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("worker status = %d: %s", resp.StatusCode, body)
	}
	return sender
}

type row struct {
	status  string
	address *string
	count   int
}

func (f fixture) read(t *testing.T) row {
	t.Helper()
	var r row
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) OVER (), status::text, sent_to_address FROM staff_notice_outbox WHERE subject_id = $1`,
		f.offerID,
	).Scan(&r.count, &r.status, &r.address); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	return r
}

func TestWorker_MailsAnOfferToHerWithTheLinkToThatPractice(t *testing.T) {
	f := newFixture(t)
	f.queue(t, f.notice())

	sent := f.run(t, time.Date(2030, time.March, 1, 12, 0, 0, 0, time.UTC)).Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	msg := sent[0]
	if msg.To != doulaAddress {
		t.Fatalf("to = %q, want %q", msg.To, doulaAddress)
	}
	if msg.Subject != "You've been offered work on DoulaCloud" {
		t.Fatalf("subject = %q", msg.Subject)
	}
	link := appBaseURL + "/practices/" + f.practiceID + "/offers"
	if !strings.Contains(msg.Text, link) {
		t.Fatalf("text = %q, want the link %q", msg.Text, link)
	}
	// The Practice's zone is America/New_York: 19:15 UTC on March 8,
	// 2030 is 2:15pm EST.
	if !strings.Contains(msg.Text, "Answer by March 8, 2030, 2:15pm EST.") {
		t.Fatalf("text = %q, want the expiry in the Practice's zone", msg.Text)
	}
	if strings.Contains(msg.Text, "Test Practice") {
		t.Fatalf("text names the Practice: %q", msg.Text)
	}

	r := f.read(t)
	if r.status != statusSent || r.address == nil || *r.address != doulaAddress {
		t.Fatalf("row = (%s, %v), want sent to %s", r.status, r.address, doulaAddress)
	}
}

func TestQueue_ASecondQueueWhilePendingWritesNothing(t *testing.T) {
	f := newFixture(t)
	f.queue(t, f.notice())
	f.queue(t, f.notice())

	if r := f.read(t); r.count != 1 {
		t.Fatalf("rows = %d, want 1", r.count)
	}
}

func TestWorker_MailsNothingOnceTheOfferIsNoLongerOpen(t *testing.T) {
	cases := map[string]func(t *testing.T, f fixture) time.Time{
		"declined": func(t *testing.T, f fixture) time.Time {
			if _, err := f.db.Admin.ExecContext(t.Context(),
				`UPDATE engagement_offers SET state = 'declined', decided_at = now(), decided_by = staff_id WHERE id = $1`, f.offerID,
			); err != nil {
				t.Fatalf("decline: %v", err)
			}
			return time.Date(2030, time.March, 1, 12, 0, 0, 0, time.UTC)
		},
		"past its expiry": func(_ *testing.T, f fixture) time.Time {
			return f.expiresAt.Add(time.Minute)
		},
		"she left the Practice": func(t *testing.T, f fixture) time.Time {
			if _, err := f.db.Admin.ExecContext(t.Context(),
				`DELETE FROM practice_memberships WHERE staff_id = $1`, f.doulaID,
			); err != nil {
				t.Fatalf("remove membership: %v", err)
			}
			return time.Date(2030, time.March, 1, 12, 0, 0, 0, time.UTC)
		},
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.queue(t, f.notice())
			now := prepare(t, f)

			if sent := f.run(t, now).Sent(); len(sent) != 0 {
				t.Fatalf("sent %d messages, want none", len(sent))
			}
			if r := f.read(t); r.status != statusSent || r.address != nil {
				t.Fatalf("row = (%s, %v), want sent with no address", r.status, r.address)
			}
		})
	}
}

func TestWorker_MailsNothingForAnOfferThatDoesNotExist(t *testing.T) {
	f := newFixture(t)
	n := f.notice()
	n.SubjectID = "00000000-0000-0000-0000-000000000001"
	f.queue(t, n)

	if sent := f.run(t, time.Date(2030, time.March, 1, 12, 0, 0, 0, time.UTC)).Sent(); len(sent) != 0 {
		t.Fatalf("sent %d messages, want none", len(sent))
	}
}
