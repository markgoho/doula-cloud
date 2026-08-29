package sitebuild_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/testdb"
)

// workerSecret is the X-Internal-Secret both endpoints check. One value
// for every test, because what is being tested is the check and not the
// string.
// #nosec G101 -- a test fixture, not a credential
const workerSecret = "site-worker-secret"

// fakeDispatcher records what the worker asked for, and can be told to
// fail, which is the whole surface Worker cares about.
type fakeDispatcher struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (f *fakeDispatcher) Dispatch(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.err
}

func (f *fakeDispatcher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeProber answers per slug, so one test can hold a live page and a
// broken one at once.
type fakeProber struct {
	mu       sync.Mutex
	results  map[string]sitebuild.PageProbe
	fallback sitebuild.PageProbe
	asked    []string
}

func (f *fakeProber) Probe(_ context.Context, slug string) sitebuild.PageProbe {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, slug)
	if r, ok := f.results[slug]; ok {
		return r
	}
	return f.fallback
}

// stubDoer answers every request with one canned response, and records
// the last request, so the real Prober and Dispatcher can be tested
// without a network.
type stubDoer struct {
	status int
	body   string
	err    error
	last   *http.Request
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	s.last = req
	if s.err != nil {
		return nil, s.err
	}
	rec := httptest.NewRecorder()
	rec.WriteHeader(s.status)
	_, _ = rec.WriteString(s.body)
	return rec.Result(), nil
}

// post calls an internal endpoint with the shared secret unless told
// otherwise.
func post(t *testing.T, h http.Handler, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(""))
	if secret != "" {
		req.Header.Set("X-Internal-Secret", secret)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// seedHostedPage creates a Practice with a published page under slug,
// and returns its id.
func seedHostedPage(t *testing.T, db *testdb.DB, name, slug string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ($1) RETURNING id`, name,
	).Scan(&id); err != nil {
		t.Fatalf("seed practice %q: %v", name, err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_websites
		     (practice_id, mode, service_description, cancellation_policy, slug, page_state)
		 VALUES ($1, 'hosted', 'Birth support.', 'Two weeks'' notice.', $2, 'pending')`,
		id, slug,
	); err != nil {
		t.Fatalf("seed hosted page %q: %v", slug, err)
	}
	return id
}

// queueRebuild inserts a pending rebuild aged by the given number of
// seconds, which is how a test moves the coalescing window without
// sleeping.
func queueRebuild(t *testing.T, db *testdb.DB, practiceID string, ageSeconds int) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO site_build_outbox (practice_id, created_at)
		 VALUES ($1, now() - make_interval(secs => $2))`, practiceID, ageSeconds,
	); err != nil {
		t.Fatalf("queue rebuild: %v", err)
	}
}

// outboxCounts reports how many rows sit in each status.
func outboxCounts(t *testing.T, db *testdb.DB) (pending, dispatched, dead int) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FILTER (WHERE status = 'pending'),
		        count(*) FILTER (WHERE status = 'dispatched'),
		        count(*) FILTER (WHERE status = 'dead_lettered')
		   FROM site_build_outbox`,
	).Scan(&pending, &dispatched, &dead); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return pending, dispatched, dead
}

// pageState reads back what a probe recorded for one Practice.
func pageState(t *testing.T, db *testdb.DB, practiceID string) (state, detail string, checked bool) {
	t.Helper()
	var s, d sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT page_state, page_check_detail, page_checked_at IS NOT NULL
		   FROM practice_websites WHERE practice_id = $1`, practiceID,
	).Scan(&s, &d, &checked); err != nil {
		t.Fatalf("read page state: %v", err)
	}
	return s.String, d.String, checked
}
