// Package simclock installs simulated time into a sandbox Postgres
// database: a sim schema, one offset row, and a sim.now() function that
// every unqualified now() call resolves to once a role's search_path
// names sim ahead of pg_catalog. See
// docs/research/simulated-clock-compression.md for the mechanism and
// why it must never reach a deployed database.
package simclock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
)

// ErrAlreadyMigrated is returned by Install when the target database
// already carries goose's migrations but was never installed with the
// shim. DEFAULT now() binds to a function at CREATE TABLE time, so
// installing after the fact would leave every existing default on real
// time -- silently wrong, not merely absent -- and there is no SQL fix
// that rebinds an existing default. A database in this state is also
// never the disposable sandbox this package is for: a deployed database
// always has its migrations applied, so this doubles as the guard
// against ever touching one.
var ErrAlreadyMigrated = errors.New("simclock: database already carries migrations without the shim installed")

// searchPath is shared by Install and Grant. public first, so every
// unqualified CREATE TABLE/FUNCTION a migration runs still lands in
// public; sim second, so an unqualified now() resolves there before
// falling through to the real pg_catalog.now() named explicitly last.
// Both schemas must be spelled out: pg_catalog is only searched last
// when a role's search_path names it -- the default, unwritten
// search_path searches it first, which is what makes the shim invisible
// until this is set.
const searchPath = "public, sim, pg_catalog"

const installSQL = `
CREATE SCHEMA sim;

CREATE TABLE sim.offset_row (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    delta interval NOT NULL DEFAULT '0'
);
INSERT INTO sim.offset_row (id) VALUES (true);

CREATE FUNCTION sim.now() RETURNS timestamptz
    LANGUAGE sql STABLE
    AS $$ SELECT pg_catalog.now() + delta FROM sim.offset_row $$;
`

// roleName guards against SQL injection through the role identifier:
// ALTER ROLE and GRANT have no placeholder for an identifier, and every
// caller in this codebase passes a hardcoded role name (app, app_e2e),
// never anything read off a request. Rejecting anything else keeps that
// true by construction rather than by convention.
var roleName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// Install creates the sim schema, its offset row and sim.now(), and
// points migrateRole's search_path at it. It must run before any
// migration is applied -- see ErrAlreadyMigrated -- so the caller owns
// sequencing this ahead of goose.
//
// Safe to call more than once against the same database: a database
// that already carries the shim (the ordinary case for a resumed run
// against a kept volume) is left untouched.
func Install(ctx context.Context, db *sql.DB, migrateRole string) error {
	if !roleName.MatchString(migrateRole) {
		return fmt.Errorf("simclock: invalid role name %q", migrateRole)
	}

	installed, err := schemaExists(ctx, db)
	// coverage:ignore reason: schemaExists only errors on a broken connection, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: schemaExists only errors on a broken connection, not exercised by unit tests
		return err
	}
	if !installed {
		migrated, err := hasMigrations(ctx, db)
		// coverage:ignore reason: hasMigrations only errors on a broken connection, not exercised by unit tests
		if err != nil {
			// coverage:ignore reason: hasMigrations only errors on a broken connection, not exercised by unit tests
			return err
		}
		if migrated {
			return ErrAlreadyMigrated
		}
		if _, err := db.ExecContext(ctx, installSQL); err != nil {
			return fmt.Errorf("simclock: install: %w", err)
		}
	}

	// Unconditionally, not only on a fresh install: a run resumed against
	// a kept volume already has the schema and skips the block above, and
	// its clock bookkeeping still has to exist. Every statement is
	// IF NOT EXISTS, so this is a no-op on a database that has them.
	if _, err := db.ExecContext(ctx, clockTablesSQL); err != nil {
		// coverage:ignore reason: DDL failure against a database that just accepted the schema, not exercised by unit tests
		return fmt.Errorf("simclock: install clock tables: %w", err)
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf("ALTER ROLE %s SET search_path = %s", migrateRole, searchPath)); err != nil {
		return fmt.Errorf("simclock: set search_path on %s: %w", migrateRole, err)
	}
	return nil
}

// Grant points role's search_path at sim, the same as Install does for
// the migrating role, and additionally grants the USAGE and SELECT
// sim.now() needs to run under a non-superuser role: it is plain SQL,
// not SECURITY DEFINER (docs/research/simulated-clock-compression.md,
// loose ends), so it executes with the caller's own privileges. Call it
// once the role exists -- the runtime login role (app_e2e in the e2e
// stack) is created by a step after migrations run, never before.
func Grant(ctx context.Context, db *sql.DB, role string) error {
	if !roleName.MatchString(role) {
		return fmt.Errorf("simclock: invalid role name %q", role)
	}

	statements := []string{
		fmt.Sprintf("ALTER ROLE %s SET search_path = %s", role, searchPath),
		fmt.Sprintf("GRANT USAGE ON SCHEMA sim TO %s", role),
		fmt.Sprintf("GRANT SELECT ON sim.offset_row TO %s", role),
	}
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("simclock: grant to %s: %w", role, err)
		}
	}
	return nil
}

func schemaExists(ctx context.Context, db *sql.DB) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'sim')").Scan(&exists)
	// coverage:ignore reason: requires a broken connection, not exercised by unit tests -- every unit test's role can read pg_namespace
	if err != nil {
		// coverage:ignore reason: requires a broken connection, not exercised by unit tests
		return false, fmt.Errorf("simclock: check for existing schema: %w", err)
	}
	return exists, nil
}

// hasMigrations reports whether goose has ever run against this
// database, by way of the tracking table it creates in public before
// applying a single migration file (api/cmd/migrate, api/internal/testdb
// both leave it at its default name and schema).
func hasMigrations(ctx context.Context, db *sql.DB) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, "SELECT to_regclass('public.goose_db_version') IS NOT NULL").Scan(&exists)
	// coverage:ignore reason: requires a broken connection, not exercised by unit tests -- to_regclass never itself errors
	if err != nil {
		// coverage:ignore reason: requires a broken connection, not exercised by unit tests
		return false, fmt.Errorf("simclock: check for goose's tracking table: %w", err)
	}
	return exists, nil
}
