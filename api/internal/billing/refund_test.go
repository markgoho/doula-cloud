package billing_test

import (
	"context"
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

// opsTx opens a transaction as ops_runtime, the one role a refund runs
// as (ADR-0062).
func opsTx(t *testing.T, db *testdb.DB) *sql.Tx {
	t.Helper()
	tx, err := db.Ops.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

// refunder issues refunds on one ops_runtime transaction, each under the
// act it records.
type refunder struct {
	t      *testing.T
	db     *testdb.DB
	tx     *sql.Tx
	client *billing.FakeStripeClient
	actID  string
	quotes []billing.RefundQuote
}

func newRefunder(t *testing.T, db *testdb.DB, client *billing.FakeStripeClient) *refunder {
	t.Helper()
	return &refunder{t: t, db: db, tx: opsTx(t, db), client: client, actID: testdb.SeedOperatorAct(t, db, "Mark Goho")}
}

func (r *refunder) refund(practiceID, key string, credits int, now time.Time) (billing.RefundReceipt, error) {
	r.t.Helper()
	return billing.Refund(r.t.Context(), r.tx, r.client, //nolint:wrapcheck // test helper; callers match the error with errors.Is
		billing.RefundOrder{PracticeID: practiceID, Credits: credits, RequestKey: key}, now,
		func(_ context.Context, q billing.RefundQuote) (string, error) {
			r.quotes = append(r.quotes, q)
			return r.actID, nil
		})
}

func (r *refunder) commit() {
	r.t.Helper()
	if err := r.tx.Commit(); err != nil {
		r.t.Fatalf("commit: %v", err)
	}
}

// refundRows counts the refund rows at practiceID and their total.
func refundRows(t *testing.T, db *testdb.DB, practiceID string) (rows, total int) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), COALESCE(SUM(quantity), 0) FROM credit_ledger WHERE origin = 'refund' AND practice_id = $1`, practiceID,
	).Scan(&rows, &total); err != nil {
		t.Fatalf("count refund rows: %v", err)
	}
	return rows, total
}

func consume(t *testing.T, db *testdb.DB, practiceID string, engagements []string) {
	t.Helper()
	tx := practiceTx(t, db, practiceID)
	for i, engagementID := range engagements {
		if err := billing.ConsumeCredit(t.Context(), tx, practiceID, engagementID); err != nil {
			t.Fatalf("consume %d: %v", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// TestQuoteRefund_PricesTheUnspentBalanceFromTheLedgerAlone: a Practice
// that bought at two different prices and spent some of what she
// bought. Because consumption is FIFO, the Credits she spent are the
// cheap ones, and what she still holds is priced at what the later
// purchase actually cost -- computed from credit_ledger with no call to
// Stripe.
func TestQuoteRefund_PricesTheUnspentBalanceFromTheLedgerAlone(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Two Prices")
	now := time.Now()
	seedPurchase(t, db, practiceID, 4, 500, 40, "pi_old", now.AddDate(0, -6, 0))
	seedPurchase(t, db, practiceID, 3, 2000, 165, "pi_new", now.AddDate(0, -1, 0))
	consume(t, db, practiceID, seedEngagements(t, db, practiceID, 5))

	quote, err := billing.QuoteRefund(t.Context(), opsTx(t, db), practiceID, 0, now)
	if err != nil {
		t.Fatalf("QuoteRefund: %v", err)
	}
	// Four cheap Credits and one dear one are gone, so two $20.00
	// Credits remain: $40.00, plus two thirds of the $1.65 charged on
	// that lot, rounded down.
	if quote.Credits != 2 || quote.MaxCredits != 2 || quote.AmountCents != 4110 || quote.TaxCents != 110 {
		t.Fatalf("quote = %+v, want 2 of 2 credits at 4110 cents including 110 of tax", quote)
	}
}

func TestQuoteRefund_PricesFewerCreditsThanTheLotHolds(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Part Of A Lot")
	seedPurchase(t, db, practiceID, 4, 2000, 200, "pi_part", time.Now())

	quote, err := billing.QuoteRefund(t.Context(), opsTx(t, db), practiceID, 1, time.Now())
	if err != nil {
		t.Fatalf("QuoteRefund: %v", err)
	}
	if quote.Credits != 1 || quote.MaxCredits != 4 || quote.AmountCents != 2050 || quote.TaxCents != 50 {
		t.Fatalf("quote = %+v, want 1 of 4 credits at 2050 cents including 50 of tax", quote)
	}
}

// TestRefund_ReturnsPriceAndTaxAgainstTheOriginalPayment proves the whole
// promise in one pass: the money goes back at the price paid, with the
// tax charged on it, against the PaymentIntent it arrived on -- which is
// what makes Stripe Tax reverse the tax it reported to New York -- and
// the ledger row keys the act that took it.
func TestRefund_ReturnsPriceAndTaxAgainstTheOriginalPayment(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Refundable")
	lotID := seedPurchase(t, db, practiceID, 5, 2000, 686, "pi_original", time.Now().AddDate(0, -2, 0))
	client := billing.NewFakeStripeClient()
	r := newRefunder(t, db, client)
	at := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)

	receipt, err := r.refund(practiceID, "req-full", 5, at)
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	r.commit()
	if receipt.Credits != 5 || receipt.AmountCents != 10686 || receipt.TaxCents != 686 || receipt.PaymentIntentID != "pi_original" {
		t.Fatalf("receipt = %+v, want 5 credits, 10686 cents including 686 of tax, on pi_original", receipt)
	}
	if len(r.quotes) != 1 || r.quotes[0].AmountCents != 10686 {
		t.Fatalf("acts recorded = %+v, want one at 10686 cents", r.quotes)
	}

	calls := client.RefundCalls()
	if len(calls) != 1 || calls[0].PaymentIntentID != "pi_original" || calls[0].AmountCents != 10686 || calls[0].IdempotencyKey != "req-full" {
		t.Fatalf("stripe refund calls = %+v, want one for pi_original at 10686 under req-full", calls)
	}

	var quantity, taxCents int
	var drawnLot, refundID, actID string
	var createdAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT quantity, tax_cents, drawn_lot_id, stripe_refund_id, operator_act_id, created_at FROM credit_ledger WHERE origin = 'refund'`,
	).Scan(&quantity, &taxCents, &drawnLot, &refundID, &actID, &createdAt); err != nil {
		t.Fatalf("read refund row: %v", err)
	}
	if quantity != -5 || taxCents != -686 || drawnLot != lotID || refundID != receipt.StripeRefundID || actID != r.actID || !createdAt.Equal(at) {
		t.Fatalf("refund row = (%d, %d, %s, %s, act %s, %s), want (-5, -686, %s, %s, act %s, %s)",
			quantity, taxCents, drawnLot, refundID, actID, createdAt, lotID, receipt.StripeRefundID, r.actID, at)
	}

	balance, err := billing.Balance(t.Context(), practiceTx(t, db, practiceID), practiceID)
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
	practiceID := testdb.SeedPractice(t, db, "Partial")
	seedPurchase(t, db, practiceID, 3, 2000, 100, "pi_partial", time.Now())
	r := newRefunder(t, db, billing.NewFakeStripeClient())

	total := int64(0)
	for i := range 3 {
		receipt, err := r.refund(practiceID, fmt.Sprintf("req-partial-%d", i), 1, time.Now())
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
	practiceID := testdb.SeedPractice(t, db, "Granted Only")
	seedSignupBonus(t, db, practiceID)
	client := billing.NewFakeStripeClient()

	_, err := newRefunder(t, db, client).refund(practiceID, "req-refused", 1, time.Now())
	if !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("Refund err = %v, want ErrNothingRefundable", err)
	}
	if calls := client.RefundCalls(); len(calls) != 0 {
		t.Fatalf("stripe refund calls = %+v, want none", calls)
	}
}

