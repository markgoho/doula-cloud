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
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"doula-cloud/api/db/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// New starts a Postgres container, applies all goose migrations against it,
// and returns an open *sql.DB. The container and connection are torn down
// automatically via t.Cleanup.
func New(t *testing.T) *sql.DB {
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

	return db
}
