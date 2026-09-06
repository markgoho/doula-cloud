// Command simclock installs or extends simulated time on a sandbox
// Postgres database (see api/internal/simclock). It never runs against a
// deployed database: nothing in scripts/migrate.sh or the deploy
// workflow invokes it, and simclock.Install refuses outright once
// goose's own migrations are present without the shim.
//
// Usage:
//
//	simclock install <role>                      # before `go run ./cmd/migrate`
//	simclock grant <role>                        # after that role's CREATE ROLE
//	simclock allocate <client-id> <account-id>   # give a Client her Stripe Customer, on a test clock
//	simclock advance <duration>                  # move the offset row and every held clock together
//
// allocate and advance also need STRIPE_API_KEY, which is always a
// Sandbox key: test clocks do not exist in live mode.
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
	modeInstall  = "install"
	modeGrant    = "grant"
	modeAllocate = "allocate"
	modeAdvance  = "advance"
)

const usage = "simclock: usage: simclock install|grant <role> | allocate <client-id> <account-id> | advance <duration>"

// roleName is checked here too, not only inside simclock.Install/Grant:
// os.Args is this binary's own boundary with the outside world, and the
// role is echoed back in a log line below.
var roleName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// stripeAccountID is the shape of a Connect account id, checked here for
// the same reason roleName is: os.Args is this binary's boundary with the
// outside world, and the value is echoed back in an error below.
var stripeAccountID = regexp.MustCompile(`^acct_[A-Za-z0-9]+$`)

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
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case modeInstall, modeGrant:
		return runRoleMode(args)
	case modeAllocate:
		return runAllocate(args)
	case modeAdvance:
		return runAdvance(args)
	default:
		return errors.New(usage)
	}
}

// runRoleMode is install and grant: both take one role name and neither
// touches Stripe.
func runRoleMode(args []string) error {
	if len(args) != 2 {
		return errors.New(usage)
	}
	mode, role := args[0], args[1]
	if !roleName.MatchString(role) {
		return fmt.Errorf("simclock: invalid role name %q", role)
	}

	db, err := openDB()
	if err != nil {
		return err
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	defer func() { _ = db.Close() }()

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

// runAllocate gives one Client her Stripe Customer on one connected
// account, made against a test clock the run holds. It is safe to call
// again for the same pair: the second call reports the Customer the first
// one made and creates nothing.
func runAllocate(args []string) error {
	if len(args) != 3 {
		return errors.New(usage)
	}
	clientID, accountID := args[1], args[2]
	if !stripeAccountID.MatchString(accountID) {
		return fmt.Errorf("simclock: invalid stripe account id %q", accountID)
	}

	runner, db, err := newRunner()
	if err != nil {
		return err
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	defer func() { _ = db.Close() }()

	// coverage:ignore reason: requires a real DB connection and Stripe key, not exercised by unit tests
	customerID, created, err := runner.Allocate(context.Background(), clientID, accountID)
	// coverage:ignore reason: requires a real DB connection and Stripe key, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("simclock: allocate: %w", err)
	}
	// coverage:ignore reason: requires a real DB connection and Stripe key, not exercised by unit tests
	if created {
		// coverage:ignore reason: requires a real DB connection and Stripe key, not exercised by unit tests
		log.Printf("simclock: allocated customer %s", customerID) //nolint:gosec // the id came from Stripe's API, not from this binary's arguments
		// coverage:ignore reason: requires a real DB connection and Stripe key, not exercised by unit tests
		return nil
	}
	// coverage:ignore reason: requires a real DB connection and Stripe key, not exercised by unit tests
	log.Printf("simclock: customer %s already allocated", customerID) //nolint:gosec // the id came from this database, not from this binary's arguments
	// coverage:ignore reason: requires a real DB connection and Stripe key, not exercised by unit tests
	return nil
}

// runAdvance moves the offset row and every held test clock forward by
// the same duration, and does not return until every clock has landed.
// The duration is Go's own syntax -- 168h for a simulated week.
func runAdvance(args []string) error {
	if len(args) != 2 {
		return errors.New(usage)
	}
	delta, err := time.ParseDuration(args[1])
	if err != nil {
		return fmt.Errorf("simclock: invalid duration %q: %w", args[1], err)
	}

	runner, db, err := newRunner()
	if err != nil {
		return err
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	defer func() { _ = db.Close() }()

	// coverage:ignore reason: requires a real DB connection and Stripe key, not exercised by unit tests
	if err := runner.Advance(context.Background(), delta); err != nil {
		return fmt.Errorf("simclock: advance: %w", err)
	}
	// coverage:ignore reason: requires a real DB connection and Stripe key, not exercised by unit tests
	log.Printf("simclock: advanced by %s", delta) //nolint:gosec // a time.Duration parsed by time.ParseDuration, not free text
	// coverage:ignore reason: requires a real DB connection and Stripe key, not exercised by unit tests
	return nil
}

// newRunner builds the Runner allocate and advance share, and hands back
// the connection so the caller can close it.
func newRunner() (simclock.Runner, *sql.DB, error) {
	apiKey := os.Getenv("STRIPE_API_KEY")
	if apiKey == "" {
		return simclock.Runner{}, nil, errors.New("simclock: STRIPE_API_KEY must be set")
	}
	db, err := openDB()
	if err != nil {
		return simclock.Runner{}, nil, err
	}
	// coverage:ignore reason: reached only once a real database connection is open, not exercised by unit tests
	return simclock.Runner{DB: db, Stripe: simclock.NewStripeAPI(apiKey)}, db, nil
}

// openDB connects to DATABASE_URL and waits for Postgres to accept
// connections -- the e2e stack starts this binary as soon as the db
// container starts, not once it is actually listening.
func openDB() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, errors.New("simclock: DATABASE_URL must be set")
	}

	// coverage:ignore reason: malformed DSN, not exercised by unit tests
	db, err := sql.Open("pgx", dsn)
	// coverage:ignore reason: malformed DSN, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: malformed DSN, not exercised by unit tests
		return nil, fmt.Errorf("simclock: open db: %w", err)
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	if err := waitForConnection(context.Background(), db, connectTimeout); err != nil {
		// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
		_ = db.Close()
		// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
		return nil, err
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	return db, nil
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
