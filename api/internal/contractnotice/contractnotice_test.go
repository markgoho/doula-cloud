package contractnotice_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/contractnotice"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

const (
	workerSecret  = "worker-secret-test"
	appBaseURL    = "https://app.example.test"
	clientName    = "Amara"
	practiceNamed = "Sunrise Doula Care"
	clientAddress = "amara@example.com"
)

type fixture struct {
	db           *testdb.DB
	practiceID   string
	staffID      string
	clientID     string
	engagementID string
	contractID   string
}

func newFixture(t *testing.T, email string) fixture {
	t.Helper()
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, practiceNamed)
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "uid-owner", []string{"owner"}, "employee")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, clientName, email)
	var contractID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents, sent_at)
		 VALUES ($1, 'sent', 'Agreement.', 15000, now()) RETURNING id`,
		engagementID,
	).Scan(&contractID); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	return fixture{db: db, practiceID: practiceID, staffID: staffID, clientID: clientID, engagementID: engagementID, contractID: contractID}
}

func (f fixture) queue(t *testing.T, kind contractnotice.Kind) {
	t.Helper()
	tx := f.db.BeginAdmin(t)
	if err := contractnotice.Queue(t.Context(), tx, f.practiceID, f.contractID, kind, f.staffID); err != nil {
		_ = tx.Rollback()
		t.Fatalf("queue: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// run drives one pass through the endpoint the drain calls, with the
// notification door set as production sets it.
func (f fixture) run(t *testing.T, sender mail.Sender) {
	t.Helper()
	now := time.Now()
	worker := contractnotice.NewWorker(outbox.Mailer{
		Sender: sender, Now: func() time.Time { return now }, AppBaseURL: appBaseURL,
		From: "DoulaCloud <notifications@mg.example.test>", ReplyTo: "noreply@mg.example.test",
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
}

func (f fixture) mails(t *testing.T) []mail.Message {
	t.Helper()
	sender := &mail.FakeSender{}
	f.run(t, sender)
	return sender.Sent()
}

type outboxRow struct {
	status  string
	address *string
	reason  *string
	skip    *string
}

func (f fixture) read(t *testing.T) outboxRow {
	t.Helper()
	var r outboxRow
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, sent_to_address, dead_letter_reason::text, skip_reason::text
		   FROM contract_notice_outbox WHERE contract_id = $1`, f.contractID,
	).Scan(&r.status, &r.address, &r.reason, &r.skip); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	return r
}

func (f fixture) acceptedClient(t *testing.T) {
	t.Helper()
	testdb.SeedPortalUser(t, f.db, "uid-amara", f.clientID)
}

