package idempotency_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func readBody(t *testing.T, resp *http.Response) map[string]int {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var out map[string]int
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal body %q: %v", b, err)
	}
	return out
}

// TestWrap_ReplaysStoredResponseForSameKey proves the core AC: a retry
// carrying the same Idempotency-Key returns the identical stored response
// without re-running the wrapped handler.
func TestWrap_ReplaysStoredResponseForSameKey(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "same-key-owner")
	var calls int
	srv := newIdempotencyServer(db, "same-key-owner", &calls, http.StatusCreated)
	defer srv.Close()

	first := postWidget(t, srv, practiceID, "retry-key-1")
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first call status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	firstBody := readBody(t, first)

	second := postWidget(t, srv, practiceID, "retry-key-1")
	if second.StatusCode != http.StatusCreated {
		t.Fatalf("second call status = %d, want %d", second.StatusCode, http.StatusCreated)
	}
	secondBody := readBody(t, second)

	if calls != 1 {
		t.Fatalf("handler invoked %d times, want 1 -- business logic must not re-run on replay", calls)
	}
	if firstBody["n"] != secondBody["n"] {
		t.Fatalf("replayed body %v differs from stored body %v", secondBody, firstBody)
	}
	if second.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("replayed Content-Type = %q, want application/json", second.Header.Get("Content-Type"))
	}
}

// TestWrap_RunsHandlerAgainAndRefreshesRowPastTTL proves a key whose
// stored row has aged past the 48-hour TTL is treated as not found (the
// handler re-runs) and that the aged row is refreshed in place, not left
// permanently stale by ON CONFLICT DO UPDATE's WHERE guard.
func TestWrap_RunsHandlerAgainAndRefreshesRowPastTTL(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "ttl-owner")
	var calls int
	srv := newIdempotencyServer(db, "ttl-owner", &calls, http.StatusCreated)
	defer srv.Close()

	first := postWidget(t, srv, practiceID, "ttl-key")
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first call status = %d, want %d", first.StatusCode, http.StatusCreated)
	}
	firstBody := readBody(t, first)
	if calls != 1 {
		t.Fatalf("handler invoked %d times after first call, want 1", calls)
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE idempotency_keys SET created_at = now() - interval '49 hours' WHERE key = 'ttl-key'`,
	); err != nil {
		t.Fatalf("backdate stored row: %v", err)
	}

	second := postWidget(t, srv, practiceID, "ttl-key")
	if second.StatusCode != http.StatusCreated {
		t.Fatalf("second call status = %d, want %d", second.StatusCode, http.StatusCreated)
	}
	secondBody := readBody(t, second)

	if calls != 2 {
		t.Fatalf("handler invoked %d times, want 2 -- a row past the TTL must not be replayed", calls)
	}
	if secondBody["n"] == firstBody["n"] {
		t.Fatalf("second call body %v matches the expired first call's body %v -- expected a fresh execution", secondBody, firstBody)
	}

	var createdAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT created_at FROM idempotency_keys WHERE key = 'ttl-key'`,
	).Scan(&createdAt); err != nil {
		t.Fatalf("query refreshed row: %v", err)
	}
	if time.Since(createdAt) > time.Minute {
		t.Fatalf("created_at = %v, want refreshed to roughly now", createdAt)
	}
}

// TestWrap_RunsHandlerAgainWithNoKey proves requests with no
// Idempotency-Key header always execute normally.
func TestWrap_RunsHandlerAgainWithNoKey(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "no-key-owner")
	var calls int
	srv := newIdempotencyServer(db, "no-key-owner", &calls, http.StatusCreated)
	defer srv.Close()

	_ = postWidget(t, srv, practiceID, "").Body.Close()
	_ = postWidget(t, srv, practiceID, "").Body.Close()

	if calls != 2 {
		t.Fatalf("handler invoked %d times, want 2 -- no key means no idempotency protection", calls)
	}
}

// TestWrap_RunsHandlerAgainWithDifferentKey proves a different key is
// treated as an independent request, not a replay.
func TestWrap_RunsHandlerAgainWithDifferentKey(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "diff-key-owner")
	var calls int
	srv := newIdempotencyServer(db, "diff-key-owner", &calls, http.StatusCreated)
	defer srv.Close()

	_ = postWidget(t, srv, practiceID, "key-a").Body.Close()
	_ = postWidget(t, srv, practiceID, "key-b").Body.Close()

	if calls != 2 {
		t.Fatalf("handler invoked %d times, want 2 -- different keys must not replay each other", calls)
	}
}

// TestWrap_DoesNotPersistServerErrorResponse proves a 5xx response is not
// cached: a caller retrying after a transient failure must reach the
// handler again, not get the failure replayed forever.
func TestWrap_DoesNotPersistServerErrorResponse(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "server-error-owner")
	var calls int
	srv := newIdempotencyServer(db, "server-error-owner", &calls, http.StatusInternalServerError)
	defer srv.Close()

	_ = postWidget(t, srv, practiceID, "error-key").Body.Close()
	_ = postWidget(t, srv, practiceID, "error-key").Body.Close()

	if calls != 2 {
		t.Fatalf("handler invoked %d times, want 2 -- a 5xx response must not be replayed", calls)
	}
}

// TestWrap_DefaultsToStatusOKWhenHandlerOmitsWriteHeader proves the
// recorder captures the implicit 200 net/http applies when a handler calls
// Write without ever calling WriteHeader, so a replay still returns the
// correct status rather than 0.
func TestWrap_DefaultsToStatusOKWhenHandlerOmitsWriteHeader(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "no-writeheader-owner")

	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/widgets",
		staffauth.Middleware(fakeVerifier{uid: "no-writeheader-owner"}, db.App)(idempotency.Wrap(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"n":1}`))
			},
		))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	first := postWidget(t, srv, practiceID, "no-writeheader-key")
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first call status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	replayed := postWidget(t, srv, practiceID, "no-writeheader-key")
	defer replayed.Body.Close()
	if replayed.StatusCode != http.StatusOK {
		t.Fatalf("replayed status = %d, want %d", replayed.StatusCode, http.StatusOK)
	}
}

// TestWrap_TreatsOversizedKeyAsAbsent proves a key over maxKeyLength is
// treated the same as no key: the request still succeeds, just without
// idempotency protection.
func TestWrap_TreatsOversizedKeyAsAbsent(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "oversized-key-owner")
	var calls int
	srv := newIdempotencyServer(db, "oversized-key-owner", &calls, http.StatusCreated)
	defer srv.Close()

	oversized := strings.Repeat("k", 256)
	_ = postWidget(t, srv, practiceID, oversized).Body.Close()
	_ = postWidget(t, srv, practiceID, oversized).Body.Close()

	if calls != 2 {
		t.Fatalf("handler invoked %d times, want 2 -- an oversized key must not be treated as a real key", calls)
	}
}
