package billing_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/testdb"
)

// The price of a Credit at checkout is the one the version of the Terms
// in force states (#1834). termsWithRise is a history whose second
// version raises the price from $20.00 to $25.00 on riseDate.
const (
	riseDate     = "2027-03-01"
	firstVersion = "2026-09-29"
	usd          = "usd"
)

var termsWithRise = legal.Document{Name: "Terms of Service", Path: "/terms", Versions: []legal.Version{
	{Effective: firstVersion, Change: "First version.", CreditPriceCents: 2000},
	{Effective: riseDate, Material: true, Change: "A Credit costs $25.00.", CreditPriceCents: 2500, PriceRise: "Card fees went up."},
}}

// riseStart is the instant the rise starts: midnight UTC on its date,
// the reading legal.Document.InForce gives every version.
var riseStart = time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)

// publish records a version of the Terms as published, the row
// ops's publish act writes (#2486).
func publish(t *testing.T, db *testdb.DB, version string) {
	t.Helper()
	testdb.SeedLegalPublication(t, db, "terms", version, true, riseStart.Add(-61*24*time.Hour))
}

// buyAt starts a purchase of five Credits at the clock's instant and
// returns the Session request the Stripe client was asked to build.
func (e *env) buyAt() billing.CheckoutSessionRequest {
	e.T.Helper()
	before := len(e.Stripe.Calls())
	e.startPurchase(`{"quantity": 5}`, http.StatusOK)
	calls := e.Stripe.Calls()
	if len(calls) != before+1 {
		e.T.Fatalf("checkout sessions = %d, want %d", len(calls), before+1)
	}
	return calls[len(calls)-1]
}

// TestCheckoutChargesTheOldPriceUntilThePublishedRiseStarts moves the
// clock across the date a published rise starts: a nanosecond before
// it, checkout charges $20.00 under the first version; at midnight UTC
// on the date, $25.00 under the rise. Nothing else changes between the
// two purchases, so nothing else moved the price.
func TestCheckoutChargesTheOldPriceUntilThePublishedRiseStarts(t *testing.T) {
	e := newEnv(t, withTerms(termsWithRise), atInstant(riseStart.Add(-time.Nanosecond)))
	publish(t, e.DB, riseDate)

	got := e.buyAt()
	if got.UnitAmountCents != 2000 || got.TermsVersion != firstVersion {
		t.Fatalf("before the date: charged %d cents under %q, want 2000 under 2026-09-29", got.UnitAmountCents, got.TermsVersion)
	}

	e.MoveClockTo(riseStart)
	got = e.buyAt()
	if got.UnitAmountCents != 2500 || got.TermsVersion != riseDate {
		t.Fatalf("on the date: charged %d cents under %q, want 2500 under %s", got.UnitAmountCents, got.TermsVersion, riseDate)
	}
}

// TestCheckoutKeepsTheOldPriceWhenTheRiseWasNeverPublished: a rise in
// the history that was never published emailed nobody, so after its
// date checkout still charges the old price, and the Billing screen
// shows no change coming before it.
func TestCheckoutKeepsTheOldPriceWhenTheRiseWasNeverPublished(t *testing.T) {
	e := newEnv(t, withTerms(termsWithRise), atInstant(riseStart.Add(-24*time.Hour)))

	if out := e.balance(); out.ComingPrice != nil {
		t.Fatalf("comingPrice = %+v before an unpublished rise, want none", out.ComingPrice)
	}

	e.MoveClockTo(riseStart.Add(24 * time.Hour))
	// Two days on, the first session has expired; sign in again then.
	e.SignedIn = e.As(e.Staff)
	got := e.buyAt()
	if got.UnitAmountCents != 2000 || got.TermsVersion != firstVersion {
		t.Fatalf("after an unpublished rise: charged %d cents under %q, want 2000 under 2026-09-29", got.UnitAmountCents, got.TermsVersion)
	}
	if out := e.balance(); out.Price.UnitAmountCents != 2000 {
		t.Fatalf("price after an unpublished rise = %+v, want 2000", out.Price)
	}
}

// TestBillingShowsTodaysPriceAndTheComingOne: while a published rise is
// coming, the read gives today's price and the new one with its date;
// from the date, the new price alone.
func TestBillingShowsTodaysPriceAndTheComingOne(t *testing.T) {
	e := newEnv(t, withTerms(termsWithRise), atInstant(riseStart.Add(-time.Nanosecond)))
	publish(t, e.DB, riseDate)

	out := e.balance()
	if out.Price != (billing.CreditPrice{UnitAmountCents: 2000, Currency: usd}) {
		t.Fatalf("price before the date = %+v, want {2000 usd}", out.Price)
	}
	want := billing.ComingCreditPrice{UnitAmountCents: 2500, Currency: usd, Effective: riseDate}
	if out.ComingPrice == nil || *out.ComingPrice != want {
		t.Fatalf("comingPrice before the date = %+v, want %+v", out.ComingPrice, want)
	}

	e.MoveClockTo(riseStart)
	out = e.balance()
	if out.Price.UnitAmountCents != 2500 || out.ComingPrice != nil {
		t.Fatalf("on the date: price = %+v, comingPrice = %+v, want 2500 and none", out.Price, out.ComingPrice)
	}
}

// TestALowerPriceNeedsNoPublication: the Terms promise notice of a rise,
// not of a fall, so a version that lowers the price is shown as coming
// and charged from its date whether or not it was published.
func TestALowerPriceNeedsNoPublication(t *testing.T) {
	terms := legal.Document{Name: "Terms of Service", Path: "/terms", Versions: []legal.Version{
		{Effective: firstVersion, Change: "First version.", CreditPriceCents: 2000},
		{Effective: riseDate, Material: true, Change: "A Credit costs $15.00.", CreditPriceCents: 1500},
	}}
	e := newEnv(t, withTerms(terms), atInstant(riseStart.Add(-time.Nanosecond)))

	out := e.balance()
	if out.ComingPrice == nil || out.ComingPrice.UnitAmountCents != 1500 {
		t.Fatalf("comingPrice = %+v, want 1500", out.ComingPrice)
	}

	e.MoveClockTo(riseStart)
	if got := e.buyAt(); got.UnitAmountCents != 1500 {
		t.Fatalf("charged %d cents, want 1500", got.UnitAmountCents)
	}
}

// TestNoVersionInForceRefusesBoth: with no version of the Terms started
// there is no price to charge or show, and both routes say so with a 500
// rather than charging some other amount.
func TestNoVersionInForceRefusesBoth(t *testing.T) {
	e := newEnv(t, withTerms(termsWithRise), atInstant(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)))

	e.balanceAt("", http.StatusInternalServerError)
	e.startPurchase(`{"quantity": 5}`, http.StatusInternalServerError)
	if len(e.Stripe.Calls()) != 0 {
		t.Fatalf("checkout sessions = %d, want none", len(e.Stripe.Calls()))
	}
}

// TestFakeStripeClient_CheckCreditPrice: the fake has a Price at every
// amount unless a test says it has none, the answer the publish act
// reads before a version of the Terms states a price (#1834).
func TestFakeStripeClient_CheckCreditPrice(t *testing.T) {
	fake := billing.NewFakeStripeClient()
	if err := fake.CheckCreditPrice(t.Context(), 2500); err != nil {
		t.Fatalf("CheckCreditPrice = %v, want nil", err)
	}
	fake.CheckCreditPriceErr = errStripeFake
	if err := fake.CheckCreditPrice(t.Context(), 2500); !errors.Is(err, errStripeFake) {
		t.Fatalf("CheckCreditPrice = %v, want %v", err, errStripeFake)
	}
}
