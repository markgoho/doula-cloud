package main

import (
	"database/sql"
	"testing"
	"time"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const testRole = "app"

// The argument fixtures the usage tests share: a surplus argument, a
// plausible client id, a well-formed Connect account id, and a week.
const (
	extraArg     = "extra"
	testClientID = "some-client"
	testAccount  = "acct_ok"
	testDelta    = "168h"
)

func TestRun_BadUsage(t *testing.T) {
	cases := [][]string{
		nil,
		{modeInstall},
		{"bogus", testRole},
		{modeInstall, testRole, extraArg},
	}
	for _, args := range cases {
		if err := run(args); err == nil {
			t.Fatalf("run(%v): expected a usage error, got nil", args)
		}
	}
}

func TestRun_RejectsAnUnsafeRoleName(t *testing.T) {
	if err := run([]string{modeInstall, "app; DROP TABLE staff"}); err == nil {
		t.Fatal("expected an error for a role name that isn't a plain identifier")
	}
}

func TestRun_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if err := run([]string{modeInstall, testRole}); err == nil {
		t.Fatal("expected an error when DATABASE_URL is unset, got nil")
	}
}

// TestWaitForConnection_TimesOutWhenUnreachable proves the retry loop
// gives up and returns an error once its budget elapses, instead of
// retrying forever, for a DSN nothing is listening on.
func TestWaitForConnection_TimesOutWhenUnreachable(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://app:app@127.0.0.1:1/app?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := waitForConnection(t.Context(), db, 300*time.Millisecond); err == nil {
		t.Fatal("expected an error once the timeout elapses, got nil")
	}
}

// TestRun_BadAllocateAndAdvanceUsage covers the argument validation on
// the two subcommands that reach Stripe: the wrong number of arguments,
// an account id that is not a Connect account id, and a duration Go
// cannot parse are all refused before any connection is opened.
func TestRun_BadAllocateAndAdvanceUsage(t *testing.T) {
	cases := [][]string{
		{modeAllocate},
		{modeAllocate, testClientID},
		{modeAllocate, testClientID, testAccount, extraArg},
		{modeAllocate, testClientID, "not-an-account"},
		{modeAdvance},
		{modeAdvance, testDelta, extraArg},
		{modeAdvance, "a fortnight"},
	}
	for _, args := range cases {
		if err := run(args); err == nil {
			t.Fatalf("run(%v): expected a usage error, got nil", args)
		}
	}
}

// TestRun_MissingStripeKey proves allocate and advance refuse before they
// open a database connection when there is no Stripe key to use.
func TestRun_MissingStripeKey(t *testing.T) {
	t.Setenv("STRIPE_API_KEY", "")
	t.Setenv("DATABASE_URL", "")

	for _, args := range [][]string{
		{modeAllocate, testClientID, testAccount},
		{modeAdvance, testDelta},
	} {
		if err := run(args); err == nil {
			t.Fatalf("run(%v): expected an error when STRIPE_API_KEY is unset, got nil", args)
		}
	}
}

// TestRun_MissingDatabaseURLForStripeModes proves the same for the
// database: a Stripe key alone is not enough to run either subcommand.
func TestRun_MissingDatabaseURLForStripeModes(t *testing.T) {
	t.Setenv("STRIPE_API_KEY", "sk_test_not_a_real_key")
	t.Setenv("DATABASE_URL", "")

	for _, args := range [][]string{
		{modeAllocate, testClientID, testAccount},
		{modeAdvance, testDelta},
	} {
		if err := run(args); err == nil {
			t.Fatalf("run(%v): expected an error when DATABASE_URL is unset, got nil", args)
		}
	}
}
