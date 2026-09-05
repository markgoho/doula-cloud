package simclock_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"doula-cloud/api/db/migrations"
	"doula-cloud/api/internal/simclock"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// superuserRole is the container's own bootstrap user -- a real
// superuser, standing in for app in the e2e stack (compose.e2e.yaml's
// POSTGRES_USER).
const superuserRole = "simclock"

var (
	maintenanceDSN string
	dbSeq          atomic.Int64
)

// TestMain starts one shared, disposable Postgres container for the
// whole package -- freshDB below creates a genuinely empty database per
// test rather than cloning a migrated template (testdb's own approach),
// because this package's whole job is what happens *before* a database
// has ever seen a migration.
func TestMain(m *testing.M) {
	ctx := context.Background()
	c, err := postgres.Run(ctx, "docker.io/library/postgres:16-alpine",
		postgres.WithDatabase(superuserRole),
		postgres.WithUsername(superuserRole),
		postgres.WithPassword(superuserRole),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "simclock: start postgres container: %v\n", err)
		os.Exit(1)
	}
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "simclock: connection string: %v\n", err)
		os.Exit(1)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "simclock: parse dsn: %v\n", err)
		os.Exit(1)
	}
	u.Path = "/" + superuserRole
	maintenanceDSN = u.String()

	code := m.Run()
	if err := c.Terminate(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "simclock: terminate postgres container: %v\n", err)
	}
	os.Exit(code)
}

// freshDB creates a brand-new, empty database on the shared container --
// no migrations, no shim -- and returns an admin (superuser) connection
// to it plus its own DSN, for callers that need a second connection
// (e.g. one taken out after an ALTER ROLE, since a pooled connection
// opened beforehand won't see it).
func freshDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("sim_%d", dbSeq.Add(1))

	admin, err := sql.Open("pgx", maintenanceDSN)
	if err != nil {
		t.Fatalf("simclock: open maintenance db: %v", err)
	}
	defer func() { _ = admin.Close() }()
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("simclock: create database: %v", err)
	}
	t.Cleanup(func() {
		a, err := sql.Open("pgx", maintenanceDSN)
		if err != nil {
			return
		}
		defer func() { _ = a.Close() }()
		_, _ = a.ExecContext(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)")
	})

	u, err := url.Parse(maintenanceDSN)
	if err != nil {
		t.Fatalf("simclock: parse maintenance dsn: %v", err)
	}
	u.Path = "/" + name
	dsn := u.String()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("simclock: open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, dsn
}

func TestInstall_PristineDatabaseAdvancesWithTheOffset(t *testing.T) {
	ctx := context.Background()
	db, dsn := freshDB(t)

	if err := simclock.Install(ctx, db, superuserRole); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// A fresh connection, so the ALTER ROLE ... SET search_path Install
	// just ran is actually in effect for it -- database/sql may still be
	// holding db's own connection open from before that statement ran.
	fresh, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open fresh connection: %v", err)
	}
	defer func() { _ = fresh.Close() }()

	if _, err := fresh.ExecContext(ctx, "CREATE TABLE t (at timestamptz DEFAULT now())"); err != nil {
		t.Fatalf("create table with DEFAULT now(): %v", err)
	}

	if _, err := fresh.ExecContext(ctx, "INSERT INTO t DEFAULT VALUES"); err != nil {
		t.Fatalf("insert before advancing: %v", err)
	}
	var beforeAdvance time.Time
	if err := fresh.QueryRowContext(ctx, "SELECT at FROM t").Scan(&beforeAdvance); err != nil {
		t.Fatalf("select before advancing: %v", err)
	}
	// A small negative delta is ordinary clock skew between this process
	// and the container's own clock, not evidence the row is on
	// simulated time -- only a multi-day gap (what a missed shim would
	// produce) should fail this.
	if delta := time.Since(beforeAdvance); delta < -time.Minute || delta > time.Minute {
		t.Fatalf("row written before any advance should read close to real time, got %v ago", delta)
	}

	if _, err := fresh.ExecContext(ctx, "UPDATE sim.offset_row SET delta = '90 days'"); err != nil {
		t.Fatalf("advance offset: %v", err)
	}
	if _, err := fresh.ExecContext(ctx, "INSERT INTO t DEFAULT VALUES"); err != nil {
		t.Fatalf("insert after advancing: %v", err)
	}
	var afterAdvance time.Time
	if err := fresh.QueryRowContext(ctx, "SELECT at FROM t ORDER BY at DESC LIMIT 1").Scan(&afterAdvance); err != nil {
		t.Fatalf("select after advancing: %v", err)
	}
	if got := afterAdvance.Sub(beforeAdvance); got < 89*24*time.Hour {
		t.Fatalf("row written after a 90-day advance should be ~90 days later, got %v", got)
	}
}

