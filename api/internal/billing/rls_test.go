package billing_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the credit_ledger_practice_visibility policy from
// 00015_credit_ledger.sql directly via db.App and set_config, following the
// pattern in staffauth/rls_test.go.

func seedSignupBonus(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	id := testdb.Scalar[string](t, db.Admin, `INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 3) RETURNING id`,
		practiceID)
	return id
}

// TestRLS_CreditLedgerFailsClosedWithNoSessionVarSet proves credit_ledger
// denies all rows when app.current_practice_id is unset.
func TestRLS_CreditLedgerFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Some Practice")
	seedSignupBonus(t, db, practiceID)

	count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM credit_ledger`)
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_CreditLedgerVisibilityIsScopedToCurrentPractice proves Practice
// A's session sees only Practice A's ledger rows -- Practice B's ledger
// stays invisible and unaffected.
func TestRLS_CreditLedgerVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	rowA := seedSignupBonus(t, db, practiceA)
	seedSignupBonus(t, db, practiceB)

	tx := db.BeginAs(t, testdb.Practice(practiceA))
	visibleIDs := testdb.Column[string](t, tx, `SELECT id FROM credit_ledger`)

	if len(visibleIDs) != 1 || visibleIDs[0] != rowA {
		t.Fatalf("visible credit_ledger rows = %v, want only %q", visibleIDs, rowA)
	}
}

// TestRLS_CreditLedgerCannotInsertForAnotherPractice proves Practice B's
// session cannot append a ledger row for Practice A -- the WITH CHECK side
// of the policy, derived from the same USING clause, rejects it.
func TestRLS_CreditLedgerCannotInsertForAnotherPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")

	tx := db.BeginAs(t, testdb.Practice(practiceB))
	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'purchase', 5)`,
		practiceA,
	)
	if err == nil {
		t.Fatal("expected inserting a ledger row for another Practice to be rejected by RLS, got no error")
	}
}

// TestRLS_NotificationWorkerDoor proves 00033's two policies on staff and
// practice_memberships, the door billing.Worker's ownerEmails query relies
// on. They stay closed by default -- with no session context at all, the
// shape outbox.ProcessHandler runs under before it sets
// app.notification_worker_trusted, neither table admits a row -- and open,
// regardless of Practice, once the trusted flag is set.
func TestRLS_NotificationWorkerDoor(t *testing.T) {
	cases := []struct {
		name string
		vars []testdb.SessionVar
		want int
	}{
		{name: "closed without the trusted flag", want: 0},
		{name: "open with the trusted flag", vars: []testdb.SessionVar{testdb.NotificationWorker()}, want: 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := testdb.New(t)
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "worker-door", []string{ownerRole}, "employee")
			tx := db.BeginAs(t, c.vars...)

			staffCount := testdb.Scalar[int](t, tx, `SELECT count(*) FROM staff`)
			membershipCount := testdb.Scalar[int](t, tx, `SELECT count(*) FROM practice_memberships WHERE practice_id = $1`, practiceID)

			if staffCount != c.want || membershipCount != c.want {
				t.Fatalf("staffCount/membershipCount = %d/%d, want %d/%d", staffCount, membershipCount, c.want, c.want)
			}
		})
	}
}

// TestRLS_CreditLedgerIsAppendOnlyForTheRuntime proves nothing the BFF
// runs can write off a Credit balance: app_runtime holds SELECT and
// INSERT on credit_ledger and nothing else, so a stale balance can be
// corrected only by a new row that says so -- and New York's abandoned
// property law is owed the whole balance, not a written-down one (#420).
func TestRLS_CreditLedgerIsAppendOnlyForTheRuntime(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Append Only")
	seedSignupBonus(t, db, practiceID)

	// A refused statement aborts its transaction, so each attempt runs
	// in one of its own.
	for name, statement := range map[string]string{
		"zero a balance":   `UPDATE credit_ledger SET quantity = 0`,
		"delete a balance": `DELETE FROM credit_ledger`,
	} {
		tx := db.BeginAs(t, testdb.Practice(practiceID))
		if _, err := tx.ExecContext(t.Context(), statement); err == nil {
			t.Fatalf("%s: the runtime was allowed to rewrite credit_ledger", name)
		}
		_ = tx.Rollback()
	}
}
