package pagecursor_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"

	"doula-cloud/api/internal/pagecursor"
)

func TestKeyedRoundTrip(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	encoded := pagecursor.EncodeKeyed("latest", []string{"a", "b"}, id)

	got, err := pagecursor.DecodeKeyed(encoded, "latest")
	if err != nil {
		t.Fatalf("DecodeKeyed: %v", err)
	}
	if got.Order != "latest" || !slices.Equal(got.Keys, []string{"a", "b"}) || got.ID != id {
		t.Errorf("got %+v, want order latest, keys [a b], id %s", got, id)
	}
}

func TestKeyedRefusesAnotherOrder(t *testing.T) {
	encoded := pagecursor.EncodeKeyed("latest", []string{"a"}, "x")
	if _, err := pagecursor.DecodeKeyed(encoded, ""); err == nil {
		t.Error("a cursor minted for latest decoded under the default order")
	}
}

func TestKeyedRefusesMalformed(t *testing.T) {
	for name, input := range map[string]string{
		"not base64": "not!base64!",
		"not json":   "bm90LWpzb24",
		"no id":      "eyJvIjoiIiwiayI6W119",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := pagecursor.DecodeKeyed(input, ""); err == nil {
				t.Errorf("DecodeKeyed(%q) = nil error", input)
			}
		})
	}
}

func TestTimeKey(t *testing.T) {
	at := time.Date(2026, 8, 29, 14, 5, 6, 123456789, time.UTC)
	got, err := pagecursor.ParseTimeKey(pagecursor.TimeKey(at))
	if err != nil || !got.Equal(at) {
		t.Fatalf("round trip = %v, %v; want %v", got, err, at)
	}
	if _, err := pagecursor.ParseTimeKey("yesterday"); err == nil {
		t.Error("ParseTimeKey accepted a non-timestamp")
	}
}

func fetchKeyed(t *testing.T, query string, items []int) (*httptest.ResponseRecorder, pagecursor.Page[int], bool) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?"+query, nil)
	page, ok := pagecursor.FetchKeyed(rec, req, "invalid cursor", "name", 2,
		func(s string) (pagecursor.Keyed, error) { return pagecursor.DecodeKeyed(s, "name") },
		func(*pagecursor.Keyed) ([]int, error) { return items, nil },
		func(n int) ([]string, string) { return []string{"k"}, strconv.Itoa(n) })
	return rec, page, ok
}

func TestFetchKeyedMintsTheNextCursorForItsOrder(t *testing.T) {
	_, page, ok := fetchKeyed(t, "", []int{0, 1, 2})
	if !ok || !page.HasMore || page.Next == nil || len(page.Items) != 2 {
		t.Fatalf("page = %+v ok=%v; want 2 items, more, a cursor", page, ok)
	}
	got, err := pagecursor.DecodeKeyed(*page.Next, "name")
	if err != nil || got.ID != "1" {
		t.Fatalf("cursor = %+v, %v; want id 1", got, err)
	}
}

func TestFetchKeyedLastPageHasNoCursor(t *testing.T) {
	_, page, ok := fetchKeyed(t, "", []int{0})
	if !ok || page.HasMore || page.Next != nil {
		t.Fatalf("page = %+v ok=%v; want the last page", page, ok)
	}
}

func TestFetchKeyedRefusesACursorFromAnotherOrder(t *testing.T) {
	other := pagecursor.EncodeKeyed("latest", []string{"k"}, "x")
	rec, _, ok := fetchKeyed(t, "cursor="+other, []int{0})
	if ok || rec.Code != http.StatusBadRequest {
		t.Fatalf("ok=%v status=%d; want a refusal with 400", ok, rec.Code)
	}
}

func TestFetchKeyedPassesTheDecodedPositionToFetch(t *testing.T) {
	cursor := pagecursor.EncodeKeyed("name", []string{"k"}, "x")
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?cursor="+cursor, nil)
	var seen *pagecursor.Keyed
	_, ok := pagecursor.FetchKeyed(rec, req, "invalid cursor", "name", 2,
		func(s string) (pagecursor.Keyed, error) { return pagecursor.DecodeKeyed(s, "name") },
		func(after *pagecursor.Keyed) ([]int, error) { seen = after; return nil, nil },
		func(int) ([]string, string) { return nil, "" })
	if !ok || seen == nil || seen.ID != "x" {
		t.Fatalf("fetch saw %+v, ok=%v; want the cursor's position", seen, ok)
	}
}
