// Command migrate applies goose migrations directly against DATABASE_URL.
// It's the migration step for the self-contained e2e podman-compose stack
// (app/compose.e2e.yaml), which has no Cloud SQL Auth Proxy to go
// through -- scripts/migrate.sh remains the real deploy path.
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/pressly/goose/v3"

	"doula-cloud/api/db/migrations"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// coverage:ignore reason: exits the process, not exercised by unit tests
	if err := run(); err != nil {
		// coverage:ignore reason: exits the process, not exercised by unit tests
		log.Fatal(err)
	}
}

// run is exercised directly by main_test.go for the DATABASE_URL-unset
// path; everything past that needs a real Postgres instance, which is
// what the e2e run (app/e2e/global-setup.ts runs this binary against the
// compose Postgres) proves instead of a unit test.
func run() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("migrate: DATABASE_URL must be set")
	}

	// coverage:ignore reason: malformed DSN, not exercised by unit tests
	db, err := sql.Open("pgx", dsn)
	// coverage:ignore reason: malformed DSN, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: malformed DSN, not exercised by unit tests
		return fmt.Errorf("migrate: open db: %w", err)
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	defer func() { _ = db.Close() }()

	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	goose.SetBaseFS(migrations.FS)
	// coverage:ignore reason: dialect registration failure, not exercised by unit tests
	if err := goose.SetDialect("postgres"); err != nil {
		// coverage:ignore reason: dialect registration failure, not exercised by unit tests
		return fmt.Errorf("migrate: set dialect: %w", err)
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	if err := goose.Up(db, "."); err != nil {
		// coverage:ignore reason: migration failure, not exercised by unit tests
		return fmt.Errorf("migrate: apply migrations: %w", err)
	}
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	log.Println("migrate: migrations applied")
	// coverage:ignore reason: requires a real DB connection, not exercised by unit tests
	return nil
}
