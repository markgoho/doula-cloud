package billing_test

import (
	"database/sql"
	"errors"
	"testing"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/testdb"
)

// seedClientEngagement gives ConsumeCredit's tests an Engagement id to tag
// consumption rows with -- #52 owns the real insert path, but this ticket
// (#76) has no real caller yet (see billing.ConsumeCredit's doc comment),
// so tests seed one directly, following the seedClientEngagement pattern
// used by staffauth/engagement/plans/etc.'s own test helpers. Only the
// Engagement id is returned -- unlike those packages, nothing here needs
// the Client id back.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID string) (engagementID string) {
	t.Helper()
	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ('Test Client', 'client@example.com') RETURNING id`,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id) VALUES ($1, $2) RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return engagementID
}

// beginAsPractice opens a tx on db.App and sets app.current_practice_id --
// the same contract staffauth.Middleware establishes before any real
// handler runs (see billing.ConsumeCredit's doc comment) -- so these
// tests exercise ConsumeCredit through RLS, not bypassing it via Admin.
func beginAsPractice(t *testing.T, db *testdb.DB, practiceID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	return tx
}

func ledgerRowCount(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM credit_ledger WHERE practice_id = $1`, practiceID).Scan(&count); err != nil {
		t.Fatalf("count credit_ledger rows: %v", err)
	}
	return count
}

// TestConsumeCredit_AppendsConsumptionRowWhenBalancePositive proves a
// successful call appends exactly one -1 row tagged with the given
// Engagement id.
func TestConsumeCredit_AppendsConsumptionRowWhenBalancePositive(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Some Practice")
	seedSignupBonus(t, db, practiceID)
	engagementID := seedClientEngagement(t, db, practiceID)

	tx := beginAsPractice(t, db, practiceID)
	if err := billing.ConsumeCredit(t.Context(), tx, practiceID, engagementID); err != nil {
		t.Fatalf("ConsumeCredit: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var origin string
	var quantity int
	var consumedEngagementID string
	err := db.Admin.QueryRowContext(t.Context(),
		`SELECT origin, quantity, consumed_engagement_id FROM credit_ledger WHERE practice_id = $1 AND origin = 'consumption'`,
		practiceID,
	).Scan(&origin, &quantity, &consumedEngagementID)
	if err != nil {
		t.Fatalf("query consumption row: %v", err)
	}
	if origin != "consumption" || quantity != -1 || consumedEngagementID != engagementID {
		t.Fatalf("consumption row = (%q, %d, %q), want (\"consumption\", -1, %q)", origin, quantity, consumedEngagementID, engagementID)
	}
	if got := ledgerRowCount(t, db, practiceID); got != 2 {
		t.Fatalf("credit_ledger row count = %d, want 2 (signup_bonus + consumption)", got)
	}
}

// TestConsumeCredit_FailsWhenBalanceIsZeroOrLess proves a Practice with
// no credits gets ErrNoCreditsRemaining and no row is written.
func TestConsumeCredit_FailsWhenBalanceIsZeroOrLess(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Broke Practice")
	engagementID := seedClientEngagement(t, db, practiceID)

	tx := beginAsPractice(t, db, practiceID)
	err := billing.ConsumeCredit(t.Context(), tx, practiceID, engagementID)
	if !errors.Is(err, billing.ErrNoCreditsRemaining) {
		t.Fatalf("ConsumeCredit error = %v, want ErrNoCreditsRemaining", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if got := ledgerRowCount(t, db, practiceID); got != 0 {
		t.Fatalf("credit_ledger row count = %d, want 0", got)
	}
}

// TestConsumeCredit_SequenceExhaustsFreeCreditsThenFails drains a
// Practice's signup-bonus 3 credits one call at a time, then confirms the
// 4th call fails cleanly and writes nothing.
func TestConsumeCredit_SequenceExhaustsFreeCreditsThenFails(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Free Tier Practice")
	seedSignupBonus(t, db, practiceID)

	for i := range 3 {
		engagementID := seedClientEngagement(t, db, practiceID)
		tx := beginAsPractice(t, db, practiceID)
		if err := billing.ConsumeCredit(t.Context(), tx, practiceID, engagementID); err != nil {
			t.Fatalf("ConsumeCredit call %d: %v", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit call %d: %v", i+1, err)
		}
	}
	if got := ledgerRowCount(t, db, practiceID); got != 4 {
		t.Fatalf("credit_ledger row count after 3 consumptions = %d, want 4 (signup_bonus + 3 consumption)", got)
	}

	engagementID := seedClientEngagement(t, db, practiceID)
	tx := beginAsPractice(t, db, practiceID)
	err := billing.ConsumeCredit(t.Context(), tx, practiceID, engagementID)
	if !errors.Is(err, billing.ErrNoCreditsRemaining) {
		t.Fatalf("4th ConsumeCredit error = %v, want ErrNoCreditsRemaining", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if got := ledgerRowCount(t, db, practiceID); got != 4 {
		t.Fatalf("credit_ledger row count after failed 4th call = %d, want unchanged at 4", got)
	}
}

// TestConsumeCredit_CrossPracticeSessionCannotConsumeAnotherPracticesCredit
// follows rls_test.go's guardrail pattern: Practice B's session (the tx's
// app.current_practice_id) calling ConsumeCredit for Practice A's id must
// not spend A's credit -- credit_ledger's RLS policy hides A's rows from
// B's session, so the balance read comes back empty and the call fails
// the same way as a genuinely broke Practice, never a cross-tenant write.
func TestConsumeCredit_CrossPracticeSessionCannotConsumeAnotherPracticesCredit(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	seedSignupBonus(t, db, practiceA)
	engagementID := seedClientEngagement(t, db, practiceA)

	tx := beginAsPractice(t, db, practiceB)
	err := billing.ConsumeCredit(t.Context(), tx, practiceA, engagementID)
	if !errors.Is(err, billing.ErrNoCreditsRemaining) {
		t.Fatalf("ConsumeCredit error = %v, want ErrNoCreditsRemaining", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if got := ledgerRowCount(t, db, practiceA); got != 1 {
		t.Fatalf("Practice A's credit_ledger row count = %d, want 1 (untouched signup_bonus)", got)
	}
}
