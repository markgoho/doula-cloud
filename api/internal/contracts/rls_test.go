package contracts_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the contract_templates_practice_visibility RLS
// policy from 00014_contract_templates.sql directly via db.App and
// set_config, bypassing the Go handlers -- the same shape as plans'
// rls_test.go.

// TestRLS_ContractTemplatesFailsClosedWithNoSessionVarSet proves
// contract_templates denies all rows when app.current_practice_id is
// never set.
func TestRLS_ContractTemplatesFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Some Practice")
	seedTemplate(t, db, practiceID, "Some prose")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM contract_templates`).Scan(&count); err != nil {
		t.Fatalf("query contract_templates with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

// TestRLS_ContractTemplatesVisibilityIsScopedToCurrentPractice proves the
// plain-column-comparison policy narrows contract_templates to rows for
// app.current_practice_id, not every row globally.
func TestRLS_ContractTemplatesVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	seedTemplate(t, db, practiceA, "Practice A's prose")
	seedTemplate(t, db, practiceB, "Practice B's prose")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visiblePracticeIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT practice_id FROM contract_templates`)
	if err != nil {
		t.Fatalf("query contract_templates: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visiblePracticeIDs = append(visiblePracticeIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visiblePracticeIDs) != 1 || visiblePracticeIDs[0] != practiceA {
		t.Fatalf("visible practice_ids = %v, want only %q", visiblePracticeIDs, practiceA)
	}
}

// TestRLS_ContractTemplatesUpdateRejectedAcrossPractice proves the policy
// scopes UPDATE, not just SELECT: a session acting as Practice A can't
// edit a Contract Template that belongs to Practice B, even with the
// correct practice id, because the row isn't visible to update in the
// first place.
func TestRLS_ContractTemplatesUpdateRejectedAcrossPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	seedTemplate(t, db, practiceB, "Practice B's prose")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	result, err := tx.ExecContext(t.Context(),
		`UPDATE contract_templates SET prose = 'hacked' WHERE practice_id = $1`,
		practiceB,
	)
	if err != nil {
		t.Fatalf("update contract_templates: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 rows affected editing a Contract Template at a different Practice, got %d", rows)
	}
}