// TestRefund_RefusesCreditsAlreadySpent proves a Credit that started an
// Engagement has been used, and cannot also be given back.
func TestRefund_RefusesCreditsAlreadySpent(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "All Spent")
	seedPurchase(t, db, practiceID, 1, 2000, 0, "pi_spent", time.Now())
	consume(t, db, practiceID, seedEngagements(t, db, practiceID, 1))

	_, err := newRefunder(t, db, billing.NewFakeStripeClient()).refund(practiceID, "req-refused", 1, time.Now())
	if !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("Refund err = %v, want ErrNothingRefundable", err)
	}
}

// TestRefund_RefusesAPurchaseOlderThanTheWindow proves the three-year
// window /support publishes is enforced against the purchase's own date.
func TestRefund_RefusesAPurchaseOlderThanTheWindow(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Stale")
	now := time.Now()
	seedPurchase(t, db, practiceID, 2, 2000, 0, "pi_stale", now.AddDate(-billing.RefundWindowYears, 0, -1))

	r := newRefunder(t, db, billing.NewFakeStripeClient())
	if _, err := r.refund(practiceID, "req-stale", 1, now); !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("Refund err = %v, want ErrNothingRefundable", err)
	}
	if _, err := billing.QuoteRefund(t.Context(), r.tx, practiceID, 0, now); !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("QuoteRefund err = %v, want ErrNothingRefundable", err)
	}
	// The Credits themselves have not expired -- only the right to cash.
	balance, err := billing.Balance(t.Context(), practiceTx(t, db, practiceID), practiceID)
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
	practiceID := testdb.SeedPractice(t, db, "Two Lots")
	now := time.Now()
	seedPurchase(t, db, practiceID, 2, 500, 0, "pi_first", now.AddDate(0, -2, 0))
	seedPurchase(t, db, practiceID, 2, 2000, 0, "pi_second", now.AddDate(0, -1, 0))

	if _, err := newRefunder(t, db, billing.NewFakeStripeClient()).refund(practiceID, "req-too-many", 3, now); !errors.Is(err, billing.ErrRefundExceedsLot) {
		t.Fatalf("Refund err = %v, want ErrRefundExceedsLot", err)
	}
}

