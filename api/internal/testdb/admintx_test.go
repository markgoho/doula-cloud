package testdb_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"doula-cloud/api/internal/testdb"
)

func TestRunAsNotificationWorker_OpensTheDoorAndCommits(t *testing.T) {
	db := testdb.New(t)

	db.RunAsNotificationWorker(t, func(ctx context.Context, tx *sql.Tx) error {
		var door string
		if err := tx.QueryRowContext(ctx, `SELECT current_setting('app.notification_worker_trusted')`).Scan(&door); err != nil {
			return fmt.Errorf("read the door: %w", err)
		}
		if door != "true" {
			t.Errorf("door = %q, want true", door)
		}
		return nil
	})
}

func TestBeginAdmin_CommitsWhatTheTestCommitsAndRollsBackTheRest(t *testing.T) {
	db := testdb.New(t)
	committed := db.BeginAdmin(t)
	if _, err := committed.ExecContext(t.Context(), `CREATE TABLE admintx_kept (id int)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := committed.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got := testdb.Scalar[bool](t, db.Admin, `SELECT to_regclass('admintx_kept') IS NOT NULL`); !got {
		t.Fatal("a committed write did not survive")
	}

	open := db.BeginAdmin(t)
	if got := testdb.Scalar[int](t, open, `SELECT 1`); got != 1 {
		t.Fatalf("select 1 = %d", got)
	}
}
