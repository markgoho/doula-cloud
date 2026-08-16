// Package testdb starts one real, disposable Postgres container per test
// *process* (via testcontainers-go), applies the goose migrations once
// into a template database, and hands each test a fresh database cloned
// from that template -- a file copy, not a migration replay. It's
// container-engine-agnostic: testcontainers-go falls back to Docker's
// default socket with no setup (what CI uses), or reads DOCKER_HOST from
// the environment to target a Podman socket instead (see docs/testing.md
// for local dev) -- either way, no code change here.
package testdb

import (
	"context"
	"database/sql"
	"net/url"
	"testing"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	appUser     = "app_test"
	appPassword = "app_test"
)

// DB holds two connections to the same disposable Postgres instance.
// Admin is a superuser connection for migrations and fixture setup --
// Postgres superusers always bypass Row-Level Security, so Admin never
// observes RLS in effect. App is a low-privilege connection using the
// same app_runtime role the running application connects as; it is the
// connection RLS policies actually apply to, and the one tests should use
// to prove tenant isolation.
type DB struct {
	Admin *sql.DB
	App   *sql.DB
}

// New clones a fresh database from the test process's shared, migrated
// template (starting the container and template on first call, via
// sync.Once) and returns Admin and App connections to the clone. The
// clone and both connections are torn down automatically via t.Cleanup;
// the container itself lives for the whole test process -- see Main.
func New(t *testing.T) *DB {
	t.Helper()
	ctx := context.Background()

	setupOnce.Do(func() { errSetup = setup(ctx) })
	// coverage:ignore reason: one-time setup failure, not exercised by the happy-path test
	if errSetup != nil {
		t.Fatalf("testdb: setup: %v", errSetup)
	}

	name := cloneDatabase(ctx, t)
	// Registered before Admin/App are opened below so it runs *last*
	// (t.Cleanup is LIFO), after both connections are closed -- DROP
	// DATABASE fails while either is still open.
	t.Cleanup(func() {
		admin, err := sql.Open("pgx", maintenanceDSN)
		// coverage:ignore reason: driver open failure, not exercised by the happy-path test
		if err != nil {
			t.Errorf("testdb: open admin db for drop: %v", err)
			return
		}
		defer func() { _ = admin.Close() }()
		if _, err := admin.ExecContext(context.Background(), "DROP DATABASE "+name); err != nil {
			// coverage:ignore reason: drop failure, not exercised by the happy-path test
			t.Errorf("testdb: drop database %s: %v", name, err)
		}
	})

	u, err := url.Parse(maintenanceDSN)
	// coverage:ignore reason: malformed maintenance DSN, not exercised by the happy-path test
	if err != nil {
		t.Fatalf("testdb: parse dsn: %v", err)
	}
	u.Path = "/" + name

	db, err := sql.Open("pgx", u.String())
	// coverage:ignore reason: driver open failure, not exercised by the happy-path test
	if err != nil {
		t.Fatalf("testdb: open db: %v", err)
	}
	t.Cleanup(func() {
		// coverage:ignore reason: db close failure, not exercised by the happy-path test
		if err := db.Close(); err != nil {
			t.Errorf("testdb: close db: %v", err)
		}
	})

	appDB := newAppDB(t, u.String())

	return &DB{Admin: db, App: appDB}
}

// newAppDB opens a connection to the clone as app_test, the shared
// LOGIN role in the app_runtime group (created once in setup, since
// roles are cluster-global -- a per-test CREATE ROLE would fail with
// "role already exists" on the second test in the package). This is the
// connection RLS policies scoped to app_runtime actually apply to.
func newAppDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	u, err := url.Parse(dsn)
	// coverage:ignore reason: malformed DSN, not exercised by the happy-path test
	if err != nil {
		t.Fatalf("testdb: parse dsn: %v", err)
	}
	u.User = url.UserPassword(appUser, appPassword)

	appDB, err := sql.Open("pgx", u.String())
	// coverage:ignore reason: driver open failure, not exercised by the happy-path test
	if err != nil {
		t.Fatalf("testdb: open app db: %v", err)
	}
	t.Cleanup(func() {
		// coverage:ignore reason: db close failure, not exercised by the happy-path test
		if err := appDB.Close(); err != nil {
			t.Errorf("testdb: close app db: %v", err)
		}
	})

	return appDB
}
