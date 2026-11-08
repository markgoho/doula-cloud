package dbtx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/dbtx"
	"doula-cloud/api/internal/testdb"
)

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
	if !dbtx.Commit(w, tx) {
		t.Fatal("Commit failed")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want the recorder's untouched 200", w.Code)
	}
}
