// Package practicetimezone owns a Practice's own timezone (#953,
// practices.timezone): the one zone every calendar-day comparison in that
// Practice's data happens in, per ADR-0036.
//
// It is a package of its own rather than a file inside visit because two
// unrelated features have to reach the same zone. visit.DeriveType was
// the first reader, and the guard that refuses a manual Payment dated in
// the future (#1167) is the second -- a different package entirely, which
// would otherwise have to move the reader before it could use it. Load is
// that shared seam; nothing about it knows what the caller is comparing.
package practicetimezone

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	// Embeds the IANA database in the binary, as the fallback
	// time.LoadLocation reaches for when the host has no zoneinfo of its
	// own. gcr.io/distroless/static-debian12 does ship
	// /usr/share/zoneinfo today (checked by exporting the image and
	// listing it), so this is not repairing a broken image -- it is
	// refusing to depend on a base image's contents for an answer the
	// product gets wrong silently. The zone data then travels with the
	// code that reads it, which is why the import sits beside the read
	// rather than in main.
	_ "time/tzdata"
)

// MsgZoneNotRecognized is what a person reads when the zone she sent is
// not one the IANA database names -- docs/api-design.md section 7 rule 4:
// it starts with the field's own noun, says what to do, and avoids the
// four words apierr's TestDetailsWording gates on. The settings screen
// and the signup form both offer a list, so the only way to reach this
// is a hand-built request or a browser reporting a zone the database
// does not carry.
const MsgZoneNotRecognized = "Timezone must be a zone name from the IANA database, such as America/New_York"

// ErrZoneNotRecognized is what Parse reports for a name the IANA database
// does not carry, or for one of the two names time.LoadLocation answers
// without consulting it at all.
var ErrZoneNotRecognized = errors.New("practicetimezone: not an IANA zone name")

// Parse turns a submitted zone name into the location it names, refusing
// anything that is not a real IANA zone.
//
// time.LoadLocation answers two names out of thin air rather than out of
// the database, and both are refused here. "" is UTC, so an omitted field
// would otherwise be accepted as a deliberate choice of UTC -- and a
// Practice that meant to say nothing would silently get a zone no US
// Practice works in. "Local" is whichever zone the process happens to
// run in, which is the server's fact, not the Practice's: the same stored
// string would mean a different day depending on where the binary was
// deployed, which is precisely what ADR-0036 exists to stop.
func Parse(name string) (*time.Location, error) {
	if name == "" || name == "Local" {
		return nil, fmt.Errorf("%w: %q", ErrZoneNotRecognized, name)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrZoneNotRecognized, name, err)
	}
	return loc, nil
}

// Load reads practiceID's own zone and loads it. One read per request
// rather than one per row: time.LoadLocation reparses zoneinfo on every
// call, and a Practice has exactly one zone for the whole page.
//
// practices carries no row-level security of its own -- it is the table
// every tenant-scoped policy compares against -- so this reads by id on
// the request's own tx, with the caller's practice id, never one off the
// request body.
//
// A zone name that will not load is an error rather than a fall back to
// UTC. Falling back would answer a calendar-day question with the wrong
// day and say nothing about it, which is exactly the failure #953 exists
// to end.
func Load(ctx context.Context, tx *sql.Tx, practiceID string) (*time.Location, error) {
	name, err := readName(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	loc, err := Parse(name)
	if err != nil {
		return nil, fmt.Errorf("practicetimezone: load practice timezone: %w", err)
	}
	return loc, nil
}

// readName is the raw column read Load and the handlers share.
func readName(ctx context.Context, tx *sql.Tx, practiceID string) (string, error) {
	var name string
	if err := tx.QueryRowContext(ctx,
		`SELECT timezone FROM practices WHERE id = $1`, practiceID,
	).Scan(&name); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("practicetimezone: read practice timezone: %w", err)
	}
	return name, nil
}
