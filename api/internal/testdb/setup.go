package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"doula-cloud/api/db/migrations"
)

// templateDB is the migrated database New's clones are stamped from.
const templateDB = "template_test"

// createAppRoleSQL mirrors app/compose.e2e.yaml's seed-role block: roles
// are cluster-global, not per-database, so on a shared container the
// bare `CREATE ROLE` this file used to run per-test would fail with
// "role already exists" on the second call. app_runtime (the group this
// role joins) is created by the migrations applied to templateDB below,
// and exists cluster-wide by the time this runs.
const createAppRoleSQL = `
DO $$ BEGIN
	IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '` + appUser + `') THEN
		CREATE ROLE ` + appUser + ` LOGIN PASSWORD '` + appPassword + `' IN ROLE app_runtime;
	END IF;
END $$;`

var (
	setupOnce sync.Once
	errSetup  error
	container *postgres.PostgresContainer

	// maintenanceDSN points at the container's default "postgres"
	// database -- never at templateDB. CREATE DATABASE ... TEMPLATE
	// fails while any connection to the template is open, so every
	// admin/clone connection in this package goes through this DSN
	// instead.
	maintenanceDSN string

	// cloneSeq numbers each cloned database ("test_1", "test_2", ...).
	// CREATE DATABASE doesn't accept a placeholder for the name, so the
	// value is interpolated directly -- safe here because it's this
	// package's own atomic counter, never external input.
	cloneSeq atomic.Int64
)

// Main runs m.Run() and terminates the package's shared Postgres
// container afterward. Every package that calls New must define:
//
//	func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }
//
// New starts one container per test *process* (via sync.Once) and clones
// a database from it per test, so nothing short of process exit is safe
// to terminate the container -- a mid-run t.Cleanup would pull it out
// from under every other test in the package. CI's Ryuk reaper would
// eventually catch a leaked container anyway, but local dev disables
// Ryuk under Podman (docs/testing.md), so without this, every `go test
// ./...` run would leave one Postgres container behind per package.
func Main(m *testing.M) int {
	code := m.Run()
	// go test -coverprofile finalizes the profile inside m.Run(), so
	// nothing below this point is ever reflected in coverage.out, even
	// though it does run (verified via manual testing: the
	// container.Terminate log lines appear in a local run).
	if container != nil {
		// coverage:ignore reason: unreachable in coverage.out, see comment above
		if err := container.Terminate(context.Background()); err != nil {
			// coverage:ignore reason: unreachable in coverage.out, see comment above
			fmt.Fprintf(os.Stderr, "testdb: terminate postgres container: %v\n", err)
		}
	}
	// coverage:ignore reason: unreachable in coverage.out, see comment above
	return code
}

// setup starts the shared container, applies the goose migrations to
// templateDB, and creates the app_test role -- once per test process,
// via setupOnce in New.
func setup(ctx context.Context) error {
	c, err := postgres.Run(ctx, "docker.io/library/postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testdb"),
		postgres.WithPassword("testdb"),
		postgres.BasicWaitStrategies(),
	)
	// coverage:ignore reason: container startup failure, not exercised by the happy-path test
	if err != nil {
		return fmt.Errorf("start postgres container: %w", err)
	}
	container = c

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	// coverage:ignore reason: connection string failure, not exercised by the happy-path test
	if err != nil {
		return fmt.Errorf("connection string: %w", err)
	}
	u, err := url.Parse(dsn)
	// coverage:ignore reason: malformed DSN from testcontainers, not exercised by the happy-path test
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	u.Path = "/postgres"
	maintenanceDSN = u.String()

	admin, err := sql.Open("pgx", maintenanceDSN)
	// coverage:ignore reason: driver open failure, not exercised by the happy-path test
	if err != nil {
		return fmt.Errorf("open maintenance db: %w", err)
	}
	defer func() { _ = admin.Close() }()

	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+templateDB); err != nil {
		// coverage:ignore reason: template creation failure, not exercised by the happy-path test
		return fmt.Errorf("create template database: %w", err)
	}

	if err := migrateTemplate(); err != nil {
		// coverage:ignore reason: migration failure, not exercised by the happy-path test
		return err
	}

	if _, err := admin.ExecContext(ctx, createAppRoleSQL); err != nil {
		// coverage:ignore reason: role creation failure, not exercised by the happy-path test
		return fmt.Errorf("create app role: %w", err)
	}

	return nil
}

// migrateTemplate applies the goose migrations to templateDB, then closes
// its connection -- CREATE DATABASE ... TEMPLATE refuses to clone a
// database with any open connection.
func migrateTemplate() error {
	u, err := url.Parse(maintenanceDSN)
	// coverage:ignore reason: malformed maintenance DSN, not exercised by the happy-path test
	if err != nil {
		return fmt.Errorf("parse maintenance dsn: %w", err)
	}
	u.Path = "/" + templateDB

	tmpl, err := sql.Open("pgx", u.String())
	// coverage:ignore reason: driver open failure, not exercised by the happy-path test
	if err != nil {
		return fmt.Errorf("open template db: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	defer goose.SetBaseFS(nil)
	// coverage:ignore reason: dialect registration failure, not exercised by the happy-path test
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	// coverage:ignore reason: migration failure, not exercised by the happy-path test
	if err := goose.Up(tmpl, "."); err != nil {
		return fmt.Errorf("apply migrations to template: %w", err)
	}

	if err := tmpl.Close(); err != nil {
		// coverage:ignore reason: connection close failure, not exercised by the happy-path test
		return fmt.Errorf("close template connection: %w", err)
	}
	return nil
}

// cloneDatabase creates a new database from templateDB -- a file copy,
// not a migration replay -- and returns its name.
func cloneDatabase(ctx context.Context, t *testing.T) string {
	t.Helper()

	name := fmt.Sprintf("test_%d", cloneSeq.Add(1))

	admin, err := sql.Open("pgx", maintenanceDSN)
	// coverage:ignore reason: driver open failure, not exercised by the happy-path test
	if err != nil {
		t.Fatalf("testdb: open admin db: %v", err)
	}
	defer func() { _ = admin.Close() }()

	// t.Parallel() isn't used anywhere in api/ today, so clones are
	// currently serial within a package and this retry never fires --
	// it's here so that changes independently in the future without
	// reintroducing this flake.
	const maxAttempts = 5
	for attempt := 1; ; attempt++ {
		_, err = admin.ExecContext(ctx, "CREATE DATABASE "+name+" TEMPLATE "+templateDB)
		if err == nil {
			return name
		}
		// coverage:ignore reason: only reachable under concurrent clones, not exercised by the happy-path test
		if attempt >= maxAttempts || !strings.Contains(err.Error(), "is being accessed by other users") {
			t.Fatalf("testdb: clone template database: %v", err)
		}
		// coverage:ignore reason: only reachable under concurrent clones, not exercised by the happy-path test
		time.Sleep(100 * time.Millisecond)
	}
}