// TestRefund_RefusesQuantityBelowOne proves a refund of nothing is a
// programming error, not a no-op that calls Stripe.
func TestRefund_RefusesQuantityBelowOne(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Zero")
	if _, err := newRefunder(t, db, billing.NewFakeStripeClient()).refund(practiceID, "req-zero", 0, time.Now()); err == nil {
		t.Fatal("Refund accepted a quantity of 0")
	}
}

// TestRefund_StripeFailureWritesNoLedgerRow proves the ledger follows the
// money: if Stripe refuses the refund, nothing claims one happened.
func TestRefund_StripeFailureWritesNoLedgerRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Stripe Down")
	seedPurchase(t, db, practiceID, 2, 2000, 0, "pi_down", time.Now())
	client := billing.NewFakeStripeClient()
	client.RefundPaymentErr = errStripeUnavailable
	r := newRefunder(t, db, client)

	if _, err := r.refund(practiceID, "req-stripe-down", 1, time.Now()); !errors.Is(err, errStripeUnavailable) {
		t.Fatalf("Refund err = %v, want the Stripe failure", err)
	}
	r.commit()
	if rows, _ := refundRows(t, db, practiceID); rows != 0 {
		t.Fatalf("refund rows = %d, want none", rows)
	}
}

func TestRefund_AnActThatCannotBeRecordedMovesNoMoney(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "No Act")
	seedPurchase(t, db, practiceID, 2, 2000, 0, "pi_no_act", time.Now())
	client := billing.NewFakeStripeClient()
	errNoAct := errors.New("no act")

	_, err := billing.Refund(t.Context(), opsTx(t, db), client,
		billing.RefundOrder{PracticeID: practiceID, Credits: 1, RequestKey: "req-no-act"}, time.Now(),
		func(context.Context, billing.RefundQuote) (string, error) { return "", errNoAct })
	if !errors.Is(err, errNoAct) {
		t.Fatalf("Refund err = %v, want the act's failure", err)
	}
	if calls := client.RefundCalls(); len(calls) != 0 {
		t.Fatalf("stripe refund calls = %+v, want none", calls)
	}
}

