package billing_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/testdb"
)

// errStripeUnavailable stands in for any failure of the Stripe call a
// refund makes -- what the ledger does about it is the same whichever it
// is.
var errStripeUnavailable = errors.New("stripe unavailable")

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
		ids[i] = seedClientEngagement(t, db, practiceID)
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
// one test: a Practice that bought at two different prices and spent some
// of what she bought. Because consumption is FIFO, the Credits she spent
// are the cheap ones, and what she still holds is priced at what the
// later purchase actually cost -- computed from credit_ledger with no
// call to Stripe.
func TestRefundable_PricesTheUnspentBalanceFromTheLedgerAlone(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Two Prices")
	now := time.Now()
	seedPurchase(t, db, practiceID, 4, 500, 40, "pi_old", now.AddDate(0, -6, 0))
	seedPurchase(t, db, practiceID, 3, 2000, 165, "pi_new", now.AddDate(0, -1, 0))

	// Seeded before the transaction opens: ConsumeCredit locks the
	// Practice row, and seeding an Engagement needs that row too, so
	// seeding inside the transaction would wait on it forever.
	engagements := seedEngagements(t, db, practiceID, 5)

	tx := practiceTx(t, db, practiceID)
	for i, engagementID := range engagements {
		if err := billing.ConsumeCredit(t.Context(), tx, practiceID, engagementID); err != nil {
			t.Fatalf("consume %d: %v", i, err)
		}
	}

	quote, err := billing.Refundable(t.Context(), tx, practiceID, now)
	if err != nil {
		t.Fatalf("Refundable: %v", err)
	}
	// Four cheap Credits and one dear one are gone, so two $20.00
	// Credits remain: $40.00, plus two thirds of the $1.65 charged on
	// that lot, rounded down to the cent already returned as spent.
	if quote.Credits != 2 || quote.AmountCents != 4000 {
		t.Fatalf("quote = %+v, want 2 credits at 4000 cents", quote)
	}
	if quote.TaxCents != 110 {
		t.Fatalf("quote tax = %d, want 110", quote.TaxCents)
	}
}

// TestConsumeCredit_DrawsTheOldestLotFirst proves the FIFO rule the
// pricing above rests on: a granted Credit is spent before a purchased
// one, and the consumption row names the lot it drew from.
func TestConsumeCredit_DrawsTheOldestLotFirst(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "FIFO")
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

// TestRefund_ReturnsPriceAndTaxAgainstTheOriginalPayment proves the whole
// promise in one pass: the money goes back at the price paid, with the
// tax charged on it, against the PaymentIntent it arrived on -- which is
// what makes Stripe Tax reverse the tax it reported to New York.
func TestRefund_ReturnsPriceAndTaxAgainstTheOriginalPayment(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Refundable")
	lotID := seedPurchase(t, db, practiceID, 5, 2000, 686, "pi_original", time.Now().AddDate(0, -2, 0))
	client := billing.NewFakeStripeClient()

	tx := practiceTx(t, db, practiceID)
	receipt, err := billing.Refund(t.Context(), tx, client, practiceID, "req-full", 5, time.Now())
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if receipt.AmountCents != 10686 || receipt.TaxCents != 686 {
		t.Fatalf("receipt = %+v, want 10686 cents including 686 of tax", receipt)
	}

	calls := client.RefundCalls()
	if len(calls) != 1 || calls[0].PaymentIntentID != "pi_original" || calls[0].AmountCents != 10686 {
		t.Fatalf("stripe refund calls = %+v, want one for pi_original at 10686", calls)
	}

	var quantity, taxCents int
	var drawnLot, refundID string
	if err := tx.QueryRowContext(t.Context(),
		`SELECT quantity, tax_cents, drawn_lot_id, stripe_refund_id FROM credit_ledger WHERE origin = 'refund'`,
	).Scan(&quantity, &taxCents, &drawnLot, &refundID); err != nil {
		t.Fatalf("read refund row: %v", err)
	}
	if quantity != -5 || taxCents != -686 || drawnLot != lotID || refundID != receipt.StripeRefundID {
		t.Fatalf("refund row = (%d, %d, %s, %s), want (-5, -686, %s, %s)",
			quantity, taxCents, drawnLot, refundID, lotID, receipt.StripeRefundID)
	}

	balance, err := billing.Balance(t.Context(), tx, practiceID)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance != 0 {
		t.Fatalf("balance after full refund = %d, want 0", balance)
	}
}

