// Command ops runs doula-ops, the Platform operator's service at
// ops.doula.cloud (ADR-0062). It connects to Postgres as an ops_runtime
// login, never as app_runtime, and shares no wiring with doula-api.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"

	"doula-cloud/api/internal/dbpool"
	"doula-cloud/api/internal/stackswitch"
	"doula-cloud/api/internal/stripemode"
)

func main() {
	server, _, err := newServer(context.Background(), os.Getenv, newTenantVerifier)
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

// The variables newServer reads beyond the database and the port.
const (
	emulatorHostVar = stackswitch.MarkerVar
	tenantVar       = "OPS_TENANT_ID"
	projectVar      = "GCP_PROJECT_ID"
	originsVar      = "EXPECTED_ORIGINS"
	databaseURLVar  = "DATABASE_URL"
)

var errEmulatorOnCloudRun = errors.New("doula-ops: " + emulatorHostVar + " must not be set on Cloud Run (" + stripemode.CloudRunVar + " is set)")

type verifierFactory func(ctx context.Context, projectID, tenantID string) (Verifier, error)

// newServer builds doula-ops from the environment: its pool capped at
// dbpool's share for doula-ops, on PORT, or Cloud Run's 8080 when unset.
// It refuses the emulator's host on Cloud Run, since that is what admits
// the emulator's phone factor in place of an authenticator app.
func newServer(ctx context.Context, getenv func(string) string, newVerifier verifierFactory) (*http.Server, *sql.DB, error) {
	emulator := getenv(emulatorHostVar) != ""
	if emulator && getenv(stripemode.CloudRunVar) != "" {
		return nil, nil, errEmulatorOnCloudRun
	}
	tenantID := strings.TrimSpace(getenv(tenantVar))
	if tenantID == "" {
		return nil, nil, fmt.Errorf("doula-ops: %s must be set", tenantVar)
	}
	verifier, err := newVerifier(ctx, getenv(projectVar), tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("doula-ops: %w", err)
	}

	db, err := sql.Open("pgx", getenv(databaseURLVar))
	// coverage:ignore reason: pgx's sql.Open never fails; it connects on first use
	if err != nil {
		// coverage:ignore reason: pgx's sql.Open never fails; it connects on first use
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	dbpool.ApplyOps(db)

	now, _, err := stackswitch.Clock(ctx, getenv, db)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("doula-ops: %w", err)
	}

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	handler, _ := routes(Deps{
		DB:                   db,
		Now:                  now,
		Verifier:             verifier,
		AcceptEmulatorFactor: emulator,
		Origins:              origins(getenv(originsVar)),
	})
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}, db, nil
}

// origins reads a comma-separated origin list; unset is none, which
// refuses every state-changing request that names an origin.
func origins(raw string) []string {
	var list []string
	for part := range strings.SplitSeq(raw, ",") {
		if origin := strings.TrimSpace(part); origin != "" {
			list = append(list, origin)
		}
	}
	return list
}
