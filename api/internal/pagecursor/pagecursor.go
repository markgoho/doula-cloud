// Package pagecursor is the opaque cursor docs/api-design.md section 4
// asks every paginated list to carry: a (created_at, id) pair packed as
// base64 so a caller can never construct one by hand and can never be
// tempted to read an offset out of it.
//
// It exists because three packages -- offer, message and payments -- had
// each independently written the same thirty lines, differing only in the
// package name inside the error strings and the names of the two struct
// fields. A fourth copy was about to land with the work state history
// reader (#459), which is the point at which the shape stops being a
// coincidence.
package pagecursor

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Cursor is a position in a list ordered by (At, ID) -- the timestamp the
// list sorts on, and the row id that breaks a tie between two rows sharing
// it. Both are needed: a bare timestamp skips or repeats rows written in
// the same microsecond, which is exactly what a cursor exists to prevent.
type Cursor struct {
	At time.Time
	ID string
}

// Encode packs a position as opaque base64. The separator is "|" because
// neither an RFC 3339 timestamp nor a UUID can contain one, so SplitN
// below can never split in the wrong place.
func Encode(at time.Time, id string) string {
	return base64.URLEncoding.EncodeToString([]byte(at.Format(time.RFC3339Nano) + "|" + id))
}

// Decode reverses Encode, rejecting anything malformed rather than letting
// a bad cursor silently return the wrong page. Every caller turns an error
// here into a 400: a cursor is either one we minted or it is not a request
// we can answer.
func Decode(s string) (Cursor, error) {
	raw, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("pagecursor: decode: %w", err)
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return Cursor{}, errors.New("pagecursor: malformed cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return Cursor{}, fmt.Errorf("pagecursor: parse timestamp: %w", err)
	}
	return Cursor{At: at, ID: parts[1]}, nil
}
