package visit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Embeds the IANA database in the binary, as the fallback
	// time.LoadLocation reaches for when the host has no zoneinfo of its
	// own. gcr.io/distroless/static-debian12 does ship
	// /usr/share/zoneinfo today (checked by exporting the image and
	// listing it), so this is not repairing a broken image -- it is
	// refusing to depend on a base image's contents for an answer the
	// product gets wrong silently. The zone data then travels with the
	// code that reads it, which is also why the import sits here rather
	// than in main.
	_ "time/tzdata"
)

// practiceLocation reads the Practice's own timezone (#953,
// practices.timezone) and loads it. One read per request rather than one
// per row: time.LoadLocation reparses zoneinfo on every call, and a
// Practice has exactly one zone for the whole page.
//
// practices carries no row-level security of its own -- it is the table
// every tenant-scoped policy compares against -- so this reads by id on
// the request's own tx the same way payments' Connect reads do, with the
// caller's practice id, never one off the request body.
//
// A zone name that will not load is an error rather than a fall back to
// UTC. Falling back would answer the derivation's question with the
// wrong day and say nothing about it, which is exactly the failure #953
// exists to end; until #1166 lets an Owner state a zone, the only way
// the column can hold an unloadable name is a hand-written row.
func practiceLocation(ctx context.Context, tx *sql.Tx, practiceID string) (*time.Location, error) {
	var name string
	if err := tx.QueryRowContext(ctx,
		`SELECT timezone FROM practices WHERE id = $1`, practiceID,
	).Scan(&name); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("visit: read practice timezone: %w", err)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("visit: load practice timezone %q: %w", name, err)
	}
	return loc, nil
}
