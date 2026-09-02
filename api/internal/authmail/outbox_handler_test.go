package authmail_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/testdb"
)

func postProcessOutbox(t *testing.T, srv *httptest.Server, headerSecret string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
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

func TestProcessTokenMailOutboxHandler_WrongSecretUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := httptest.NewServer(authmail.ProcessTokenMailOutboxHandler(db.App, newTokenMailWorker(&mail.FakeSender{}, authntest.NewFakeAccountManager()), "correct-secret"))
	defer srv.Close()

	resp := postProcessOutbox(t, srv, "wrong-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestProcessTokenMailOutboxHandler_CorrectSecretSendsDueRows(t *testing.T) {
	db := testdb.New(t)
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("uid-handler-1", "person@example.com", false)
	rowID := seedTokenMailRow(t, db, "uid-handler-1", authmail.KindEmailVerification, "verify-token", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	srv := httptest.NewServer(authmail.ProcessTokenMailOutboxHandler(db.App, newTokenMailWorker(sender, accounts), "correct-secret"))
	defer srv.Close()

	resp := postProcessOutbox(t, srv, "correct-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _, _ := tokenMailRowState(t, db, rowID)
	if status != statusSent {
		t.Fatalf("status = %q, want sent", status)
	}
}

func TestProcessTokenMailOutboxHandler_BeginTxFailureRollsBackAndReturns500(t *testing.T) {
	db := testdb.New(t)
	handler := authmail.ProcessTokenMailOutboxHandler(db.App, newTokenMailWorker(&mail.FakeSender{}, authntest.NewFakeAccountManager()), "correct-secret")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/internal/notifications/process-staff-token-mail-outbox", nil)
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

func TestProcessEmailChangeOutboxHandler_WrongSecretUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := httptest.NewServer(authmail.ProcessEmailChangeOutboxHandler(db.App, newEmailChangeWorker(&mail.FakeSender{}), "correct-secret"))
	defer srv.Close()

	resp := postProcessOutbox(t, srv, "wrong-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestProcessEmailChangeOutboxHandler_CorrectSecretSendsDueRows(t *testing.T) {
	db := testdb.New(t)
	rowID := seedEmailChangeRow(t, db, "uid-handler-2", "old@example.com", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	srv := httptest.NewServer(authmail.ProcessEmailChangeOutboxHandler(db.App, newEmailChangeWorker(sender), "correct-secret"))
	defer srv.Close()

	resp := postProcessOutbox(t, srv, "correct-secret")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _ := emailChangeRowState(t, db, rowID)
	if status != statusSent {
		t.Fatalf("status = %q, want sent", status)
	}
}

func TestProcessEmailChangeOutboxHandler_BeginTxFailureRollsBackAndReturns500(t *testing.T) {
	db := testdb.New(t)
	handler := authmail.ProcessEmailChangeOutboxHandler(db.App, newEmailChangeWorker(&mail.FakeSender{}), "correct-secret")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/internal/notifications/process-staff-email-change-outbox", nil)
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
