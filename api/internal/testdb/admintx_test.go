package testdb_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

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
