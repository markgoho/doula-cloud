// Package testdb spins up a real, disposable Postgres instance (via
// testcontainers-go) for Go HTTP tests, applies the goose migrations, and
// hands back a ready-to-use *sql.DB. It targets Podman: testcontainers-go
// reads DOCKER_HOST from the environment, so pointing that at a Podman
// socket (see docs/testing.md) is enough to run against Podman with no
// code change here.
package testdb

import (
	"context"
	"database/sql"
	"net/url"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"doula-cloud/api/db/migrations"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"
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

// New starts a Postgres container, applies all goose migrations against
// it, and returns Admin and App connections to it. The container and both
// connections are torn down automatically via t.Cleanup.
func New(t *testing.T) *DB {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx, "docker.io/library/postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testdb"),
		postgres.WithPassword("testdb"),
		postgres.BasicWaitStrategies(),
	)
	// coverage:ignore reason: container startup failure, not exercised by the happy-path test
	if err != nil {
		t.Fatalf("testdb: start postgres container: %v", err)
	}
	t.Cleanup(func() {
		// coverage:ignore reason: container teardown failure, not exercised by the happy-path test
		if err := container.Terminate(context.Background()); err != nil {
			t.Errorf("testdb: terminate postgres container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	// coverage:ignore reason: connection string failure, not exercised by the happy-path test
	if err != nil {
		t.Fatalf("testdb: connection string: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
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

	goose.SetBaseFS(migrations.FS)
	defer goose.SetBaseFS(nil)
	// coverage:ignore reason: dialect registration failure, not exercised by the happy-path test
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("testdb: set dialect: %v", err)
	}
	// coverage:ignore reason: migration failure, not exercised by the happy-path test
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("testdb: apply migrations: %v", err)
	}

	appDB := newAppDB(ctx, t, db, dsn)

	return &DB{Admin: db, App: appDB}
}

// newAppDB creates a per-test LOGIN role in the app_runtime group (the
// role the migrations grant table privileges to) and opens a connection
// as it, so tests can exercise queries the way the running application
// -- and the RLS policies scoped to app_runtime -- actually see them.
func newAppDB(ctx context.Context, t *testing.T, admin *sql.DB, adminDSN string) *sql.DB {
	t.Helper()

	const appUser = "app_test"
	const appPassword = "app_test"
	_, err := admin.ExecContext(ctx, "CREATE ROLE "+appUser+" LOGIN PASSWORD '"+appPassword+"' IN ROLE app_runtime")
	// coverage:ignore reason: role creation failure, not exercised by the happy-path test
	if err != nil {
		t.Fatalf("testdb: create app role: %v", err)
	}

	u, err := url.Parse(adminDSN)
	// coverage:ignore reason: malformed DSN from testcontainers, not exercised by the happy-path test
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
