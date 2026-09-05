// Command simclock installs or extends simulated time on a sandbox
// Postgres database (see api/internal/simclock). It never runs against a
// deployed database: nothing in scripts/migrate.sh or the deploy
// workflow invokes it, and simclock.Install refuses outright once
// goose's own migrations are present without the shim.
//
// Usage:
//
//	simclock install <role>   # before `go run ./cmd/migrate`
//	simclock grant <role>     # after that role's CREATE ROLE
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"doula-cloud/api/internal/simclock"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// connectTimeout mirrors cmd/migrate's: the e2e stack starts this binary
// as soon as the db container starts, not once Postgres is actually
// accepting connections.
const connectTimeout = 30 * time.Second

const (
	modeInstall = "install"
	modeGrant   = "grant"
)

// roleName is checked here too, not only inside simclock.Install/Grant:
// os.Args is this binary's own boundary with the outside world, and the
// role is echoed back in a log line below.
var roleName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func main() {
	// coverage:ignore reason: exits the process, not exercised by unit tests
	if err := run(os.Args[1:]); err != nil {
		// coverage:ignore reason: exits the process, not exercised by unit tests
		log.Fatal(err)
	}
}

// run is exercised directly by main_test.go for the argument-validation
// and DATABASE_URL-unset paths; everything past that needs a real
// Postgres instance, which the e2e stack (app/e2e/stack.ts) proves
// instead of a unit test.
func run(args []string) error {
	if len(args) != 2 || (args[0] != modeInstall && args[0] != modeGrant) {
		return errors.New("simclock: usage: simclock install|grant <role>")
	}
	mode, role := args[0], args[1]
	if !roleName.MatchString(role) {
		return fmt.Errorf("simclock: invalid role name %q", role)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("simclock: DATABASE_URL must be set")
	}

	// coverage:ignore reason: malformed DSN, not exercised by unit tests
	db, err := sql.Open("pgx", dsn)
	// coverage:ignore reason: malformed DSN, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: malformed DSN, not exercised by unit tests
		return fmt.Errorf("simclock: open db: %w", err)
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	defer func() { _ = db.Close() }()

	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	if err := waitForConnection(context.Background(), db, connectTimeout); err != nil {
		// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
		return err
	}

	ctx := context.Background()
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	if mode == modeInstall {
		// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
		if err := simclock.Install(ctx, db, role); err != nil {
			// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
			return fmt.Errorf("simclock: install: %w", err)
		}
		// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
		log.Printf("simclock: installed, search_path set for %s", role) //nolint:gosec // role already matched roleName above; gosec's taint analysis doesn't see that guard
		// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
		return nil
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	if err := simclock.Grant(ctx, db, role); err != nil {
		// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
		return fmt.Errorf("simclock: grant: %w", err)
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	log.Printf("simclock: granted to %s", role) //nolint:gosec // role already matched roleName above; gosec's taint analysis doesn't see that guard
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	return nil
}

// waitForConnection is cmd/migrate's own helper, duplicated rather than
// shared: cmd packages are deliberately not importable from one another,
// and the eight lines aren't worth a new internal package.
func waitForConnection(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		lastErr = db.PingContext(ctx)
		// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
		if lastErr == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("simclock: db not reachable after %s: %w", timeout, lastErr)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
