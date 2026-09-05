package main

import (
	"database/sql"
	"testing"
	"time"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const testRole = "app"

func TestRun_BadUsage(t *testing.T) {
	cases := [][]string{
		nil,
		{modeInstall},
		{"bogus", testRole},
		{modeInstall, testRole, "extra"},
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
