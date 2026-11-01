package outbox_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestEveryOutboxTableRecordsADeadLetterReason finds each table that can
// hold a dead-lettered row -- by its status column's enum or CHECK, not
// by name -- and requires the reason column and the constraint that ties
// it to the status on every one.
func TestEveryOutboxTableRecordsADeadLetterReason(t *testing.T) {
	db := testdb.New(t)

	rows, err := db.Admin.QueryContext(t.Context(), `
		WITH dead_letterable AS (
			SELECT c.table_name
			  FROM information_schema.columns c
			  JOIN pg_type ty ON ty.typname = c.udt_name
			  JOIN pg_enum e ON e.enumtypid = ty.oid AND e.enumlabel = 'dead_lettered'
			 WHERE c.table_schema = 'public' AND c.column_name = 'status'
			UNION
			SELECT rel.relname
			  FROM pg_constraint con
			  JOIN pg_class rel ON rel.oid = con.conrelid
			  JOIN pg_namespace n ON n.oid = rel.relnamespace AND n.nspname = 'public'
			 WHERE con.contype = 'c'
			   AND pg_get_constraintdef(con.oid) LIKE '%dead_lettered%'
			   AND con.conname NOT LIKE '%dead_letter_reason%'
		)
		SELECT d.table_name,
		       COALESCE((SELECT c.udt_name FROM information_schema.columns c
		                  WHERE c.table_schema = 'public' AND c.table_name = d.table_name
		                    AND c.column_name = 'dead_letter_reason'), ''),
		       EXISTS (SELECT 1 FROM pg_constraint con
		                 JOIN pg_class rel ON rel.oid = con.conrelid
		                WHERE rel.relname = d.table_name
		                  AND con.conname = d.table_name || '_dead_letter_reason_check')
		  FROM dead_letterable d
		 ORDER BY d.table_name`)
	if err != nil {
		t.Fatalf("query outbox tables: %v", err)
	}
	defer func() { _ = rows.Close() }()

	seen := 0
	for rows.Next() {
		var table, reasonType string
		var constrained bool
		if err := rows.Scan(&table, &reasonType, &constrained); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen++
		if reasonType != "outbox_dead_letter_reason" {
			t.Errorf("%s.dead_letter_reason type = %q, want outbox_dead_letter_reason", table, reasonType)
		}
		if !constrained {
			t.Errorf("%s has no %s_dead_letter_reason_check tying the reason to status", table, table)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	if seen == 0 {
		t.Fatal("found no table that can hold a dead-lettered row; the discovery query is wrong")
	}
}
