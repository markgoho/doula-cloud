package pagecursor_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/pagecursor"
)

func TestTrim(t *testing.T) {
	page, more := pagecursor.Trim([]int{1, 2, 3}, 2)
	if !more || len(page) != 2 {
		t.Fatalf("Trim(3 items, 2) = (%v, %v), want 2 items and more", page, more)
	}
	page, more = pagecursor.Trim([]int{1, 2}, 2)
	if more || len(page) != 2 {
		t.Fatalf("Trim(2 items, 2) = (%v, %v), want 2 items and no more", page, more)
	}
}

func TestCut(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	type row struct {
		at time.Time
		id string
	}
	key := func(r row) (time.Time, string) { return r.at, r.id }

	page, next, more := pagecursor.Cut([]row{{at, "a"}, {at, "b"}, {at, "c"}}, 2, key)
	if !more || len(page) != 2 || next == nil {
		t.Fatalf("Cut(3 rows, 2) = (%v, %v, %v), want 2 rows, a cursor and more", page, next, more)
	}
	if want := pagecursor.Encode(at, "b"); *next != want {
		t.Fatalf("cursor = %q, want the second row's %q", *next, want)
	}

	page, next, more = pagecursor.Cut([]row{{at, "a"}, {at, "b"}}, 2, key)
	if more || len(page) != 2 || next != nil {
		t.Fatalf("Cut(2 rows, 2) = (%v, %v, %v), want 2 rows, no cursor and no more", page, next, more)
	}
}

func TestFetch(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	type row struct {
		at time.Time
		id string
	}
	key := func(r row) (time.Time, string) { return r.at, r.id }
	rows := []row{{at, "a"}, {at, "b"}, {at, "c"}}
	fetching := func(err error) func(*pagecursor.Cursor) ([]row, error) {
		return func(*pagecursor.Cursor) ([]row, error) { return rows, err }
	}
	fetchRows := fetching(nil)

	t.Run("a page past size carries the cursor and what follows it", func(t *testing.T) {
		rec := httptest.NewRecorder()
		page, ok := pagecursor.Fetch(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), "bad cursor", 2, fetchRows, key)
		if !ok || len(page.Items) != 2 || !page.HasMore || page.Next == nil {
			t.Fatalf("Fetch = (%+v, %v), want 2 rows with a cursor", page, ok)
		}
	})

	t.Run("the decoded cursor reaches fetch", func(t *testing.T) {
		var got *pagecursor.Cursor
		target := "/?cursor=" + pagecursor.Encode(at, "a")
		_, ok := pagecursor.Fetch(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil), "bad cursor", 2,
			func(after *pagecursor.Cursor) ([]row, error) { got = after; return nil, nil }, key)
		if !ok || got == nil || got.ID != "a" {
			t.Fatalf("fetch saw %+v (ok %v), want the decoded cursor", got, ok)
		}
	})

	t.Run("a malformed cursor is a 400 with the caller's message", func(t *testing.T) {
		rec := httptest.NewRecorder()
		_, ok := pagecursor.Fetch(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?cursor=%21", nil), "bad cursor", 2, fetchRows, key)
		if ok || rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "bad cursor") {
			t.Fatalf("got ok=%v status=%d body=%s, want a 400 naming bad cursor", ok, rec.Code, rec.Body.String())
		}
	})

	t.Run("a fetch error is a 500", func(t *testing.T) {
		rec := httptest.NewRecorder()
		_, ok := pagecursor.Fetch(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), "bad cursor", 2,
			fetching(errors.New("db down")), key)
		if ok || rec.Code != http.StatusInternalServerError {
			t.Fatalf("got ok=%v status=%d, want a 500", ok, rec.Code)
		}
	})
}

func TestFromQuery(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	good := pagecursor.Encode(at, "row-1")

	t.Run("absent cursor is the first page", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)
		after, ok := pagecursor.FromQuery(w, r, pagecursor.Decode, "invalid cursor")
		if !ok || after != nil {
			t.Fatalf("got (%v, %v), want (nil, true)", after, ok)
		}
	})

	t.Run("valid cursor decodes", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x?cursor="+good, nil)
		after, ok := pagecursor.FromQuery(w, r, pagecursor.Decode, "invalid cursor")
		if !ok || after == nil || after.ID != "row-1" {
			t.Fatalf("got (%v, %v), want the decoded cursor", after, ok)
		}
	})

	t.Run("malformed cursor is a 400 with the caller's message", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x?cursor=!!!", nil)
		after, ok := pagecursor.FromQuery(w, r, pagecursor.Decode, "invalid cursor")
		if ok || after != nil {
			t.Fatalf("got (%v, %v), want (nil, false)", after, ok)
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}