func TestInstall_IdempotentOnAnAlreadyInstalledDatabase(t *testing.T) {
	ctx := context.Background()
	db, _ := freshDB(t)

	if err := simclock.Install(ctx, db, superuserRole); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	if err := simclock.Install(ctx, db, superuserRole); err != nil {
		t.Fatalf("second Install should be a no-op, got: %v", err)
	}
}

func TestInstall_RefusesADatabaseMigratedWithoutTheShim(t *testing.T) {
	ctx := context.Background()
	db, dsn := freshDB(t)

	tmpl, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db for migrating: %v", err)
	}
	goose.SetBaseFS(migrations.FS)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}
	if err := goose.Up(tmpl, "."); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := tmpl.Close(); err != nil {
		t.Fatalf("close migrating connection: %v", err)
	}

	err = simclock.Install(ctx, db, superuserRole)
	if !errors.Is(err, simclock.ErrAlreadyMigrated) {
		t.Fatalf("Install on an already-migrated database: got %v, want ErrAlreadyMigrated", err)
	}
}

func TestInstall_RejectsAnUnsafeRoleName(t *testing.T) {
	db, _ := freshDB(t)
	if err := simclock.Install(context.Background(), db, "app; DROP TABLE staff"); err == nil {
		t.Fatal("Install should reject a role name that isn't a plain identifier")
	}
}

func TestInstall_ReportsAnInstallFailure(t *testing.T) {
	ctx := context.Background()
	db, dsn := freshDB(t)

	const lowPrivRole = "sim_low_priv"
	if _, err := db.ExecContext(ctx, "CREATE ROLE "+lowPrivRole+" LOGIN PASSWORD '"+lowPrivRole+"'"); err != nil {
		t.Fatalf("create low-privilege role: %v", err)
	}

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	u.User = url.UserPassword(lowPrivRole, lowPrivRole)
	lowPriv, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatalf("open low-privilege connection: %v", err)
	}
	defer func() { _ = lowPriv.Close() }()

	// A login role with no CREATE privilege on this database (Postgres
	// 15+'s default, since it isn't the owner) can't run CREATE SCHEMA --
	// exactly the failure this exercises.
	err = simclock.Install(ctx, lowPriv, lowPrivRole)
	if err == nil || errors.Is(err, simclock.ErrAlreadyMigrated) {
		t.Fatalf("Install without CREATE privilege should fail with a plain error, got %v", err)
	}
}

func TestInstall_ReportsAnAlterRoleFailure(t *testing.T) {
	db, _ := freshDB(t)
	if err := simclock.Install(context.Background(), db, "does_not_exist"); err == nil {
		t.Fatal("Install should fail when the migrating role doesn't exist yet")
	}
}

func TestGrant_LetsANonSuperuserRoleReadSimulatedTime(t *testing.T) {
	ctx := context.Background()
	db, dsn := freshDB(t)

	if err := simclock.Install(ctx, db, superuserRole); err != nil {
		t.Fatalf("Install: %v", err)
	}

	const runtimeRole = "app_e2e_test"
	if _, err := db.ExecContext(ctx, "CREATE ROLE "+runtimeRole+" LOGIN PASSWORD '"+runtimeRole+"'"); err != nil {
		t.Fatalf("create runtime role: %v", err)
	}

	if err := simclock.Grant(ctx, db, runtimeRole); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	if _, err := db.ExecContext(ctx, "UPDATE sim.offset_row SET delta = '30 days'"); err != nil {
		t.Fatalf("advance offset: %v", err)
	}

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	u.User = url.UserPassword(runtimeRole, runtimeRole)
	runtime, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatalf("open runtime connection: %v", err)
	}
	defer func() { _ = runtime.Close() }()

	var readsSimulatedTime bool
	err = runtime.QueryRowContext(ctx, "SELECT now() > pg_catalog.now() + interval '29 days'").Scan(&readsSimulatedTime)
	if err != nil {
		t.Fatalf("select now() as %s: %v", runtimeRole, err)
	}
	if !readsSimulatedTime {
		t.Fatalf("%s's now() should resolve to sim.now(), not real time", runtimeRole)
	}
}

func TestGrant_RejectsAnUnsafeRoleName(t *testing.T) {
	db, _ := freshDB(t)
	if err := simclock.Grant(context.Background(), db, "app_e2e; DROP TABLE staff"); err == nil {
		t.Fatal("Grant should reject a role name that isn't a plain identifier")
	}
}

func TestGrant_ReportsAGrantFailure(t *testing.T) {
	db, _ := freshDB(t)
	if err := simclock.Grant(context.Background(), db, "does_not_exist"); err == nil {
		t.Fatal("Grant should fail when the target role doesn't exist")
	}
}
