// Package stackswitch holds the two switches only the end-to-end stack
// (app/e2e/stack.ts) turns on, and the gate that keeps them off
// everywhere else (#1833):
//
//   - ClockVar makes the BFF's clock follow sim.offset_row, the offset a
//     simulation jump moves (#762), so clock.Now(ctx) and the database's
//     now() tell the same time after a jump.
//   - LegalVar appends versions to the shipped legal history
//     (api/internal/legal), so a spec can publish a material version the
//     shipped history does not have, without a fixture in legal.go.
//
// # The gate
//
// Either switch set, the BFF refuses to start unless both of the stack's
// own marks are present: FIREBASE_AUTH_EMULATOR_HOST is set, and the
// database carries the sim schema. Cloud Run sets neither. The first
// would send every sign-in to an emulator that does not exist there, and
// the second is installed by simclock only into a database that has
// never seen a migration (simclock.ErrAlreadyMigrated), which a deployed
// database never is. A switch set without both is a mistake, so it stops
// the process rather than being ignored.
//
// Unset, neither switch reads the database or the environment past its
// own name: the clock is clock.Real and the history is the shipped one.
package stackswitch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/legal"
)

const (
	// ClockVar is the switch that, set to "1", makes the clock follow
	// sim.offset_row.
	ClockVar = "E2E_CLOCK_FOLLOWS_SIM_OFFSET"
	// LegalVar is the switch holding a JSON list of versions to append to the shipped
	// history, each {"document", "effective", "material", "change"}.
	LegalVar = "E2E_LEGAL_VERSIONS"
	// MarkerVar is the stack's mark in the environment: the Firebase
	// Auth emulator's address.
	MarkerVar = "FIREBASE_AUTH_EMULATOR_HOST"
)

// ErrNotTheStack is returned when a switch is set in a process that does
// not carry both of the stack's marks.
var ErrNotTheStack = errors.New("stackswitch: an end-to-end switch is set outside the end-to-end stack")

// offsetQuery reads the offset in microseconds, the precision of a
// Postgres interval.
const offsetQuery = `SELECT (extract(epoch FROM delta) * 1000000)::bigint FROM sim.offset_row`

// Clock returns the clock the BFF reads: clock.Real with ClockVar unset,
// and on the stack, real time plus sim.offset_row's delta, read again on
// every call so a jump is followed at once.
func Clock(ctx context.Context, getenv func(string) string, db *sql.DB) (clock.Clock, error) {
	switch getenv(ClockVar) {
	case "":
		return clock.Real, nil
	case "1":
	default:
		return nil, fmt.Errorf("stackswitch: %s must be 1 or unset, not %q", ClockVar, getenv(ClockVar))
	}
	if err := requireStack(ctx, getenv, db); err != nil {
		return nil, err
	}
	// The clock outlives the call that built it, so it keeps ctx's values
	// and drops its cancellation.
	base := context.WithoutCancel(ctx)
	var last atomic.Int64
	read := func() (int64, error) {
		var micros int64
		if err := db.QueryRowContext(base, offsetQuery).Scan(&micros); err != nil {
			return 0, fmt.Errorf("stackswitch: read sim.offset_row: %w", err)
		}
		return micros, nil
	}
	micros, err := read()
	if err != nil {
		// coverage:ignore reason: requireStack has just read the same row's table, so a failure here needs the connection to break in between
		return nil, err
	}
	last.Store(micros)
	return func() time.Time {
		micros, err := read()
		if err != nil {
			// A Clock cannot return an error. The last offset read is
			// the nearest true answer, and the log says why it is old.
			log.Printf("stackswitch: read sim.offset_row: %v; keeping the last offset", err)
			micros = last.Load()
		} else {
			last.Store(micros)
		}
		return clock.Real().Add(time.Duration(micros) * time.Microsecond)
	}, nil
}

