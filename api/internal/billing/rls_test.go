package billing_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the credit_ledger_practice_visibility policy from
// 00014_credit_ledger.sql directly via db.App and set_config, following the
// pattern in staffauth/rls_test.go.

func seedPractice(t *testing.T, db *testdb.DB, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(), `INSERT INTO practices (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed practice %q: %v", name, err)
	}
	return id
}

func seedSignupBonus(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 3) RETURNING id`,
		practiceID,
	).Scan(&id); err != nil {
		t.Fatalf("seed credit_ledger row: %v", err)
	}
	return id
}

// TestRLS_CreditLedgerFailsClosedWithNoSessionVarSet proves credit_ledger
// denies all rows when app.current_practice_id is unset.
func TestRLS_CreditLedgerFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Some Practice")
	seedSignupBonus(t, db, practiceID)

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM credit_ledger`).Scan(&count); err != nil {
		t.Fatalf("query credit_ledger with no session variables set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_CreditLedgerVisibilityIsScopedToCurrentPractice proves Practice
// A's session sees only Practice A's ledger rows -- Practice B's ledger
// stays invisible and unaffected.
func TestRLS_CreditLedgerVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	rowA := seedSignupBonus(t, db, practiceA)
	seedSignupBonus(t, db, practiceB)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM credit_ledger`)
	if err != nil {
		t.Fatalf("query credit_ledger: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visibleIDs = append(visibleIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visibleIDs) != 1 || visibleIDs[0] != rowA {
		t.Fatalf("visible credit_ledger rows = %v, want only %q", visibleIDs, rowA)
	}
}

// TestRLS_CreditLedgerCannotInsertForAnotherPractice proves Practice B's
// session cannot append a ledger row for Practice A -- the WITH CHECK side
// of the policy, derived from the same USING clause, rejects it.
func TestRLS_CreditLedgerCannotInsertForAnotherPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceB); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	_, err = tx.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'purchase', 5)`,
		practiceA,
	)
	if err == nil {
		t.Fatal("expected inserting a ledger row for another Practice to be rejected by RLS, got no error")
	}
}
