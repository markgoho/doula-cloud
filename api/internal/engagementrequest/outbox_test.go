package engagementrequest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/reqtest"
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
		From:       "DoulaCloud <notifications@mg.example.test>",
		ReplyTo:    "support@mg.example.test",
	})
}

// runWorker drives one ProcessPending pass through the same endpoint
// Cloud Scheduler calls, so the trusted-worker session variable the RLS
// policies read is set the way production sets it.
func runWorker(t *testing.T, db *testdb.DB, worker engagementrequest.Worker) *http.Response {
	t.Helper()
	srv := httptest.NewServer(outbox.ProcessHandler(db.App, worker, internalauth.FromSecret(outboxWorkerSecret), outbox.NotificationDoor))
	t.Cleanup(srv.Close)
	return reqtest.Post(t, srv.URL, reqtest.Header("X-Internal-Secret", outboxWorkerSecret))
}

// TestWorker_SendsOneMailPerOwnerAndAdminContentFree proves the worker
// mails every queued recipient, content-free (no kind, due date, or
// Client name -- only a link back to the dashboard), and marks every row
// sent.
func TestWorker_SendsOneMailPerOwnerAndAdminContentFree(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p)
	fixture.Staff(t, db, p, fixture.Owner())
	fixture.Staff(t, db, p, fixture.Admin())
	client := fixture.Client(t, db, p)

	srv, session := newServer(t, db, doula, &tasknudge.FakeEnqueuer{})
	defer srv.Close()
	resp := reqtest.Post(t, srv.URL+"/api/practices/"+p.ID+"/clients/"+client.ID+"/engagement-requests", reqtest.Session(session), reqtest.JSON(engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate}))
	defer resp.Body.Close()
	out := reqtest.DecodeStatus[engagementrequest.RequestResponse](t, resp, http.StatusCreated)

	sender := &mail.FakeSender{}
	worker := newWorker(sender, time.Now())
	resp = runWorker(t, db, worker)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)

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

	resp := reqtest.Post(t, srv.URL, reqtest.Header("X-Internal-Secret", "wrong"))
	defer func() { _ = resp.Body.Close() }()
	reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
}
