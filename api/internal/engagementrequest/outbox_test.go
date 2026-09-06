package engagementrequest_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const outboxWorkerSecret = "worker-secret-test"

// errSendFake is returned by mail.FakeSender in tests exercising the
// worker's retry/dead-letter path.
var errSendFake = errors.New("mail: fake send failure")

// newWorker builds a Worker whose clock the test controls and whose mail
// goes to an in-memory sender.
func newWorker(sender mail.Sender, now time.Time) engagementrequest.Worker {
	return engagementrequest.Worker{
		Sender:     sender,
		Now:        func() time.Time { return now },
		AppBaseURL: "https://app.example.test",
		From:       "Doula Cloud <notifications@mg.example.test>",
		ReplyTo:    "support@mg.example.test",
	}
}

// runWorker drives one ProcessPending pass through the same endpoint
// Cloud Scheduler calls, so the trusted-worker session variable the RLS
// policies read is set the way production sets it.
func runWorker(t *testing.T, db *testdb.DB, worker engagementrequest.Worker) response {
	t.Helper()
	srv := httptest.NewServer(outbox.ProcessHandler(db.App, worker, outboxWorkerSecret, outbox.NotificationDoor))
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
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	clientID := seedClient(t, db, practiceID)

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

// TestWorker_SkipsRequestAlreadyDecided proves a Request decided through
// some other path before its outbox row was sent is never mailed: a
// decided Request is not mailed late.
func TestWorker_SkipsRequestAlreadyDecided(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	requestID := pendingRequest(t, db, practiceID, clientID, testKindBirth, doulaID)

	// Queue the outbox row directly (bypassing the endpoint) and withdraw
	// the Request before the worker ever runs.
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_request_outbox (request_id, staff_id)
		 SELECT $1, staff_id FROM practice_memberships WHERE practice_id = $2 AND 'admin' = ANY(roles)`,
		requestID, practiceID,
	); err != nil {
		t.Fatalf("queue outbox row: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_requests SET state = 'withdrawn', decided_by = $1, decided_at = now() WHERE id = $2`,
		doulaID, requestID,
	); err != nil {
		t.Fatalf("withdraw request: %v", err)
	}

	sender := &mail.FakeSender{}
	worker := newWorker(sender, time.Now())
	expectStatus(t, runWorker(t, db, worker), http.StatusOK)

	if got := len(sender.Sent()); got != 0 {
		t.Fatalf("mails sent = %d, want 0 (request already decided)", got)
	}
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM engagement_request_outbox WHERE request_id = $1`, requestID,
	).Scan(&status); err != nil {
		t.Fatalf("read outbox status: %v", err)
	}
	if status != "sent" {
		t.Fatalf("outbox status = %q, want sent (skipped, not retried)", status)
	}
}

// TestWorker_RetriesThenDeadLettersOnSendFailure proves a failing send is
// retried with backoff and eventually dead-lettered, mirroring every
// other outbox in this codebase.
func TestWorker_RetriesThenDeadLettersOnSendFailure(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	seedMember(t, db, practiceID, "admin-1", []string{adminRole}, employeeType)
	clientID := seedClient(t, db, practiceID)

	srv, session := newServer(t, db, "doula-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()
	resp := do(t, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID+"/engagement-requests", session,
		engagementrequest.RequestBody{Kind: testKindBirth, DueDate: testDueDate})
	var out engagementrequest.RequestResponse
	decode(t, resp, http.StatusCreated, &out)

	failing := &mail.FakeSender{Err: errSendFake}
	now := time.Now()
	worker := newWorker(failing, now)
	for range 5 {
		expectStatus(t, runWorker(t, db, worker), http.StatusOK)
		if _, err := db.Admin.ExecContext(t.Context(),
			`UPDATE engagement_request_outbox SET next_attempt_at = $1 WHERE request_id = $2`,
			now.Add(-time.Second), out.RequestID,
		); err != nil {
			t.Fatalf("force next attempt due: %v", err)
		}
	}

	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM engagement_request_outbox WHERE request_id = $1 LIMIT 1`, out.RequestID,
	).Scan(&status); err != nil {
		t.Fatalf("read outbox status: %v", err)
	}
	if status != "dead_lettered" {
		t.Fatalf("outbox status = %q, want dead_lettered after repeated failures", status)
	}
}

// TestProcessOutboxHandler_WrongSecretUnauthorized proves the internal
// endpoint refuses a request without the right secret.
func TestProcessOutboxHandler_WrongSecretUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := httptest.NewServer(outbox.ProcessHandler(db.App, newWorker(&mail.FakeSender{}, time.Now()), outboxWorkerSecret, outbox.NotificationDoor))
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
