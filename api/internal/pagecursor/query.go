package pagecursor

import (
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
)

// FromQuery reads the request's "cursor" query parameter and decodes it
// with decode. An absent cursor is the first page: it returns (nil, true).
// A cursor decode refuses writes a 400 carrying message and returns
// ok=false, the caller's signal to return without doing anything else.
func FromQuery[T any](w http.ResponseWriter, r *http.Request, decode func(string) (T, error), message string) (after *T, ok bool) {
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		return nil, true
	}
	c, err := decode(raw)
	if err != nil {
		apierr.WriteError(w, message, http.StatusBadRequest)
		return nil, false
	}
	return &c, true
}

// Trim cuts a page fetched with one row past size back to size, and
// reports whether that extra row was there, which is how a list answers
// hasMore without a count.
func Trim[T any](items []T, size int) (page []T, hasMore bool) {
	if len(items) > size {
		return items[:size], true
	}
	return items, false
}

// Page is one page of a cursor-paged list: its rows, the cursor for the
// page after it when there is one, and whether there is.
type Page[T any] struct {
	Items   []T
	Next    *string
	HasMore bool
}

// Fetch is the whole read side of a cursor-paged list handler: it decodes
// the request's cursor with Decode (a malformed one is a 400 carrying
// message), has fetch read one page past size from after, and cuts it with
// Cut. A fetch error is a 500. Either refusal is written here and reported
// as ok=false, the caller's signal to return without doing anything else.
func Fetch[T any](
	w http.ResponseWriter,
	r *http.Request,
	message string,
	size int,
	fetch func(after *Cursor) ([]T, error),
	key func(T) (time.Time, string),
) (page Page[T], ok bool) {
	after, ok := FromQuery(w, r, Decode, message)
	if !ok {
		return Page[T]{}, false
	}
	items, err := fetch(after)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return Page[T]{}, false
	}
	items, next, hasMore := Cut(items, size, key)
	return Page[T]{Items: items, Next: next, HasMore: hasMore}, true
}

// Cut trims a page fetched with one row past size, as Trim does, and mints
// the cursor for the row the page ends on when that extra row was there.
// key reads the (timestamp, id) pair the list sorts on from a row.
func Cut[T any](items []T, size int, key func(T) (time.Time, string)) (page []T, next *string, hasMore bool) {
	page, hasMore = Trim(items, size)
	if !hasMore {
		return page, nil, false
	}
	at, id := key(page[len(page)-1])
	cursor := Encode(at, id)
	return page, &cursor, true
}
