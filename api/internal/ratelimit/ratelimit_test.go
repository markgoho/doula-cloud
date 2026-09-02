package ratelimit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/testdb"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func ipRule(t *testing.T, db *testdb.DB, endpoint string, maxRequests int, window time.Duration) *httptest.Server {
	t.Helper()
	handler := ratelimit.Wrap(db.App, endpoint, []ratelimit.Rule{ratelimit.IPRule(maxRequests, window)})(okHandler())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server) *http.Response {
	t.Helper()
	return getURL(t, srv.URL)
}

func getURL(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url) //nolint:gosec,noctx // test-only fixed-format URL, not user input
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// TestWrap_UnderLimitPasses proves every request up to and including Max
// within a window reaches the wrapped handler.
func TestWrap_UnderLimitPasses(t *testing.T) {
	db := testdb.New(t)
	srv := ipRule(t, db, "test_under", 3, time.Hour)

	for i := 1; i <= 3; i++ {
		resp := get(t, srv)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, resp.StatusCode)
		}
	}
}

// TestWrap_OverLimitRefuses proves the request past Max is refused with
// docs/api-design.md's structured error body and a Retry-After header,
// and that a further request restates the refusal rather than passing.
func TestWrap_OverLimitRefuses(t *testing.T) {
	db := testdb.New(t)
	srv := ipRule(t, db, "test_over", 2, time.Hour)

	for i := 1; i <= 2; i++ {
		resp := get(t, srv)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, resp.StatusCode)
		}
	}

	resp := get(t, srv)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Fatal("Retry-After header missing")
	}
	if resp.Header.Get("RateLimit-Limit") != "2" {
		t.Fatalf("RateLimit-Limit = %q, want 2", resp.Header.Get("RateLimit-Limit"))
	}

	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "RATE_LIMITED" {
		t.Fatalf("code = %q, want RATE_LIMITED", body.Code)
	}
	if body.Message == "" {
		t.Fatal("message empty")
	}

	// A refusal is recorded, so repeated refusals against one address can
	// be seen after the fact.
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM rate_limit_refusals WHERE endpoint = 'test_over' AND dimension = 'ip'`,
	).Scan(&count); err != nil {
		t.Fatalf("count refusals: %v", err)
	}
	if count != 1 {
		t.Fatalf("refusal rows = %d, want 1", count)
	}
}

// TestWrap_SuccessHeadersReportTheTightestRule proves a passing request
// still carries docs/api-design.md section 6's informational headers,
// reporting whichever rule is closest to its own cap.
func TestWrap_SuccessHeadersReportTheTightestRule(t *testing.T) {
	db := testdb.New(t)
	handler := ratelimit.Wrap(db.App, "test_headers", []ratelimit.Rule{
		ratelimit.IPRule(10, time.Hour),
		ratelimit.PathValueRule("id", 3, time.Hour),
	})(okHandler())
	mux := http.NewServeMux()
	mux.Handle("GET /widgets/{id}", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp := getURL(t, srv.URL+"/widgets/w1")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("RateLimit-Limit") != "3" {
		t.Fatalf("RateLimit-Limit = %q, want 3 (the tighter of the two rules)", resp.Header.Get("RateLimit-Limit"))
	}
	if resp.Header.Get("RateLimit-Remaining") != "2" {
		t.Fatalf("RateLimit-Remaining = %q, want 2", resp.Header.Get("RateLimit-Remaining"))
	}
}

// TestWrap_WindowRollsOff proves a bucket past its own window resets
// rather than staying refused forever.
func TestWrap_WindowRollsOff(t *testing.T) {
	db := testdb.New(t)
	srv := ipRule(t, db, "test_rollover", 1, time.Hour)

	first := get(t, srv)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", first.StatusCode)
	}

	blocked := get(t, srv)
	_ = blocked.Body.Close()
	if blocked.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second request within window: status = %d, want 429", blocked.StatusCode)
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE rate_limit_buckets SET window_start = now() - interval '2 hours' WHERE key = 'test_rollover:ip:127.0.0.1'`,
	); err != nil {
		t.Fatalf("age out bucket: %v", err)
	}

	after := get(t, srv)
	_ = after.Body.Close()
	if after.StatusCode != http.StatusOK {
		t.Fatalf("request after window rollover: status = %d, want 200", after.StatusCode)
	}
}

