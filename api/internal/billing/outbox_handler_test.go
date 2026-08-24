package billing_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/testdb"
)

func newLowCreditOutboxServer(db *testdb.DB, worker billing.Worker, secret string) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /internal/notifications/process-low-credit-outbox", billing.ProcessOutboxHandler(db.App, worker, secret))
	return httptest.NewServer(mux)
}

func postProcessLowCreditOutbox(t *testing.T, srv *httptest.Server, headerSecret string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/internal/notifications/process-low-credit-outbox", nil)
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

func TestLowCreditProcessOutboxHandler_WrongSecretUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newLowCreditOutboxServer(db, newTestWorker(&mail.FakeSender{}), "correct-secret")
	defer srv.Close()

	resp := postProcessLowCreditOutbox(t, srv, "wrong-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLowCreditProcessOutboxHandler_EmptyConfiguredSecretAlwaysUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newLowCreditOutboxServer(db, newTestWorker(&mail.FakeSender{}), "")
	defer srv.Close()

	resp := postProcessLowCreditOutbox(t, srv, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLowCreditProcessOutboxHandler_CorrectSecretSendsDueRows(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-handler")
	outboxID := seedLowCreditOutboxRow(t, db, practiceID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	srv := newLowCreditOutboxServer(db, newTestWorker(sender), "correct-secret")
	defer srv.Close()

	resp := postProcessLowCreditOutbox(t, srv, "correct-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _ := lowCreditOutboxRowState(t, db, outboxID)
	if status != testLowCreditStatusSent {
		t.Fatalf("status = %q, want %s", status, testLowCreditStatusSent)
	}
	if len(sender.Sent()) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sender.Sent()))
	}
}

// TestLowCreditProcessOutboxHandler_BeginTxFailureRollsBackAndReturns500
// mirrors portalinvite's own version of this test: a canceled request
// context fails the handler closed (500), the same direction every other
// failure in this handler takes.
func TestLowCreditProcessOutboxHandler_BeginTxFailureRollsBackAndReturns500(t *testing.T) {
	db := testdb.New(t)
	handler := billing.ProcessOutboxHandler(db.App, newTestWorker(&mail.FakeSender{}), "correct-secret")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/internal/notifications/process-low-credit-outbox", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Internal-Secret", "correct-secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
