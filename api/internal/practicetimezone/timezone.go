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
	"fmt"
	"time"

	"doula-cloud/api/internal/ianazone"
)

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
	loc, err := ianazone.Parse(name)
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