// TestRefund_PartialRefundsReturnExactlyTheTaxCharged proves the
// cumulative tax share: three refunds of one Credit each return, between
// them, every cent of tax the lot was charged and not one more -- which a
// per-refund third of $1.00 would not.
func TestRefund_PartialRefundsReturnExactlyTheTaxCharged(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Partial")
	seedPurchase(t, db, practiceID, 3, 2000, 100, "pi_partial", time.Now())
	client := billing.NewFakeStripeClient()

	tx := practiceTx(t, db, practiceID)
	total := int64(0)
	for i := range 3 {
		receipt, err := billing.Refund(t.Context(), tx, client, practiceID, fmt.Sprintf("req-partial-%d", i), 1, time.Now())
		if err != nil {
			t.Fatalf("refund %d: %v", i, err)
		}
		total += receipt.TaxCents
	}
	if total != 100 {
		t.Fatalf("tax returned across three refunds = %d, want 100", total)
	}
}

// TestRefund_RefusesCreditsGivenFreeOfCharge holds /support to its word:
// the promise covers Credits a Practice "has purchased", and a grant is
// not one. It is refused rather than refunded at $0.00.
func TestRefund_RefusesCreditsGivenFreeOfCharge(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Granted Only")
	seedSignupBonus(t, db, practiceID)

	tx := practiceTx(t, db, practiceID)
	_, err := billing.Refund(t.Context(), tx, billing.NewFakeStripeClient(), practiceID, "req-refused", 1, time.Now())
	if !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("Refund err = %v, want ErrNothingRefundable", err)
	}
}

// TestRefund_RefusesCreditsAlreadySpent proves a Credit that started an
// Engagement has been used, and cannot also be given back.
func TestRefund_RefusesCreditsAlreadySpent(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "All Spent")
	seedPurchase(t, db, practiceID, 1, 2000, 0, "pi_spent", time.Now())

	engagementID := seedClientEngagement(t, db, practiceID)

	tx := practiceTx(t, db, practiceID)
	if err := billing.ConsumeCredit(t.Context(), tx, practiceID, engagementID); err != nil {
		t.Fatalf("consume: %v", err)
	}
	_, err := billing.Refund(t.Context(), tx, billing.NewFakeStripeClient(), practiceID, "req-refused", 1, time.Now())
	if !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("Refund err = %v, want ErrNothingRefundable", err)
	}
}

// TestRefund_RefusesAPurchaseOlderThanTheWindow proves the three-year
// window /support publishes is enforced against the purchase's own date.
func TestRefund_RefusesAPurchaseOlderThanTheWindow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Stale")
	now := time.Now()
	seedPurchase(t, db, practiceID, 2, 2000, 0, "pi_stale", now.AddDate(-billing.RefundWindowYears, 0, -1))

	tx := practiceTx(t, db, practiceID)
	if _, err := billing.Refund(t.Context(), tx, billing.NewFakeStripeClient(), practiceID, "req-stale", 1, now); !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("Refund err = %v, want ErrNothingRefundable", err)
	}
	quote, err := billing.Refundable(t.Context(), tx, practiceID, now)
	if err != nil {
		t.Fatalf("Refundable: %v", err)
	}
	if quote.Credits != 0 {
		t.Fatalf("quote = %+v, want nothing refundable", quote)
	}
	// The Credits themselves have not expired -- only the right to cash.
	balance, err := billing.Balance(t.Context(), tx, practiceID)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if balance != 2 {
		t.Fatalf("balance = %d, want the 2 Credits still spendable", balance)
	}
}

// TestRefund_RefusesMoreThanTheLotHolds proves a refund never spans two
// purchases: one refund reverses one payment.
func TestRefund_RefusesMoreThanTheLotHolds(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Two Lots")
	now := time.Now()
	seedPurchase(t, db, practiceID, 2, 500, 0, "pi_first", now.AddDate(0, -2, 0))
	seedPurchase(t, db, practiceID, 2, 2000, 0, "pi_second", now.AddDate(0, -1, 0))

	tx := practiceTx(t, db, practiceID)
	if _, err := billing.Refund(t.Context(), tx, billing.NewFakeStripeClient(), practiceID, "req-too-many", 3, now); !errors.Is(err, billing.ErrRefundExceedsLot) {
		t.Fatalf("Refund err = %v, want ErrRefundExceedsLot", err)
	}
}

// TestRefund_RefusesQuantityBelowOne proves a refund of nothing is a
// programming error, not a no-op that calls Stripe.
func TestRefund_RefusesQuantityBelowOne(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Zero")
	tx := practiceTx(t, db, practiceID)
	if _, err := billing.Refund(t.Context(), tx, billing.NewFakeStripeClient(), practiceID, "req-zero", 0, time.Now()); err == nil {
		t.Fatal("Refund accepted a quantity of 0")
	}
}

