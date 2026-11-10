package outbox_test

import (
	"testing"

	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

func TestScanMeta_ReadsTheIDAndAttemptsAheadOfTheRow(t *testing.T) {
	db := testdb.New(t)
	rows, err := db.Admin.QueryContext(t.Context(), `SELECT 'row-1', 3, 'person@example.com'`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatal("no row")
	}
	var to string
	meta, err := outbox.ScanMeta(rows, &to)
	if err != nil {
		t.Fatalf("ScanMeta: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if meta != (outbox.RowMeta{ID: "row-1", AttemptCount: 3}) || to != "person@example.com" {
		t.Errorf("ScanMeta = (%+v, %q), want (row-1 at 3 attempts, person@example.com)", meta, to)
	}
}