func (f fixture) invitedClient(t *testing.T) (token string) {
	t.Helper()
	puID := testdb.SeedPendingPortalInvite(t, f.db, f.clientID)
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT invite_token::text FROM client_portal_users WHERE id = $1`, puID).Scan(&token); err != nil {
		t.Fatalf("read token: %v", err)
	}
	return token
}

func requireContentFree(t *testing.T, msg mail.Message) {
	t.Helper()
	for _, named := range []string{clientName, practiceNamed, "Test Practice", "birth", "15000", "$150"} {
		if strings.Contains(msg.Subject+msg.Text, named) {
			t.Fatalf("mail names %q: %q / %q", named, msg.Subject, msg.Text)
		}
	}
}

func TestWorker_MailsAClientWithALoginHerContract(t *testing.T) {
	cases := map[contractnotice.Kind]string{
		contractnotice.KindSent:     "You have a contract waiting for you",
		contractnotice.KindReminder: "A reminder: you have a contract waiting for you",
	}
	for kind, subject := range cases {
		t.Run(string(kind), func(t *testing.T) {
			f := newFixture(t, clientAddress)
			f.acceptedClient(t)
			f.queue(t, kind)

			sent := f.mails(t)
			if len(sent) != 1 {
				t.Fatalf("sent %d messages, want 1", len(sent))
			}
			msg := sent[0]
			if msg.To != clientAddress || msg.Subject != subject {
				t.Fatalf("to = %q subject = %q", msg.To, msg.Subject)
			}
			link := appBaseURL + "/portal/engagements/" + f.engagementID + "/contract"
			if !strings.Contains(msg.Text, "\n"+link+"\n") {
				t.Fatalf("text = %q, want the link %q on its own line", msg.Text, link)
			}
			requireContentFree(t, msg)
			if got := f.read(t); got.status != "sent" || got.address == nil || *got.address != clientAddress {
				t.Fatalf("row = %+v, want sent to %s", got, clientAddress)
			}
		})
	}
}

func TestWorker_MailsAClientWithOnlyAnInvitationThatInvitation(t *testing.T) {
	f := newFixture(t, clientAddress)
	token := f.invitedClient(t)
	f.queue(t, contractnotice.KindSent)

	sent := f.mails(t)
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	link := appBaseURL + "/portal/accept-invite?token=" + token
	if !strings.Contains(sent[0].Text, "\n"+link+"\n") {
		t.Fatalf("text = %q, want her invitation %q", sent[0].Text, link)
	}
}

func TestWorker_SkipsAContractThatIsNoLongerSent(t *testing.T) {
	for _, status := range []string{"signed", "voided"} {
		t.Run(status, func(t *testing.T) {
			f := newFixture(t, clientAddress)
			f.acceptedClient(t)
			f.queue(t, contractnotice.KindReminder)
			if _, err := f.db.Admin.ExecContext(t.Context(),
				`UPDATE contracts SET status = $2::contract_status, signed_at = now(),
				        voided_at = CASE WHEN $2 = 'voided' THEN now() END WHERE id = $1`, f.contractID, status,
			); err != nil {
				t.Fatalf("move contract: %v", err)
			}

			if sent := f.mails(t); len(sent) != 0 {
				t.Fatalf("sent %d messages, want 0", len(sent))
			}
			if got := f.read(t); got.status != "skipped" || got.skip == nil || *got.skip != "already_resolved" {
				t.Fatalf("row = %+v, want skipped as already_resolved", got)
			}
		})
	}
}

func TestWorker_DeadLettersWhatNoWaitingCanFix(t *testing.T) {
	cases := map[string]struct {
		email  string
		invite func(*testing.T, fixture)
	}{
		"no address on file":                 {"", func(t *testing.T, f fixture) { f.acceptedClient(t) }},
		"never invited":                      {clientAddress, func(*testing.T, fixture) {}},
		"her invitation expired":             {clientAddress, func(t *testing.T, f fixture) { f.expireInvite(t) }},
		"her invitation was never delivered": {clientAddress, func(t *testing.T, f fixture) { f.undeliveredInvite(t) }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, tc.email)
			tc.invite(t, f)
			f.queue(t, contractnotice.KindSent)

			if sent := f.mails(t); len(sent) != 0 {
				t.Fatalf("sent %d messages, want 0", len(sent))
			}
			if got := f.read(t); got.status != "dead_lettered" || got.reason == nil || *got.reason != "own_data" {
				t.Fatalf("row = %+v, want dead-lettered as own_data", got)
			}
		})
	}
}

func (f fixture) expireInvite(t *testing.T) {
	t.Helper()
	f.invitedClient(t)
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`UPDATE client_portal_users SET invite_token_expires_at = now() - interval '1 day' WHERE client_id = $1`, f.clientID,
	); err != nil {
		t.Fatalf("expire invite: %v", err)
	}
}

func (f fixture) undeliveredInvite(t *testing.T) {
	t.Helper()
	f.invitedClient(t)
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`UPDATE portal_invite_outbox SET status = 'dead_lettered', dead_letter_reason = 'retries_exhausted', last_error = 'bounced' WHERE status = 'pending'`,
	); err != nil {
		t.Fatalf("fail invite delivery: %v", err)
	}
}

func TestWorker_RetriesAMailThatFailedToSend(t *testing.T) {
	f := newFixture(t, clientAddress)
	f.acceptedClient(t)
	f.queue(t, contractnotice.KindSent)

	f.run(t, &mail.FakeSender{Err: errors.New("mailgun is down")})

	got := f.read(t)
	if got.status != "pending" {
		t.Fatalf("status = %q, want pending for a retry", got.status)
	}
}

func TestQueue_TwoOfAKindWhilePendingWriteOne(t *testing.T) {
	f := newFixture(t, clientAddress)
	f.queue(t, contractnotice.KindReminder)
	f.queue(t, contractnotice.KindReminder)

	if n := testdb.Scalar[int](t, f.db.Admin, `SELECT count(*) FROM contract_notice_outbox WHERE contract_id = $1`, f.contractID); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}