// TestRefund_StripeFailureWritesNoLedgerRow proves the ledger follows the
// money: if Stripe refuses the refund, nothing claims one happened.
func TestRefund_StripeFailureWritesNoLedgerRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Stripe Down")
	seedPurchase(t, db, practiceID, 2, 2000, 0, "pi_down", time.Now())
	client := billing.NewFakeStripeClient()
	client.RefundPaymentErr = errStripeUnavailable

	tx := practiceTx(t, db, practiceID)
	if _, err := billing.Refund(t.Context(), tx, client, practiceID, "req-stripe-down", 1, time.Now()); err == nil {
		t.Fatal("Refund reported success on a failed Stripe call")
	}
	var rows int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM credit_ledger WHERE origin = 'refund'`).Scan(&rows); err != nil {
		t.Fatalf("count refund rows: %v", err)
	}
	if rows != 0 {
		t.Fatalf("refund rows = %d, want none", rows)
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

	dormantID := seedPractice(t, db, "Dormant")
	seedPurchase(t, db, dormantID, 4, 2000, 0, "pi_dormant", now.AddDate(-3, 0, 0))
	seedStaffLastActive(t, db, dormantID, "dormant-uid", now.AddDate(-3, 0, 0))

	activeID := seedPractice(t, db, "Active")
	seedPurchase(t, db, activeID, 4, 2000, 0, "pi_active", now.AddDate(-3, 0, 0))
	seedStaffLastActive(t, db, activeID, "active-uid", now.AddDate(0, 0, -1))

	spentID := seedPractice(t, db, "No Balance")
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
	practiceID := seedPractice(t, db, "Never Seen")
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
	staffID := seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID, "{owner}")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE staff SET last_active_at = $1 WHERE id = $2`, lastActive, staffID); err != nil {
		t.Fatalf("seed last_active_at: %v", err)
	}
}

// TestRefund_ARetriedRefundIsTheSameRefund proves the endpoint survives
// the failure it will actually meet: a request run by hand that times
// out, and is run again under the same name. The second attempt is
// answered with the refund the first one made -- no second Stripe call,
// and no second negative row.
func TestRefund_ARetriedRefundIsTheSameRefund(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Retried")
	seedPurchase(t, db, practiceID, 4, 2000, 200, "pi_retried", time.Now())
	client := billing.NewFakeStripeClient()

	tx := practiceTx(t, db, practiceID)
	first, err := billing.Refund(t.Context(), tx, client, practiceID, "req-retried", 1, time.Now())
	if err != nil {
		t.Fatalf("first refund: %v", err)
	}
	second, err := billing.Refund(t.Context(), tx, client, practiceID, "req-retried", 1, time.Now())
	if err != nil {
		t.Fatalf("retried refund: %v", err)
	}
	if second != first {
		t.Fatalf("retry returned %+v, want the first refund %+v", second, first)
	}

	if calls := client.RefundCalls(); len(calls) != 1 {
		t.Fatalf("stripe refund calls = %+v, want only the first", calls)
	}
	var rows, balance int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*), COALESCE(SUM(quantity), 0) FROM credit_ledger WHERE origin = 'refund'`,
	).Scan(&rows, &balance); err != nil {
		t.Fatalf("count refund rows: %v", err)
	}
	if rows != 1 || balance != -1 {
		t.Fatalf("refund rows = %d totalling %d, want one row of -1", rows, balance)
	}
}

// TestRefund_ADifferentRequestIsADifferentRefund proves the key is not so
// blunt that it blocks a Practice asking for the rest of her money: a
// second, deliberate request carries its own name and is honoured.
func TestRefund_ADifferentRequestIsADifferentRefund(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Twice Over")
	seedPurchase(t, db, practiceID, 4, 2000, 0, "pi_twice", time.Now())
	client := billing.NewFakeStripeClient()

	tx := practiceTx(t, db, practiceID)
	for i := range 2 {
		if _, err := billing.Refund(t.Context(), tx, client, practiceID, fmt.Sprintf("req-twice-%d", i), 1, time.Now()); err != nil {
			t.Fatalf("refund %d: %v", i, err)
		}
	}

	if calls := client.RefundCalls(); len(calls) != 2 {
		t.Fatalf("stripe refund calls = %+v, want two", calls)
	}
}

// TestRefund_TheSameStripeRefundIsRecordedOnce proves the second guard,
// from the other side: two differently-named requests that Stripe answers
// with one Refund -- what a replayed idempotency key looks like when the
// first attempt's transaction never committed -- record one row, not two.
func TestRefund_TheSameStripeRefundIsRecordedOnce(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "One Stripe Refund")
	seedPurchase(t, db, practiceID, 4, 2000, 0, "pi_once", time.Now())
	client := billing.NewFakeStripeClient()
	client.ReplayedRefundID = "re_replayed"

	tx := practiceTx(t, db, practiceID)
	if _, err := billing.Refund(t.Context(), tx, client, practiceID, "req-once-a", 1, time.Now()); err != nil {
		t.Fatalf("first refund: %v", err)
	}
	if _, err := billing.Refund(t.Context(), tx, client, practiceID, "req-once-b", 1, time.Now()); err == nil {
		t.Fatal("one Stripe Refund was recorded twice")
	}
}
