package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/testdb"
)

// TestWrap_WindowEdgeOnTheRequestClock is #1992's edge for touch: with
// one request allowed per hour and the clock fixed by clock.Middleware,
// the window opened by the first request still refuses one microsecond
// before an hour has passed -- with a Retry-After counted from that same
// clock -- and has rolled off at exactly an hour. Only a window written
// and judged at clock.Now(ctx) can pass: the database's own now() never
// moves with the fixed clock.
func TestWrap_WindowEdgeOnTheRequestClock(t *testing.T) {
	const window = time.Hour
	openedAt := time.Now().Add(-3 * time.Hour).Truncate(time.Microsecond).UTC()
	cases := []struct {
		name           string
		secondAt       time.Time
		wantStatus     int
		wantRetryAfter string
	}{
		{"just-inside", openedAt.Add(window - time.Microsecond), http.StatusTooManyRequests, "1"},
		{"at-window", openedAt.Add(window), http.StatusOK, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := testdb.New(t)
			var now atomic.Pointer[time.Time]
			now.Store(&openedAt)
			handler := ratelimit.Wrap(db.App, "test_clock_edge", []ratelimit.Rule{ratelimit.IPRule(1, window)})(okHandler())
			srv := httptest.NewServer(clock.Middleware(func() time.Time { return *now.Load() })(handler))
			t.Cleanup(srv.Close)

			first := get(t, srv)
			_ = first.Body.Close()
			if first.StatusCode != http.StatusOK {
				t.Fatalf("first request: status = %d, want 200", first.StatusCode)
			}
			var windowStart time.Time
			if err := db.Admin.QueryRowContext(t.Context(),
				`SELECT window_start FROM rate_limit_buckets WHERE key = 'test_clock_edge:ip:127.0.0.1'`,
			).Scan(&windowStart); err != nil {
				t.Fatalf("read window_start: %v", err)
			}
			if !windowStart.Equal(openedAt) {
				t.Fatalf("window_start = %v, want the request clock %v", windowStart, openedAt)
			}

			now.Store(&c.secondAt)
			second := get(t, srv)
			_ = second.Body.Close()
			if second.StatusCode != c.wantStatus {
				t.Fatalf("second request: status = %d, want %d", second.StatusCode, c.wantStatus)
			}
			if got := second.Header.Get("Retry-After"); got != c.wantRetryAfter {
				t.Fatalf("Retry-After = %q, want %q", got, c.wantRetryAfter)
			}
		})
	}
}
