package payments_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

func newPayoutOutboxServer(db *testdb.DB, worker payments.Worker, secret string) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /internal/notifications/process-payout-outbox", payments.ProcessOutboxHandler(db.App, worker, secret))
	return httptest.NewServer(mux)
}

func postProcessPayoutOutbox(t *testing.T, srv *httptest.Server, headerSecret string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/internal/notifications/process-payout-outbox", nil)
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

func TestPayoutProcessOutboxHandler_WrongSecretUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newPayoutOutboxServer(db, newTestPayoutWorker(&mail.FakeSender{}), "correct-secret")
	defer srv.Close()

	resp := postProcessPayoutOutbox(t, srv, "wrong-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestPayoutProcessOutboxHandler_EmptyConfiguredSecretAlwaysUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newPayoutOutboxServer(db, newTestPayoutWorker(&mail.FakeSender{}), "")
	defer srv.Close()

	resp := postProcessPayoutOutbox(t, srv, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestPayoutProcessOutboxHandler_CorrectSecretSendsDueRows(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedOwner(t, db, "owner-handler")
	setRequirementsDue(t, db, practiceID, []string{testRequirementDOB})
	outboxID := seedPayoutOutboxRow(t, db, practiceID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	srv := newPayoutOutboxServer(db, newTestPayoutWorker(sender), "correct-secret")
	defer srv.Close()

	resp := postProcessPayoutOutbox(t, srv, "correct-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _ := payoutOutboxRowState(t, db, outboxID)
	if status != testPayoutStatusSent {
		t.Fatalf("status = %q, want %s", status, testPayoutStatusSent)
	}
	if len(sender.Sent()) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sender.Sent()))
	}
}

// TestPayoutProcessOutboxHandler_BeginTxFailureRollsBackAndReturns500
// mirrors billing's own version of this test: a canceled request context
// fails the handler closed (500), the same direction every other failure
// in this handler takes.
func TestPayoutProcessOutboxHandler_BeginTxFailureRollsBackAndReturns500(t *testing.T) {
	db := testdb.New(t)
	handler := payments.ProcessOutboxHandler(db.App, newTestPayoutWorker(&mail.FakeSender{}), "correct-secret")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/internal/notifications/process-payout-outbox", nil)
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