// TestWrap_DifferentSubjectsDoNotShareACounter proves two different keys
// on the same Rule are counted independently -- refusing one IP must not
// refuse another.
func TestWrap_DifferentSubjectsDoNotShareACounter(t *testing.T) {
	db := testdb.New(t)
	handler := ratelimit.Wrap(db.App, "test_subjects", []ratelimit.Rule{ratelimit.IPRule(1, time.Hour)})(okHandler())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req1 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.1:1"
	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.2:1"

	rec1a := httptest.NewRecorder()
	handler.ServeHTTP(rec1a, req1)
	if rec1a.Code != http.StatusOK {
		t.Fatalf("subject 1, request 1: status = %d, want 200", rec1a.Code)
	}

	rec2a := httptest.NewRecorder()
	handler.ServeHTTP(rec2a, req2)
	if rec2a.Code != http.StatusOK {
		t.Fatalf("subject 2, request 1: status = %d, want 200 (must not share subject 1's counter)", rec2a.Code)
	}

	rec1b := httptest.NewRecorder()
	handler.ServeHTTP(rec1b, req1)
	if rec1b.Code != http.StatusTooManyRequests {
		t.Fatalf("subject 1, request 2: status = %d, want 429", rec1b.Code)
	}
}

// TestWrap_MultipleRulesEachEnforced proves a second Rule still refuses
// even when the first Rule's key is fresh on every request -- neither
// dimension alone can be evaded by varying the other.
func TestWrap_MultipleRulesEachEnforced(t *testing.T) {
	db := testdb.New(t)
	calls := 0
	freshEveryTime := ratelimit.Rule{
		Dimension: "fresh",
		Key: func(_ *http.Request) (string, bool) {
			calls++
			return http.StatusText(calls), true // a distinct key every call
		},
		Max:    1000,
		Window: time.Hour,
	}
	handler := ratelimit.Wrap(db.App, "test_multi", []ratelimit.Rule{freshEveryTime, ratelimit.IPRule(1, time.Hour)})(okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.9:1"

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("request 1: status = %d, want 200", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("request 2: status = %d, want 429 from the IP rule despite the fresh-key rule never repeating", rec2.Code)
	}
}

// TestBearerTokenRule_KeysByTokenDigest proves BearerTokenRule counts a
// presented Bearer token, independently per token, and that the same
// token replayed past Max is refused.
func TestBearerTokenRule_KeysByTokenDigest(t *testing.T) {
	db := testdb.New(t)
	handler := ratelimit.Wrap(db.App, "test_bearer", []ratelimit.Rule{ratelimit.BearerTokenRule(1, time.Hour)})(okHandler())

	reqA1 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	reqA1.Header.Set("Authorization", "Bearer token-a")
	recA1 := httptest.NewRecorder()
	handler.ServeHTTP(recA1, reqA1)
	if recA1.Code != http.StatusOK {
		t.Fatalf("token-a, request 1: status = %d, want 200", recA1.Code)
	}

	reqB1 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	reqB1.Header.Set("Authorization", "Bearer token-b")
	recB1 := httptest.NewRecorder()
	handler.ServeHTTP(recB1, reqB1)
	if recB1.Code != http.StatusOK {
		t.Fatalf("token-b, request 1: status = %d, want 200 (must not share token-a's counter)", recB1.Code)
	}

	reqA2 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	reqA2.Header.Set("Authorization", "Bearer token-a")
	recA2 := httptest.NewRecorder()
	handler.ServeHTTP(recA2, reqA2)
	if recA2.Code != http.StatusTooManyRequests {
		t.Fatalf("token-a, request 2: status = %d, want 429", recA2.Code)
	}
}

// TestPathValueRule_KeysByNamedPathParameter proves PathValueRule counts
// by the named path parameter, independently per value -- the pre-account
// Offer routes' "subject" dimension.
func TestPathValueRule_KeysByNamedPathParameter(t *testing.T) {
	db := testdb.New(t)
	mux := http.NewServeMux()
	mux.Handle("GET /offers/{offerId}",
		ratelimit.Wrap(db.App, "test_offer", []ratelimit.Rule{ratelimit.PathValueRule("offerId", 1, time.Hour)})(okHandler()))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	first := getURL(t, srv.URL+"/offers/offer-a")
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("offer-a, request 1: status = %d, want 200", first.StatusCode)
	}

	otherOffer := getURL(t, srv.URL+"/offers/offer-b")
	_ = otherOffer.Body.Close()
	if otherOffer.StatusCode != http.StatusOK {
		t.Fatalf("offer-b, request 1: status = %d, want 200 (must not share offer-a's counter)", otherOffer.StatusCode)
	}

	second := getURL(t, srv.URL+"/offers/offer-a")
	_ = second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("offer-a, request 2: status = %d, want 429", second.StatusCode)
	}
}

// TestWrap_SkippedRuleIsNotCounted proves a Rule whose Key reports ok=false
// is skipped rather than counted against an empty key -- a request with no
// Bearer token must not spend BearerTokenRule's budget.
func TestWrap_SkippedRuleIsNotCounted(t *testing.T) {
	db := testdb.New(t)
	handler := ratelimit.Wrap(db.App, "test_skip", []ratelimit.Rule{ratelimit.BearerTokenRule(1, time.Hour)})(okHandler())

	for i := 1; i <= 5; i++ {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d with no Bearer token: status = %d, want 200 (rule should be skipped, not counted)", i, rec.Code)
		}
	}
}
