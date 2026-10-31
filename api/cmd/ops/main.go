// Command ops runs doula-ops, the Platform operator's service at
// ops.doula.cloud (ADR-0062). It connects to Postgres as an ops_runtime
// login, never as app_runtime, and shares no wiring with doula-api.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/dbpool"
)

func main() {
	server, _, err := newServer(os.Getenv)
	// coverage:ignore reason: main's wiring; newServer's own tests cover it
	if err != nil {
		// coverage:ignore reason: main's wiring; newServer's own tests cover it
		log.Fatalf("doula-ops: %v", err)
	}
	log.Printf("doula-ops listening on %s", server.Addr)
	// coverage:ignore reason: listener startup, not exercised by unit tests
	if err := server.ListenAndServe(); err != nil {
		// coverage:ignore reason: listener startup, not exercised by unit tests
		log.Fatal(err)
	}
}

// newServer builds doula-ops from the environment: its pool capped at
// dbpool's share for doula-ops, on PORT, or Cloud Run's 8080 when unset.
func newServer(getenv func(string) string) (*http.Server, *sql.DB, error) {
	db, err := sql.Open("pgx", getenv("DATABASE_URL"))
	// coverage:ignore reason: pgx's sql.Open never fails; it connects on first use
	if err != nil {
		// coverage:ignore reason: pgx's sql.Open never fails; it connects on first use
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	dbpool.ApplyOps(db)

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	handler, _ := routes(Deps{DB: db, Now: clock.Real})
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}, db, nil
}
