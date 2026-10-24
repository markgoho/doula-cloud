package billing_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
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

// movableClock is a clock a test moves by hand across riseStart.
type movableClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *movableClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *movableClock) set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = at
}

// publish records a version of the Terms as published, the row
// POST /api/internal/legal/publish writes (00127).
func publish(t *testing.T, db *testdb.DB, version string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO legal_version_publications (document, version, material, published_by, published_at)
		 VALUES ('terms', $1, true, 'Mark Goho', $2)`,
		version, riseStart.Add(-61*24*time.Hour),
	); err != nil {
		t.Fatalf("seed publication: %v", err)
	}
}

// buyAt starts a purchase of five Credits at the clock's instant and
// returns the Session request the Stripe client was asked to build.
func buyAt(t *testing.T, fake *billing.FakeStripeClient, srv *httptest.Server, session, practiceID string) billing.CheckoutSessionRequest {
	t.Helper()
	before := len(fake.Calls())
	resp := postPurchase(t, srv, session, practiceID, `{"quantity": 5}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("purchase status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	calls := fake.Calls()
	if len(calls) != before+1 {
		t.Fatalf("checkout sessions = %d, want %d", len(calls), before+1)
	}
	return calls[len(calls)-1]
}

func readBalance(t *testing.T, srv *httptest.Server, session, practiceID string) billing.BalanceResponse {
	t.Helper()
	resp := getBalance(t, srv, session, practiceID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("billing read status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out billing.BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode billing read: %v", err)
	}
	return out
}

// TestCheckoutChargesTheOldPriceUntilThePublishedRiseStarts moves the
// clock across the date a published rise starts: a nanosecond before
// it, checkout charges $20.00 under the first version; at midnight UTC
// on the date, $25.00 under the rise. Nothing else changes between the
// two purchases, so nothing else moved the price.
func TestCheckoutChargesTheOldPriceUntilThePublishedRiseStarts(t *testing.T) {
	db := testdb.New(t)
	const uid = "price-rise-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	publish(t, db, riseDate)

	fake := billing.NewFakeStripeClient()
	clk := &movableClock{at: riseStart.Add(-time.Nanosecond)}
	srv, session := newBillingServerAt(t, db, uid, fake, termsWithRise, clk.now)
	defer srv.Close()

	got := buyAt(t, fake, srv, session, practiceID)
	if got.UnitAmountCents != 2000 || got.TermsVersion != firstVersion {
		t.Fatalf("before the date: charged %d cents under %q, want 2000 under 2026-09-29", got.UnitAmountCents, got.TermsVersion)
	}

	clk.set(riseStart)
	got = buyAt(t, fake, srv, session, practiceID)
	if got.UnitAmountCents != 2500 || got.TermsVersion != riseDate {
		t.Fatalf("on the date: charged %d cents under %q, want 2500 under %s", got.UnitAmountCents, got.TermsVersion, riseDate)
	}
}

// TestCheckoutKeepsTheOldPriceWhenTheRiseWasNeverPublished: a rise in
// the history that was never published emailed nobody, so after its
// date checkout still charges the old price, and the Billing screen
// shows no change coming before it.
func TestCheckoutKeepsTheOldPriceWhenTheRiseWasNeverPublished(t *testing.T) {
	db := testdb.New(t)
	const uid = "price-rise-unpublished-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	fake := billing.NewFakeStripeClient()
	clk := &movableClock{at: riseStart.Add(-24 * time.Hour)}
	srv, session := newBillingServerAt(t, db, uid, fake, termsWithRise, clk.now)
	defer srv.Close()

	if out := readBalance(t, srv, session, practiceID); out.ComingPrice != nil {
		t.Fatalf("comingPrice = %+v before an unpublished rise, want none", out.ComingPrice)
	}

	clk.set(riseStart.Add(24 * time.Hour))
	// Two days on, the first session has expired; sign in again then.
	session = authntest.SeedSessionAt(t, db.App, uid, clk.now())
	got := buyAt(t, fake, srv, session, practiceID)
	if got.UnitAmountCents != 2000 || got.TermsVersion != firstVersion {
		t.Fatalf("after an unpublished rise: charged %d cents under %q, want 2000 under 2026-09-29", got.UnitAmountCents, got.TermsVersion)
	}
	if out := readBalance(t, srv, session, practiceID); out.Price.UnitAmountCents != 2000 {
		t.Fatalf("price after an unpublished rise = %+v, want 2000", out.Price)
	}
}

// TestBillingShowsTodaysPriceAndTheComingOne: while a published rise is
// coming, the read gives today's price and the new one with its date;
// from the date, the new price alone.
func TestBillingShowsTodaysPriceAndTheComingOne(t *testing.T) {
	db := testdb.New(t)
	const uid = "price-rise-billing-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	publish(t, db, riseDate)

	clk := &movableClock{at: riseStart.Add(-time.Nanosecond)}
	srv, session := newBillingServerAt(t, db, uid, billing.NewFakeStripeClient(), termsWithRise, clk.now)
	defer srv.Close()

	out := readBalance(t, srv, session, practiceID)
	if out.Price != (billing.CreditPrice{UnitAmountCents: 2000, Currency: usd}) {
		t.Fatalf("price before the date = %+v, want {2000 usd}", out.Price)
	}
	want := billing.ComingCreditPrice{UnitAmountCents: 2500, Currency: usd, Effective: riseDate}
	if out.ComingPrice == nil || *out.ComingPrice != want {
		t.Fatalf("comingPrice before the date = %+v, want %+v", out.ComingPrice, want)
	}

	clk.set(riseStart)
	out = readBalance(t, srv, session, practiceID)
	if out.Price.UnitAmountCents != 2500 || out.ComingPrice != nil {
		t.Fatalf("on the date: price = %+v, comingPrice = %+v, want 2500 and none", out.Price, out.ComingPrice)
	}
}

// TestALowerPriceNeedsNoPublication: the Terms promise notice of a rise,
// not of a fall, so a version that lowers the price is shown as coming
// and charged from its date whether or not it was published.
func TestALowerPriceNeedsNoPublication(t *testing.T) {
	db := testdb.New(t)
	const uid = "price-fall-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	terms := legal.Document{Name: "Terms of Service", Path: "/terms", Versions: []legal.Version{
		{Effective: firstVersion, Change: "First version.", CreditPriceCents: 2000},
		{Effective: riseDate, Material: true, Change: "A Credit costs $15.00.", CreditPriceCents: 1500},
	}}

	fake := billing.NewFakeStripeClient()
	clk := &movableClock{at: riseStart.Add(-time.Nanosecond)}
	srv, session := newBillingServerAt(t, db, uid, fake, terms, clk.now)
	defer srv.Close()

	out := readBalance(t, srv, session, practiceID)
	if out.ComingPrice == nil || out.ComingPrice.UnitAmountCents != 1500 {
		t.Fatalf("comingPrice = %+v, want 1500", out.ComingPrice)
	}

	clk.set(riseStart)
	if got := buyAt(t, fake, srv, session, practiceID); got.UnitAmountCents != 1500 {
		t.Fatalf("charged %d cents, want 1500", got.UnitAmountCents)
	}
}

// TestNoVersionInForceRefusesBoth: with no version of the Terms started
// there is no price to charge or show, and both routes say so with a 500
// rather than charging some other amount.
func TestNoVersionInForceRefusesBoth(t *testing.T) {
	db := testdb.New(t)
	const uid = "price-none-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	fake := billing.NewFakeStripeClient()
	early := func() time.Time { return time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC) }
	srv, session := newBillingServerAt(t, db, uid, fake, termsWithRise, early)
	defer srv.Close()

	resp := getBalance(t, srv, session, practiceID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("billing read status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	purchase := postPurchase(t, srv, session, practiceID, `{"quantity": 5}`)
	defer purchase.Body.Close()
	if purchase.StatusCode != http.StatusInternalServerError {
		t.Fatalf("purchase status = %d, want %d", purchase.StatusCode, http.StatusInternalServerError)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("checkout sessions = %d, want none", len(fake.Calls()))
	}
}