// appendedVersion is one version LegalVar adds. A price of a Credit and
// a price rise are left out on purpose: publishing a version that states
// a price asks Stripe for a Price at it, and the stack has no Stripe in
// CI.
type appendedVersion struct {
	Document  string `json:"document"`
	Effective string `json:"effective"`
	Material  bool   `json:"material"`
	Change    string `json:"change"`
}

// Legal returns the legal history the BFF reads: shipped with LegalVar
// unset, and on the stack, a copy of shipped with LegalVar's versions
// appended, each later than its document's last. shipped itself is never
// changed.
func Legal(ctx context.Context, getenv func(string) string, db *sql.DB, shipped []legal.Document) ([]legal.Document, error) {
	raw := getenv(LegalVar)
	if raw == "" {
		return shipped, nil
	}
	if err := requireStack(ctx, getenv, db); err != nil {
		return nil, err
	}
	versions, err := decodeVersions(raw)
	if err != nil {
		return nil, err
	}
	history := make([]legal.Document, len(shipped))
	for i, doc := range shipped {
		doc.Versions = append([]legal.Version(nil), doc.Versions...)
		history[i] = doc
	}
	for _, v := range versions {
		if err := appendVersion(history, v); err != nil {
			return nil, err
		}
	}
	return history, nil
}

// knownFields is appendedVersion's JSON names. A field outside them, such
// as a price, is refused rather than dropped: a version that silently
// lost a field would publish something other than what the spec wrote.
var knownFields = map[string]bool{"document": true, "effective": true, "material": true, "change": true}

// decodeVersions reads LegalVar's list. json.Unmarshal, not a Decoder
// with DisallowUnknownFields: a Decoder outside apierr is what
// apierr's TestNoDirectJSONUsage refuses, so the field names are
// checked by hand instead.
func decodeVersions(raw string) ([]appendedVersion, error) {
	var fields []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil, fmt.Errorf("stackswitch: %s: %w", LegalVar, err)
	}
	for _, f := range fields {
		for name := range f {
			if !knownFields[name] {
				return nil, fmt.Errorf("stackswitch: %s: unknown field %q", LegalVar, name)
			}
		}
	}
	var versions []appendedVersion
	if err := json.Unmarshal([]byte(raw), &versions); err != nil {
		return nil, fmt.Errorf("stackswitch: %s: %w", LegalVar, err)
	}
	return versions, nil
}

// appendVersion appends v to its document in history.
func appendVersion(history []legal.Document, v appendedVersion) error {
	if _, err := time.Parse(time.DateOnly, v.Effective); err != nil {
		return fmt.Errorf("stackswitch: %s: effective %q is not a date", LegalVar, v.Effective)
	}
	if v.Change == "" {
		return fmt.Errorf("stackswitch: %s: the version of %s on %s says no change", LegalVar, v.Document, v.Effective)
	}
	for i := range history {
		doc := &history[i]
		if doc.Path != "/"+v.Document {
			continue
		}
		if n := len(doc.Versions); n > 0 && v.Effective <= doc.Versions[n-1].Effective {
			return fmt.Errorf("stackswitch: %s: %s %s is not later than its last version", LegalVar, v.Document, v.Effective)
		}
		doc.Versions = append(doc.Versions, legal.Version{Effective: v.Effective, Material: v.Material, Change: v.Change})
		return nil
	}
	return fmt.Errorf("stackswitch: %s: no document %q", LegalVar, v.Document)
}

// requireStack is the gate: both of the stack's marks, or ErrNotTheStack.
func requireStack(ctx context.Context, getenv func(string) string, db *sql.DB) error {
	if getenv(MarkerVar) == "" {
		return fmt.Errorf("%w: %s is unset", ErrNotTheStack, MarkerVar)
	}
	var installed bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('sim.offset_row') IS NOT NULL`).Scan(&installed); err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests -- to_regclass answers NULL rather than failing on a missing table
		return fmt.Errorf("%w: read the sim schema: %w", ErrNotTheStack, err)
	}
	if !installed {
		return fmt.Errorf("%w: the database has no sim schema", ErrNotTheStack)
	}
	return nil
}
