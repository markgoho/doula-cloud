// Command ops runs doula-ops at ops.doula.cloud, connecting to Postgres as ops_runtime, never app_runtime.
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

	_ "github.com/jackc/pgx/v5/stdlib"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/dbpool"
	"doula-cloud/api/internal/stackswitch"
	"doula-cloud/api/internal/stripemode"
)

func main() {
	server, _, err := newServer(context.Background(), os.Getenv, newTenantVerifier, newStaffAccounts)
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

const (
	emulatorHostVar = stackswitch.MarkerVar
	tenantVar       = "OPS_TENANT_ID"
	projectVar      = "GCP_PROJECT_ID"
	originsVar      = "EXPECTED_ORIGINS"
	databaseURLVar  = "DATABASE_URL"
	stripeKeyVar    = "STRIPE_API_KEY"
	creditPriceVar  = "STRIPE_CREDIT_PRICE_ID" // #nosec G101 -- an environment variable's name, not a credential
	feedbackRepoVar = "GITHUB_FEEDBACK_REPO"
)

var errEmulatorOnCloudRun = errors.New("doula-ops: " + emulatorHostVar + " must not be set on Cloud Run (" + stripemode.CloudRunVar + " is set)")

type verifierFactory func(ctx context.Context, projectID, tenantID string) (Verifier, error)

type accountsFactory func(ctx context.Context, projectID string) (authn.AccountManager, error)

func newStaffAccounts(ctx context.Context, projectID string) (authn.AccountManager, error) {
	// coverage:ignore reason: requires GCP credentials or the Auth emulator, not exercised by unit tests
	accounts, err := authn.NewFirebaseVerifier(ctx, projectID)
	if err != nil {
		// coverage:ignore reason: requires GCP credentials or the Auth emulator, not exercised by unit tests
		return nil, fmt.Errorf("staff accounts: %w", err)
	}
	// coverage:ignore reason: requires GCP credentials or the Auth emulator, not exercised by unit tests
	return accounts, nil
}

// The emulator's host is refused on Cloud Run, since it is what admits the emulator's phone factor.
func newServer(ctx context.Context, getenv func(string) string, newVerifier verifierFactory, newAccounts accountsFactory) (*http.Server, *sql.DB, error) {
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
	accounts, err := newAccounts(ctx, getenv(projectVar))
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

	history, err := stackswitch.Legal(ctx, getenv, db, shippedLegal)
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
		Stripe:               stripeClient(getenv(stripeKeyVar), getenv(creditPriceVar), emulator),
		Accounts:             accounts,
		FeedbackRepo:         strings.TrimSpace(getenv(feedbackRepoVar)),
		Legal:                history,
	})
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}, db, nil
}

// The end-to-end stack has no Stripe, so with the emulator and no key this is the in-memory fake.
func stripeClient(apiKey, creditPriceID string, emulator bool) billing.StripeClient {
	if emulator && apiKey == "" {
		return billing.NewFakeStripeClient()
	}
	return billing.NewStripeAPIClient(apiKey, creditPriceID, "")
}

// Unset is none, which refuses every state-changing request that names an origin.
func origins(raw string) []string {
	var list []string
	for part := range strings.SplitSeq(raw, ",") {
		if origin := strings.TrimSpace(part); origin != "" {
			list = append(list, origin)
		}
	}
	return list
}
