package visit

import (
	"context"
	"database/sql"
	"time"

	"doula-cloud/api/internal/practicetimezone"
)

// practiceLocation reads the Practice's own timezone (#953,
// practices.timezone) and loads it. One read per request rather than one
// per row: time.LoadLocation reparses zoneinfo on every call, and a
// Practice has exactly one zone for the whole page.
//
// The reading, the tzdata embed and the refusal to fall back to UTC all
// moved to practicetimezone on #1166, because the guard that refuses a
// manual Payment dated in the future (#1167) needs the same zone from a
// package that cannot import this one. This stays as the name the
// derivation calls, so nothing about how a Visit is typed reads as
// though it owns the zone.
func practiceLocation(ctx context.Context, tx *sql.Tx, practiceID string) (*time.Location, error) {
	return practicetimezone.Load(ctx, tx, practiceID)
}
