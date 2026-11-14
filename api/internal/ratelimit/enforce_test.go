package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/testdb"
)

// TestEnforce_CountsOnTheCallersTransaction proves a handler that already
// holds the request's one pool connection is limited through its own
// transaction, with no second connection asked for: App is capped at one
// connection, so a count taken on the pool would hang.
func TestEnforce_CountsOnTheCallersTransaction(t *testing.T) {
	db := testdb.New(t)
	rules := []ratelimit.Rule{ratelimit.IPRule(2, time.Hour)}

	var statuses []int
	for range 3 {
		tx, err := db.App.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		req.RemoteAddr = "203.0.113.9:1234"
		if ratelimit.Enforce(rec, req, tx, "test_enforce", rules) {
			rec.WriteHeader(http.StatusOK)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		statuses = append(statuses, rec.Code)
		if rec.Header().Get("RateLimit-Limit") != "2" {
			t.Fatalf("RateLimit-Limit = %q, want 2", rec.Header().Get("RateLimit-Limit"))
		}
	}

	if statuses[0] != http.StatusOK || statuses[1] != http.StatusOK || statuses[2] != http.StatusTooManyRequests {
		t.Fatalf("statuses = %v, want 200, 200, 429", statuses)
	}
}