// TestRefund_ARetriedRefundIsTheSameRefund proves the act survives the
// failure it will actually meet: a request that times out and is sent
// again under the same name. The second attempt is answered with the
// refund the first one made -- no second act, no second Stripe call, and
// no second negative row.
func TestRefund_ARetriedRefundIsTheSameRefund(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Retried")
	seedPurchase(t, db, practiceID, 4, 2000, 200, "pi_retried", time.Now())
	client := billing.NewFakeStripeClient()
	r := newRefunder(t, db, client)

	first, err := r.refund(practiceID, "req-retried", 1, time.Now())
	if err != nil {
		t.Fatalf("first refund: %v", err)
	}
	second, err := r.refund(practiceID, "req-retried", 1, time.Now())
	if err != nil {
		t.Fatalf("retried refund: %v", err)
	}
	r.commit()
	if second != first {
		t.Fatalf("retry returned %+v, want the first refund %+v", second, first)
	}
	if len(r.quotes) != 1 {
		t.Fatalf("acts recorded = %d, want only the first", len(r.quotes))
	}
	if calls := client.RefundCalls(); len(calls) != 1 {
		t.Fatalf("stripe refund calls = %+v, want only the first", calls)
	}
	if rows, total := refundRows(t, db, practiceID); rows != 1 || total != -1 {
		t.Fatalf("refund rows = %d totaling %d, want one row of -1", rows, total)
	}
}

// TestRefund_ARequestKeyAtAnotherPracticeIsNotItsRefund proves a key
// answers only for the Practice it was used at.
func TestRefund_ARequestKeyAtAnotherPracticeIsNotItsRefund(t *testing.T) {
	db := testdb.New(t)
	first := testdb.SeedPractice(t, db, "First")
	second := testdb.SeedPractice(t, db, "Second")
	seedPurchase(t, db, first, 1, 2000, 0, "pi_first_practice", time.Now())
	r := newRefunder(t, db, billing.NewFakeStripeClient())
	if _, err := r.refund(first, "req-shared", 1, time.Now()); err != nil {
		t.Fatalf("first refund: %v", err)
	}

	if _, err := r.refund(second, "req-shared", 1, time.Now()); !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("Refund at the second Practice err = %v, want ErrNothingRefundable", err)
	}
}

// TestRefund_ADifferentRequestIsADifferentRefund proves the key is not so
// blunt that it blocks a Practice asking for the rest of her money: a
// second, deliberate request carries its own name and is honored.
func TestRefund_ADifferentRequestIsADifferentRefund(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Twice Over")
	seedPurchase(t, db, practiceID, 4, 2000, 0, "pi_twice", time.Now())
	client := billing.NewFakeStripeClient()
	r := newRefunder(t, db, client)

	for i := range 2 {
		if _, err := r.refund(practiceID, fmt.Sprintf("req-twice-%d", i), 1, time.Now()); err != nil {
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
	practiceID := testdb.SeedPractice(t, db, "One Stripe Refund")
	seedPurchase(t, db, practiceID, 4, 2000, 0, "pi_once", time.Now())
	client := billing.NewFakeStripeClient()
	client.ReplayedRefundID = "re_replayed"
	r := newRefunder(t, db, client)

	if _, err := r.refund(practiceID, "req-once-a", 1, time.Now()); err != nil {
		t.Fatalf("first refund: %v", err)
	}
	if _, err := r.refund(practiceID, "req-once-b", 1, time.Now()); err == nil {
		t.Fatal("one Stripe Refund was recorded twice")
	}
}
