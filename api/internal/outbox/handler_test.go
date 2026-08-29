package outbox_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// stubProcessor is outbox.ProcessHandler's Processor test double.
// rollbackFirst lets a test drive ProcessHandler's own tx.Commit into a
// real failure (Commit on an already-finished tx returns sql.ErrTxDone)
// without needing a genuine DB outage.
type stubProcessor struct {
	err           error
	rollbackFirst bool
	called        bool
}

func (s *stubProcessor) ProcessPending(_ context.Context, tx *sql.Tx) error {
	s.called = true
	if s.rollbackFirst {
		_ = tx.Rollback()
	}
	return s.err
}

func newHandlerServer(db *testdb.DB, worker outbox.Processor, secret string) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /process", outbox.ProcessHandler(db.App, worker, secret))
	return httptest.NewServer(mux)
}

func postProcess(t *testing.T, srv *httptest.Server, headerSecret string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/process", nil)
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

func TestProcessHandler_WrongSecretUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newHandlerServer(db, &stubProcessor{}, "correct-secret")
	defer srv.Close()

	resp := postProcess(t, srv, "wrong-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestProcessHandler_EmptyConfiguredSecretAlwaysUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newHandlerServer(db, &stubProcessor{}, "")
	defer srv.Close()

	resp := postProcess(t, srv, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestProcessHandler_CorrectSecretRunsWorkerAndReturns200(t *testing.T) {
	db := testdb.New(t)
	worker := &stubProcessor{}
	srv := newHandlerServer(db, worker, "correct-secret")
	defer srv.Close()

	resp := postProcess(t, srv, "correct-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !worker.called {
		t.Fatal("worker.ProcessPending was not called")
	}
}

func TestProcessHandler_BeginTxFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	handler := outbox.ProcessHandler(db.App, &stubProcessor{}, "correct-secret")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/process", nil)
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

func TestProcessHandler_ProcessPendingFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	srv := newHandlerServer(db, &stubProcessor{err: errors.New("boom")}, "correct-secret")
	defer srv.Close()

	resp := postProcess(t, srv, "correct-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestProcessHandler_CommitFailureAfterProcessPendingReturns500(t *testing.T) {
	db := testdb.New(t)
	srv := newHandlerServer(db, &stubProcessor{rollbackFirst: true}, "correct-secret")
	defer srv.Close()

	resp := postProcess(t, srv, "correct-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
