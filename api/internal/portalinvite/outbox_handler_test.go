package portalinvite_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/testdb"
)

func newOutboxServer(db *testdb.DB, worker portalinvite.Worker, secret string) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /internal/notifications/process-outbox", portalinvite.ProcessOutboxHandler(db.App, worker, secret))
	return httptest.NewServer(mux)
}

func postProcessOutbox(t *testing.T, srv *httptest.Server, headerSecret string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/internal/notifications/process-outbox", nil)
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
	srv := newOutboxServer(db, newTestWorker(&mail.FakeSender{}), "correct-secret")
	defer srv.Close()

	resp := postProcessOutbox(t, srv, "wrong-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestProcessOutboxHandler_EmptyConfiguredSecretAlwaysUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newOutboxServer(db, newTestWorker(&mail.FakeSender{}), "")
	defer srv.Close()

	resp := postProcessOutbox(t, srv, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestProcessOutboxHandler_CorrectSecretSendsDueRows(t *testing.T) {
	db := testdb.New(t)
	clientID, _ := seedPendingPortalInvite(t, db)
	portalUserID := portalUserIDForClient(t, db, clientID)
	outboxID := seedOutboxRow(t, db, portalUserID, 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	srv := newOutboxServer(db, newTestWorker(sender), "correct-secret")
	defer srv.Close()

	resp := postProcessOutbox(t, srv, "correct-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testOutboxStatusSent)
	}
	if len(sender.Sent()) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sender.Sent()))
	}
}

// TestProcessOutboxHandler_BeginTxFailureRollsBackAndReturns500 confirms
// a canceled request context fails the handler closed (500), the same
// direction every other failure in this handler takes. It lands on
// BeginTx's own already-justified coverage:ignore branch rather than
// ProcessPending's -- the outbox worker has no non-DB failure mode to
// drive independently (unlike accept.go/middleware.go's rollback tests,
// which use an ordinary bad-input 400; this endpoint takes no input past
// the secret check above BeginTx).
func TestProcessOutboxHandler_BeginTxFailureRollsBackAndReturns500(t *testing.T) {
	db := testdb.New(t)
	handler := portalinvite.ProcessOutboxHandler(db.App, newTestWorker(&mail.FakeSender{}), "correct-secret")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/internal/notifications/process-outbox", nil)
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
