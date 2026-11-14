package pagecursor

import (
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
)

// Keyed is a position in a list a reader can order more than one way
// (#2869): the order it was minted for, the sort key values the row it
// follows holds, and that row's id as the tiebreak. A cursor is only
// meaningful in its own order, so DecodeKeyed refuses one minted for
// another.
type Keyed struct {
	Order string   `json:"o"`
	Keys  []string `json:"k"`
	ID    string   `json:"id"`
}

// EncodeKeyed packs a position as opaque base64url JSON.
func EncodeKeyed(order string, keys []string, id string) string {
	raw, err := json.Marshal(Keyed{Order: order, Keys: keys, ID: id})
	if err != nil {
		// coverage:ignore reason: marshaling strings cannot fail
		panic(fmt.Sprintf("pagecursor: encode keyed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeKeyed reverses EncodeKeyed for a request in the given order,
// rejecting anything malformed and any cursor minted for a different
// order. Every caller turns the error into a 400.
func DecodeKeyed(s, order string) (Keyed, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Keyed{}, fmt.Errorf("pagecursor: decode: %w", err)
	}
	var c Keyed
	if err := json.Unmarshal(raw, &c); err != nil {
		return Keyed{}, fmt.Errorf("pagecursor: parse keyed cursor: %w", err)
	}
	if c.Order != order {
		return Keyed{}, errors.New("pagecursor: cursor belongs to another order")
	}
	if c.ID == "" {
		return Keyed{}, errors.New("pagecursor: cursor has no id")
	}
	return c, nil
}

// FetchKeyed is Fetch for a keyed cursor: decode turns the request's
// cursor into the position fetch reads from (it should start from
// DecodeKeyed with the request's order and refuse a cursor whose keys it
// cannot use), fetch reads one page past size, and the next cursor is
// minted for order from the key of the row the page ends on.
func FetchKeyed[T, C any](
	w http.ResponseWriter,
	r *http.Request,
	message, order string,
	size int,
	decode func(string) (C, error),
	fetch func(after *C) ([]T, error),
	key func(T) ([]string, string),
) (page Page[T], ok bool) {
	after, ok := FromQuery(w, r, decode, message)
	if !ok {
		return Page[T]{}, false
	}
	items, err := fetch(after)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return Page[T]{}, false
	}
	items, hasMore := Trim(items, size)
	if !hasMore {
		return Page[T]{Items: items}, true
	}
	keys, id := key(items[len(items)-1])
	next := EncodeKeyed(order, keys, id)
	return Page[T]{Items: items, Next: &next, HasMore: true}, true
}

// TimeKey is a timestamp as a sort key value.
func TimeKey(t time.Time) string { return t.Format(time.RFC3339Nano) }

// ParseTimeKey reads a TimeKey back.
func ParseTimeKey(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("pagecursor: parse time key: %w", err)
	}
	return t, nil
}
