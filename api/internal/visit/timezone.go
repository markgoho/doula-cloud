package visit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// The API image is gcr.io/distroless/static-debian12, which ships no
	// /usr/share/zoneinfo, so time.LoadLocation below would fail for
	// every zone name on Cloud Run while succeeding on any developer's
	// machine. This blank import embeds the IANA database in the binary.
	// It lives beside the one call that needs it rather than in main, so
	// the dependency travels with the code that has it.
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
