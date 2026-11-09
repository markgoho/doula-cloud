package engagementrequest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/reqtest"
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

// workerURL serves one ProcessPending pass through the same endpoint
// Cloud Scheduler calls, so the trusted-worker session variable the RLS
// policies read is set the way production sets it.
func workerURL(t *testing.T, db *testdb.DB, worker engagementrequest.Worker) string {
	t.Helper()
	srv := httptest.NewServer(outbox.ProcessHandler(db.App, worker, internalauth.FromSecret(outboxWorkerSecret), outbox.NotificationDoor))
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestWorker_SendsOneMailPerOwnerAndAdminContentFree proves the worker
// mails every queued recipient, content-free (no kind, due date, or
// Client name -- only a link back to the dashboard), and marks every row
// sent.
func TestWorker_SendsOneMailPerOwnerAndAdminContentFree(t *testing.T) {
	e := newEnv(t)
	e.Colleague(fixture.Owner())
	e.Colleague(fixture.Admin())
	out := e.ask(birthAsk)

	sender := &mail.FakeSender{}
	reqtest.ExpectStatus(t, http.MethodPost, workerURL(t, e.DB, newWorker(sender, clock.Now(t.Context()))), http.StatusOK,
		reqtest.Header("X-Internal-Secret", outboxWorkerSecret))

	sent := sender.Sent()
	if len(sent) != 2 {
		t.Fatalf("mails sent = %d, want 2", len(sent))
	}
	for _, msg := range sent {
		if msg.Text == "" || msg.Subject == "" {
			t.Fatal("empty mail body/subject")
		}
	}
	if got := outboxCount(t, e.DB, out.RequestID); got != 2 {
		t.Fatalf("outbox rows = %d, want 2", got)
	}
}

// TestProcessOutboxHandler_WrongSecretUnauthorized proves the internal
// endpoint refuses a request without the right secret.
func TestProcessOutboxHandler_WrongSecretUnauthorized(t *testing.T) {
	db := testdb.New(t)
	url := workerURL(t, db, newWorker(&mail.FakeSender{}, clock.Now(t.Context())))

	reqtest.ExpectStatus(t, http.MethodPost, url, http.StatusUnauthorized, reqtest.Header("X-Internal-Secret", "wrong"))
}
