package billing

import "testing"

// TestSplitPurchase_ApportionsAndAlwaysSumsToTheTotal proves the New York
// share is the headcount ratio, that it rounds in New York's favour, and
// that the two shares never lose or invent a cent.
func TestSplitPurchase_ApportionsAndAlwaysSumsToTheTotal(t *testing.T) {
	cases := []struct {
		name          string
		total         int64
		newYork       int
		staff         int
		wantNewYork   int64
		wantRemainder int64
	}{
		// The pair #389 proved against /v1/tax/calculations: $100.00
		// split 6-of-7 is $85.72 taxable, and 8% of that is the $6.86
		// New York is owed -- not 8% of the whole.
		{"six of seven", 10000, 6, 7, 8572, 1428},
		{"all in new york", 10000, 4, 4, 10000, 0},
		{"none in new york", 10000, 0, 4, 0, 10000},
		{"half", 10000, 1, 2, 5000, 5000},
		// A third of a cent goes to New York rather than away from it.
		{"rounds up to new york", 100, 1, 3, 34, 66},
		{"single staff member outside new york", 500, 0, 1, 0, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitPurchase(tc.total, tc.newYork, tc.staff)
			if got.NewYorkCents != tc.wantNewYork || got.RemainderCents != tc.wantRemainder {
				t.Fatalf("SplitPurchase(%d, %d, %d) = %+v, want {%d %d}",
					tc.total, tc.newYork, tc.staff, got, tc.wantNewYork, tc.wantRemainder)
			}
			if got.NewYorkCents+got.RemainderCents != tc.total {
				t.Fatalf("shares sum to %d, want %d", got.NewYorkCents+got.RemainderCents, tc.total)
			}
		})
	}
}

const (
	testPriceID   = "price_credit"
	testProductID = "prod_credit"
	testCurrency  = "usd"
)

func testPricing() creditPricing {
	return creditPricing{
		priceID:         testPriceID,
		productID:       testProductID,
		currency:        testCurrency,
		unitAmountCents: 500,
	}
}

// TestCreditLineItems_WhollyNewYorkPracticeGetsTheOrdinaryPrice proves a
// Practice with no out-of-state Staff is billed the configured Price
// times a quantity, so her Checkout page is the one that existed before
// apportionment.
func TestCreditLineItems_WhollyNewYorkPracticeGetsTheOrdinaryPrice(t *testing.T) {
	items := creditLineItems(testPricing(), CheckoutSessionRequest{Quantity: 5, NewYorkStaff: 3, TotalStaff: 3})

	if len(items) != 1 {
		t.Fatalf("line items = %d, want 1", len(items))
	}
	if items[0].Price == nil || *items[0].Price != testPriceID {
		t.Fatalf("line item price = %v, want price_credit", items[0].Price)
	}
	if *items[0].Quantity != 5 {
		t.Fatalf("quantity = %d, want 5", *items[0].Quantity)
	}
	if items[0].PriceData != nil {
		t.Fatal("wholly New York purchase should not build an inline price")
	}
}

