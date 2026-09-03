package mfarecoverymail_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mfarecoverymail"
	"doula-cloud/api/internal/testdb"
)

func newOutboxServer(db *testdb.DB, worker mfarecoverymail.Worker, secret string) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /internal/notifications/process-mfa-recovery-outbox", mfarecoverymail.ProcessOutboxHandler(db.App, worker, secret))
	return httptest.NewServer(mux)
}

func postProcessOutbox(t *testing.T, srv *httptest.Server, headerSecret string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/internal/notifications/process-mfa-recovery-outbox", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if headerSecret != "" {
		req.Header.Set("X-Internal-Secret", headerSecret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestProcessOutboxHandler_WrongSecretUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newOutboxServer(db, newWorker(&mail.FakeSender{}, authntest.NewFakeAccountManager()), "correct-secret")
	defer srv.Close()

	resp := postProcessOutbox(t, srv, "wrong-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestProcessOutboxHandler_CorrectSecretSendsDueRows(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedStaffRow(t, db, "subject-handler", "Someone")
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("owner-uid-handler", "owner-handler@example.com", true)
	outboxID := seedOutboxRow(t, db, "owner-uid-handler", subjectID, "99990000", time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	srv := newOutboxServer(db, newWorker(sender, accounts), "correct-secret")
	defer srv.Close()

	resp := postProcessOutbox(t, srv, "correct-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _, _ := rowState(t, db, outboxID)
	if status != statusSent {
		t.Fatalf("status = %q, want %s", status, statusSent)
	}
	if len(sender.Sent()) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sender.Sent()))
	}
}
