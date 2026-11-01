package billing_test

import (
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/testdb"
)

// TestFoundingGrant_IsSpentBeforePurchasedCreditsAndIsNotRefundable
// proves the two consequences #449 said the implementation must not
// break, both of which fall out of the lot being priced at a real $0.00:
// FIFO reaches the grant first, and the refund path passes over it
// instead of computing a $0.00 refund. ops gives the grant (#2482).
func TestFoundingGrant_IsSpentBeforePurchasedCreditsAndIsNotRefundable(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Grant Then Purchase")
	testdb.SeedFoundingGrant(t, db, practiceID, 3, time.Now())
	purchaseID := seedPurchase(t, db, practiceID, 2, 2000, 100, "pi_after_grant", time.Now().Add(time.Hour))

	consume(t, db, practiceID, seedEngagements(t, db, practiceID, 3))

	// Three Credits spent against a three-Credit grant and a two-Credit
	// purchase: FIFO means all three came out of the grant, so the
	// purchase is untouched.
	var drawsOnPurchase int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM credit_ledger WHERE drawn_lot_id = $1`, purchaseID,
	).Scan(&drawsOnPurchase); err != nil {
		t.Fatalf("count draws on purchase: %v", err)
	}
	if drawsOnPurchase != 0 {
		t.Fatalf("draws on the purchase = %d, want 0 -- the grant must be spent first", drawsOnPurchase)
	}

	// And the grant itself is worth nothing back: the quote counts the
	// two purchased Credits only, at the price paid for them.
	quote, err := billing.QuoteRefund(t.Context(), opsTx(t, db), practiceID, 0, time.Now())
	if err != nil {
		t.Fatalf("QuoteRefund: %v", err)
	}
	if quote.Credits != 2 || quote.AmountCents != 4100 {
		t.Fatalf("quote = %+v, want 2 purchased credits at 4100 cents including tax", quote)
	}
}

// TestFoundingGrant_IsAllThatIsRefusedWhenOnlyAGrantIsHeld proves the
// refund path says "nothing refundable" rather than issuing a $0.00
// refund against a lot that was never paid for.
func TestFoundingGrant_IsAllThatIsRefusedWhenOnlyAGrantIsHeld(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Grant Only")
	testdb.SeedFoundingGrant(t, db, practiceID, 3, time.Now())

	client := billing.NewFakeStripeClient()
	if _, err := newRefunder(t, db, client).refund(practiceID, t.Name(), 1, time.Now()); !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("Refund error = %v, want ErrNothingRefundable", err)
	}
	if calls := client.RefundCalls(); len(calls) != 0 {
		t.Fatalf("stripe refund calls = %+v, want none", calls)
	}
}