// TestCreditLineItems_MixedPracticeSplitsAcrossTwoTaxCodes proves a
// Practice whose Staff straddle the state line is billed as a taxable New
// York share plus a nontaxable remainder, and that the two still add up to
// what the credits cost.
func TestCreditLineItems_MixedPracticeSplitsAcrossTwoTaxCodes(t *testing.T) {
	items := creditLineItems(testPricing(), CheckoutSessionRequest{Quantity: 20, NewYorkStaff: 6, TotalStaff: 7})

	if len(items) != 2 {
		t.Fatalf("line items = %d, want 2", len(items))
	}

	taxable := items[0].PriceData
	if taxable == nil || taxable.Product == nil || *taxable.Product != testProductID {
		t.Fatalf("taxable line = %+v, want the credit product", taxable)
	}
	if taxable.ProductData != nil {
		t.Fatal("the taxable line must take its tax code from the credit Product, not restate one")
	}
	if *taxable.UnitAmount != 8572 {
		t.Fatalf("taxable amount = %d, want 8572", *taxable.UnitAmount)
	}
	if *taxable.TaxBehavior != "exclusive" {
		t.Fatalf("taxable tax behavior = %q, want exclusive", *taxable.TaxBehavior)
	}
	if *taxable.Currency != testCurrency {
		t.Fatalf("taxable currency = %q, want usd", *taxable.Currency)
	}

	remainder := items[1].PriceData
	if remainder == nil || remainder.ProductData == nil {
		t.Fatalf("remainder line = %+v, want an inline nontaxable product", remainder)
	}
	if *remainder.ProductData.TaxCode != nontaxableTaxCode {
		t.Fatalf("remainder tax code = %q, want %q", *remainder.ProductData.TaxCode, nontaxableTaxCode)
	}
	if *remainder.ProductData.Name != remainderProductName {
		t.Fatalf("remainder name = %q, want %q", *remainder.ProductData.Name, remainderProductName)
	}
	if *remainder.UnitAmount != 1428 {
		t.Fatalf("remainder amount = %d, want 1428", *remainder.UnitAmount)
	}
	if *remainder.TaxBehavior != "exclusive" {
		t.Fatalf("remainder tax behavior = %q, want exclusive", *remainder.TaxBehavior)
	}
	if *taxable.UnitAmount+*remainder.UnitAmount != 10000 {
		t.Fatalf("line items sum to %d, want 10000", *taxable.UnitAmount+*remainder.UnitAmount)
	}
	if *items[0].Quantity != 1 || *items[1].Quantity != 1 {
		t.Fatal("an apportioned line carries the whole share as its unit amount, quantity 1")
	}
}

// TestCreditLineItems_PracticeWithNoNewYorkStaffPaysInFullAndIsTaxedOnNothing
// proves the wholly out-of-state Practice is charged the full price of her
// credits on a single nontaxable line -- not billed the configured Price,
// which would have New York's tax code on it and would tax the lot if she
// happened to give a New York billing address.
func TestCreditLineItems_PracticeWithNoNewYorkStaffPaysInFullAndIsTaxedOnNothing(t *testing.T) {
	items := creditLineItems(testPricing(), CheckoutSessionRequest{Quantity: 4, NewYorkStaff: 0, TotalStaff: 4})

	if len(items) != 1 {
		t.Fatalf("line items = %d, want 1", len(items))
	}
	line := items[0].PriceData
	if line == nil || line.ProductData == nil {
		t.Fatalf("line = %+v, want an inline nontaxable product", line)
	}
	if items[0].Price != nil {
		t.Fatal("a wholly out-of-state purchase must not use the taxable credit Price")
	}
	if *line.ProductData.TaxCode != nontaxableTaxCode {
		t.Fatalf("tax code = %q, want %q", *line.ProductData.TaxCode, nontaxableTaxCode)
	}
	if *line.UnitAmount != 2000 {
		t.Fatalf("amount = %d, want the full 2000", *line.UnitAmount)
	}
}

// TestCreditLineItems_RemainderTooSmallToBillIsDropped proves a share that
// rounds away to nothing produces no line item at all, rather than a
// zero-amount one Stripe would reject.
func TestCreditLineItems_RemainderTooSmallToBillIsDropped(t *testing.T) {
	pricing := creditPricing{priceID: testPriceID, productID: testProductID, currency: testCurrency, unitAmountCents: 3}
	items := creditLineItems(pricing, CheckoutSessionRequest{Quantity: 1, NewYorkStaff: 99, TotalStaff: 100})

	if len(items) != 1 {
		t.Fatalf("line items = %d, want 1", len(items))
	}
	if *items[0].PriceData.UnitAmount != 3 {
		t.Fatalf("amount = %d, want 3", *items[0].PriceData.UnitAmount)
	}
	if items[0].PriceData.Product == nil {
		t.Fatal("the surviving line should be the taxable one")
	}
}
