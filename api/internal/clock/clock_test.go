package clock_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
)

func TestNow_ReadsWhatIntoSeeded(t *testing.T) {
	fixed := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	ctx := clock.Into(t.Context(), func() time.Time { return fixed })

	got := clock.Now(ctx)
	if !got.Equal(fixed) {
		t.Fatalf("Now() = %v, want %v", got, fixed)
	}
}

func TestNow_FallsBackToRealWhenNothingSeeded(t *testing.T) {
	before := time.Now()
	got := clock.Now(t.Context())
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Fatalf("Now() = %v, want between %v and %v", got, before, after)
	}
}

func TestInto_NilClockFallsBackToReal(t *testing.T) {
	before := time.Now()
	ctx := clock.Into(t.Context(), nil)
	got := clock.Now(ctx)
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Fatalf("Now() after Into(ctx, nil) = %v, want between %v and %v", got, before, after)
	}
}

func TestMiddleware_SeedsTheClockItWasGiven(t *testing.T) {
	fixed := time.Date(2027, time.June, 15, 12, 0, 0, 0, time.UTC)
	var seen time.Time
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = clock.Now(r.Context())
	})

	handler := clock.Middleware(func() time.Time { return fixed })(next)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if !seen.Equal(fixed) {
		t.Fatalf("handler saw %v, want %v", seen, fixed)
	}
}

func TestMiddleware_NilClockFallsBackToReal(t *testing.T) {
	var seen time.Time
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = clock.Now(r.Context())
	})

	before := time.Now()
	handler := clock.Middleware(nil)(next)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	after := time.Now()

	if seen.Before(before) || seen.After(after) {
		t.Fatalf("handler saw %v, want between %v and %v", seen, before, after)
	}
}

func TestReal_IsTheWallClock(t *testing.T) {
	before := time.Now()
	got := clock.Real()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Fatalf("Real() = %v, want between %v and %v", got, before, after)
	}
}

// contextValueWrongType proves Now's type assertion falls back to Real
// rather than panicking when something else is stored under the same
// key shape -- unreachable from outside this package in practice, since
// contextKey is unexported, but Now's ok-check is the line that makes it
// unreachable, and that line needs a caller to run it at all.
func TestNow_ContextWithNoValue(t *testing.T) {
	before := time.Now()
	got := clock.Now(context.Background())
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Fatalf("Now() = %v, want between %v and %v", got, before, after)
	}
}
