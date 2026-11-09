package dbtx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/dbtx"
	"doula-cloud/api/internal/testdb"
)

func TestStrings(t *testing.T) {
	db := testdb.New(t)
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	got, err := dbtx.Strings(t.Context(), tx, `SELECT s FROM unnest($1::text[]) AS s`, []string{"a", "b"})
	if err != nil || len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("Strings = (%v, %v), want [a b]", got, err)
	}
}

func TestBeginReadOnly_RefusesAWrite(t *testing.T) {
	db := testdb.New(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)

	tx, ok := dbtx.BeginReadOnly(w, r, db.Admin)
	if !ok {
		t.Fatal("BeginReadOnly refused a reachable database")
	}
	defer dbtx.Rollback(tx)

	if _, err := tx.ExecContext(t.Context(), `CREATE TEMP TABLE dbtx_readonly_probe (n int)`); err == nil {
		t.Fatal("a read-only transaction accepted a write")
	}
}

func TestLifecycle(t *testing.T) {
	db := testdb.New(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/x", nil)

	tx, ok := dbtx.Begin(w, r, db.Admin)
	if !ok {
		t.Fatal("Begin refused a reachable database")
	}
	defer dbtx.Rollback(tx)

	if _, err := tx.ExecContext(t.Context(), `CREATE TEMP TABLE dbtx_probe (n int)`); err != nil {
		t.Fatalf("create probe: %v", err)
	}
	rows, ok := dbtx.ExecRows(t.Context(), w, tx, `INSERT INTO dbtx_probe (n) VALUES (1), (2)`)
	if !ok || rows != 2 {
		t.Fatalf("ExecRows = (%d, %v), want (2, true)", rows, ok)
	}
	dbtx.CommitWith(w, tx, http.StatusAccepted)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}
}
