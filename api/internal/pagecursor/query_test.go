package pagecursor_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/pagecursor"
)

func TestFromQuery(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	good := pagecursor.Encode(at, "row-1")

	t.Run("absent cursor is the first page", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/x", nil)
		after, ok := pagecursor.FromQuery(w, r, pagecursor.Decode, "invalid cursor")
		if !ok || after != nil {
			t.Fatalf("got (%v, %v), want (nil, true)", after, ok)
		}
	})

	t.Run("valid cursor decodes", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/x?cursor="+good, nil)
		after, ok := pagecursor.FromQuery(w, r, pagecursor.Decode, "invalid cursor")
		if !ok || after == nil || after.ID != "row-1" {
			t.Fatalf("got (%v, %v), want the decoded cursor", after, ok)
		}
	})

	t.Run("malformed cursor is a 400 with the caller's message", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/x?cursor=!!!", nil)
		after, ok := pagecursor.FromQuery(w, r, pagecursor.Decode, "invalid cursor")
		if ok || after != nil {
			t.Fatalf("got (%v, %v), want (nil, false)", after, ok)
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}
