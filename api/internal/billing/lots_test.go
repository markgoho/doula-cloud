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
