package billing_test

import (
	"database/sql"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/testdb"
)

// seedPurchase seeds one purchase lot: quantity Credits at unitPriceCents
// each, with taxCents of sales tax charged on the whole lot, bought
// boughtAt and paid on paymentIntentID.
func seedPurchase(t *testing.T, db *testdb.DB, practiceID string, quantity int, unitPriceCents, taxCents int64, paymentIntentID string, boughtAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO credit_ledger
		     (practice_id, origin, quantity, unit_price_cents, tax_cents, stripe_payment_intent_id, created_at)
		 VALUES ($1, 'purchase', $2, $3, $4, $5, $6) RETURNING id`,
		practiceID, quantity, unitPriceCents, taxCents, paymentIntentID, boughtAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed purchase lot: %v", err)
	}
	return id
}

// seedEngagements seeds n Engagements for practiceID and returns their
// ids. Every caller seeds before opening its transaction: ConsumeCredit
// holds a lock on the Practice row, and an Engagement insert needs that
// row, so seeding inside the transaction waits on a lock it holds itself.
func seedEngagements(t *testing.T, db *testdb.DB, practiceID string, n int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := range ids {
		_, ids[i] = testdb.SeedEngagement(t, db, practiceID)
	}
	return ids
}

// practiceTx opens a transaction on the low-privilege app_runtime
// connection with practiceID in the session variable every credit_ledger
// policy reads -- what staffauth.Middleware does for a real request.
func practiceTx(t *testing.T, db *testdb.DB, practiceID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice: %v", err)
	}
	return tx
}

// TestRefundable_PricesTheUnspentBalanceFromTheLedgerAlone is the AC in
// TestConsumeCredit_DrawsTheOldestLotFirst proves the FIFO rule the
// pricing above rests on: a granted Credit is spent before a purchased
// one, and the consumption row names the lot it drew from.
func TestConsumeCredit_DrawsTheOldestLotFirst(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "FIFO")
	grantID := seedSignupBonus(t, db, practiceID)
	// An hour later, explicitly: the grant above is stamped with the
	// database's clock and this one with Go's, and the order is the
	// point of the test, so it must not rest on the two agreeing.
	purchaseID := seedPurchase(t, db, practiceID, 2, 2000, 0, "pi_fifo", time.Now().Add(time.Hour))

	engagements := seedEngagements(t, db, practiceID, 4)

	tx := practiceTx(t, db, practiceID)
	for i, engagementID := range engagements {
		if err := billing.ConsumeCredit(t.Context(), tx, practiceID, engagementID); err != nil {
			t.Fatalf("consume %d: %v", i, err)
		}
	}

	// Counted rather than ordered: every row written in one transaction
	// shares that transaction's timestamp, so which draw came first is
	// not recoverable from the rows. What the counts prove is the thing
	// FIFO is for -- the grant was emptied before the purchase was
	// touched at all.
	draws := map[string]int{}
	rows, err := tx.QueryContext(t.Context(),
		`SELECT drawn_lot_id, count(*) FROM credit_ledger WHERE origin = 'consumption' GROUP BY drawn_lot_id`)
	if err != nil {
		t.Fatalf("read draws: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var lotID string
		var n int
		if err := rows.Scan(&lotID, &n); err != nil {
			t.Fatalf("scan draw: %v", err)
		}
		draws[lotID] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate draws: %v", err)
	}
	if draws[grantID] != 3 || draws[purchaseID] != 1 {
		t.Fatalf("draws = %v, want 3 from the grant %s and 1 from the purchase %s", draws, grantID, purchaseID)
	}
}

// TestDormantPractices_FindsBalancesNobodyHasTouched proves the query the
// annual balance notice and the December/January due-diligence mailings
// are worked from: a Practice holding Credits whose Staff have not been
// seen. A Practice with no balance is not on it, and neither is one whose
// Staff were here yesterday.
func TestDormantPractices_FindsBalancesNobodyHasTouched(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()
	notSeenSince := now.AddDate(-billing.DormancyNoticeYears, 0, 0)

	dormantID := testdb.SeedPractice(t, db, "Dormant")
	seedPurchase(t, db, dormantID, 4, 2000, 0, "pi_dormant", now.AddDate(-3, 0, 0))
	seedStaffLastActive(t, db, dormantID, "dormant-uid", now.AddDate(-3, 0, 0))

	activeID := testdb.SeedPractice(t, db, "Active")
	seedPurchase(t, db, activeID, 4, 2000, 0, "pi_active", now.AddDate(-3, 0, 0))
	seedStaffLastActive(t, db, activeID, "active-uid", now.AddDate(0, 0, -1))

	spentID := testdb.SeedPractice(t, db, "No Balance")
	seedStaffLastActive(t, db, spentID, "spent-uid", now.AddDate(-3, 0, 0))

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	dormant, err := billing.DormantPractices(t.Context(), tx, notSeenSince)
	if err != nil {
		t.Fatalf("DormantPractices: %v", err)
	}
	if len(dormant) != 1 || dormant[0].PracticeID != dormantID || dormant[0].Balance != 4 {
		t.Fatalf("dormant = %+v, want only the dormant Practice with 4 Credits", dormant)
	}
	if dormant[0].LastContactAt == nil {
		t.Fatal("dormant Practice reported no last contact, want the seeded one")
	}
}

// TestDormantPractices_CountsAPracticeNobodyHasEverVisited proves a
// Practice that has never signed in is dormant rather than invisible --
// it is exactly the one the notice is for.
func TestDormantPractices_CountsAPracticeNobodyHasEverVisited(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()
	practiceID := testdb.SeedPractice(t, db, "Never Seen")
	seedPurchase(t, db, practiceID, 1, 2000, 0, "pi_never", now.AddDate(-3, 0, 0))

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	dormant, err := billing.DormantPractices(t.Context(), tx, now.AddDate(-billing.DormancyNoticeYears, 0, 0))
	if err != nil {
		t.Fatalf("DormantPractices: %v", err)
	}
	if len(dormant) != 1 || dormant[0].LastContactAt != nil {
		t.Fatalf("dormant = %+v, want one entry with no recorded contact", dormant)
	}
}

// seedStaffLastActive seeds a Staff member of practiceID last seen at
// lastActive -- the durable contact record 00053 added.
func seedStaffLastActive(t *testing.T, db *testdb.DB, practiceID, identityUID string, lastActive time.Time) {
	t.Helper()
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, "employee")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE staff SET last_active_at = $1 WHERE id = $2`, lastActive, staffID); err != nil {
		t.Fatalf("seed last_active_at: %v", err)
	}
}
