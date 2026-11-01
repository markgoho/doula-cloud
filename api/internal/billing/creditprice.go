package billing

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"doula-cloud/api/internal/legal"
)

// creditCurrency is the currency every Credit is priced in: the Terms
// state the price in dollars, and the credit Product's Prices are USD.
const creditCurrency = "usd"

// CreditPrice is what one Credit costs at checkout today, in cents
// before sales tax: the price the version of the Terms in force states
// (#1834). It is never zero and never a float.
type CreditPrice struct {
	UnitAmountCents int64  `json:"unitAmountCents"`
	Currency        string `json:"currency"`
}

// ComingCreditPrice is a change to the price of a Credit that has been
// announced and has not started: the new price, and the date of the
// version of the Terms it starts with.
type ComingCreditPrice struct {
	CreditPrice
	Effective string `json:"effective"`
}

// priceInForce is the price of a Credit at one instant: what checkout
// charges, the version of the Terms that set it, and the change still to
// come, if one has been announced.
type priceInForce struct {
	today        CreditPrice
	termsVersion string
	coming       *ComingCreditPrice
}

// priceAt is the price of a Credit at the instant at, under the versions
// of terms, given which versions have been published through
// ops (#2486).
//
// A version that has started sets the price unless it raises it and
// was never published: an unpublished rise emailed nobody, so the Terms'
// promise ("we email each Owner at least 60 days before, with the
// reason") was not kept, and checkout keeps the old price. The publish
// act itself refuses a rise fewer than 60 days ahead, so a published
// rise always gave the notice. A version that lowers the price or keeps
// it needs no publication.
//
// The change still to come is the first version that has not started
// and would change the price under the same rule: a rise appears only
// once it is published, which is when the Owners are told.
//
// ok is false when no version of terms has started, which legal_test.go
// holds never happens for the shipped Terms.
func priceAt(terms legal.Document, published map[string]bool, at time.Time) (pricing priceInForce, ok bool) {
	coming := terms.Coming(at)
	started := terms.Versions[:len(terms.Versions)-len(coming)]

	var cents int64
	for _, v := range started {
		if !ok || maySetPrice(v, cents, published) {
			cents, pricing.termsVersion, ok = v.CreditPriceCents, v.Effective, true
		}
	}
	if !ok {
		return priceInForce{}, false
	}
	pricing.today = CreditPrice{UnitAmountCents: cents, Currency: creditCurrency}

	for _, v := range coming {
		if v.CreditPriceCents != cents && maySetPrice(v, cents, published) {
			pricing.coming = &ComingCreditPrice{UnitAmountCents: v.CreditPriceCents, Currency: creditCurrency, Effective: v.Effective}
			break
		}
	}
	return pricing, true
}

// maySetPrice answers whether version v may set the price of a Credit when
// it is cents now: always when it does not raise it, and when it does,
// only once a version that gives the reason has been published.
func maySetPrice(v legal.Version, cents int64, published map[string]bool) bool {
	if v.CreditPriceCents <= cents {
		return true
	}
	return v.PriceRise != "" && published[v.Effective]
}

// publishedVersions reads which versions of terms have been published,
// by their dates. legal_version_publications has no row-level security
// and grants SELECT to app_runtime (00127), because that a document
// changes on a date is a public fact.
func publishedVersions(ctx context.Context, tx *sql.Tx, terms legal.Document) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT to_char(version, 'YYYY-MM-DD') FROM legal_version_publications WHERE document = $1`,
		strings.TrimPrefix(terms.Path, "/"),
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: read published versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	published := map[string]bool{}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("billing: scan published version: %w", err)
		}
		published[version] = true
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: iterate published versions: %w", err)
	}
	return published, nil
}

// creditPriceNow is the price of a Credit at now, read in the request's
// transaction.
func creditPriceNow(ctx context.Context, tx *sql.Tx, terms legal.Document, now time.Time) (priceInForce, error) {
	published, err := publishedVersions(ctx, tx, terms)
	if err != nil {
		// coverage:ignore reason: DB failure inside publishedVersions, not exercised by unit tests
		return priceInForce{}, err
	}
	pricing, ok := priceAt(terms, published, now)
	if !ok {
		return priceInForce{}, fmt.Errorf("billing: no version of the %s is in force at %s", terms.Name, now.UTC().Format(time.RFC3339))
	}
	return pricing, nil
}
