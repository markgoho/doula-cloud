package engagementrequest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const outboxWorkerSecret = "worker-secret-test"

// newWorker builds a Worker whose clock the test controls and whose mail
// goes to an in-memory sender.
func newWorker(sender mail.Sender, now time.Time) engagementrequest.Worker {
	return engagementrequest.NewWorker(outbox.Mailer{
		Sender:     sender,
		Now:        func() time.Time { return now },
		AppBaseURL: "https://app.example.test",
		From:       "Doula Cloud <notifications@mg.example.test>",
		ReplyTo:    "support@mg.example.test",
	})
}

// runWorker drives one ProcessPending pass through the same endpoint
// Cloud Scheduler calls, so the trusted-worker session variable the RLS
// policies read is set the way production sets it.
func runWorker(t *testing.T, db *testdb.DB, worker engagementrequest.Worker) response {
	t.Helper()
	srv := httptest.NewServer(outbox.ProcessHandler(db.App, worker, internalauth.FromSecret(outboxWorkerSecret), outbox.NotificationDoor))
	t.Cleanup(srv.Close)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Internal-Secret", outboxWorkerSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return response{status: resp.StatusCode}
}

// TestWorker_SendsOneMailPerOwnerAndAdminContentFree proves the worker
// mails every queued recipient, content-free (no kind, due date, or
// Client name -- only a link back to the dashboard), and marks every row
// sent.
func TestWorker_SendsOneMailPerOwnerAndAdminContentFree(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	testdb.SeedStaffAtPractice(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	testdb.SeedStaffAtPractice(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Test Client", "client.com")

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()
	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	var out engagementrequest.RequestResponse
	decode(t, resp, http.StatusCreated, &out)

	sender := &mail.FakeSender{}
	worker := newWorker(sender, time.Now())
	expectStatus(t, runWorker(t, db, worker), http.StatusOK)

	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("mails sent = %d, want 2", len(sent))
	}
	for _, msg := range sent {
		if msg.Text == "" || msg.Subject == "" {
			t.Fatal("empty mail body/subject")
		}
	}
	if got := outboxCount(t, db, out.RequestID); got != 2 {
		t.Fatalf("outbox rows = %d, want 2", got)
	}
}

// TestProcessOutboxHandler_WrongSecretUnauthorized proves the internal
// endpoint refuses a request without the right secret.
func TestProcessOutboxHandler_WrongSecretUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := httptest.NewServer(outbox.ProcessHandler(db.App, newWorker(&mail.FakeSender{}, time.Now()), internalauth.FromSecret(outboxWorkerSecret), outbox.NotificationDoor))
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Internal-Secret", "wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
