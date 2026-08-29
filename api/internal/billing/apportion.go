package billing

import (
	"github.com/stripe/stripe-go/v86"
)

// nontaxableTaxCode is Stripe's "Nontaxable" tax category. It is what the
// remainder line item carries -- the share of a credit purchase used by
// Staff who do not work in New York, which New York is not owed tax on.
//
// The taxable line does not name a code at all: it points at the credit
// Product, which carries txcd_10103001 (Stripe's SaaS category), so the
// answer to "what tax category are credits?" lives in exactly one place
// and that place is Stripe.
const nontaxableTaxCode = "txcd_00000000"

// remainderProductName is what a buyer reads on the Checkout page beside
// the untaxed share. It names the reason for the split rather than the
// tax treatment, because the reason is the part she can check: the
// apportionment is a headcount of where her Staff work.
const remainderProductName = "Credits for staff outside New York"

// creditPricing is what CreateCheckoutSession needs to know about the
// configured credit Price to build a Session's line items. StripeAPIClient
// reads it from Stripe once, at construction, so the Price object stays
// the single source of truth for what a credit costs -- there is no
// second copy in an environment variable to drift away from it.
type creditPricing struct {
	priceID         string // the configured Price, used whole when no apportionment is needed
	productID       string // the Product that Price belongs to; carries the SaaS tax code
	currency        string
	unitAmountCents int64
}

// PurchaseSplit is how one credit purchase divides between the share New
// York taxes and the share it does not.
//
// New York sources a sale of remotely accessed software to where the
// purchaser uses it, and where users are split across states TB-ST-128
// says to collect "based on the portion of the receipt attributable to
// the users located in New York" -- so the split is a headcount of Staff,
// not a share of the caseload (#389).
type PurchaseSplit struct {
	NewYorkCents   int64
	RemainderCents int64
}

// SplitPurchase apportions totalCents between the New York share and the
// remainder, by newYorkStaff over totalStaff.
//
// The New York share rounds *up*, so a fraction of a cent is always
// resolved in New York's favour. Under-collecting is what Tax Law §1145
// penalises; over-collecting by at most one cent on a purchase is not,
// and a fixed direction makes the number reproducible years later when an
// ST-100 has to be substantiated. The two shares always sum to
// totalCents, because the remainder is what is left rather than a second
// rounding.
//
// totalStaff is never zero: only an Owner can reach this, and an Owner
// holds a Membership at the Practice she is buying for.
func SplitPurchase(totalCents int64, newYorkStaff, totalStaff int) PurchaseSplit {
	ny := (totalCents*int64(newYorkStaff) + int64(totalStaff) - 1) / int64(totalStaff)
	return PurchaseSplit{NewYorkCents: ny, RemainderCents: totalCents - ny}
}

// creditLineItems builds the Checkout Session line items for a purchase of
// req.Quantity credits by a Practice whose Staff are split as req records.
//
// A Practice whose Staff all work in New York gets the configured Price
// and a quantity, so her Checkout page looks ordinary: "Engagement credit
// x 5". Any other Practice gets the amount split across two inline
// prices, because a Price times a quantity can only ever produce whole
// multiples of one credit and the New York share is a ratio, not a count
// of credits.
func creditLineItems(cfg creditPricing, req CheckoutSessionRequest) []*stripe.CheckoutSessionCreateLineItemParams {
	if req.NewYorkStaff == req.TotalStaff {
		return []*stripe.CheckoutSessionCreateLineItemParams{
			{Price: stripe.String(cfg.priceID), Quantity: new(int64(req.Quantity))},
		}
	}

	split := SplitPurchase(cfg.unitAmountCents*int64(req.Quantity), req.NewYorkStaff, req.TotalStaff)
	items := []*stripe.CheckoutSessionCreateLineItemParams{}
	if split.NewYorkCents > 0 {
		items = append(items, &stripe.CheckoutSessionCreateLineItemParams{
			Quantity: new(int64(1)),
			PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
				Currency:    stripe.String(cfg.currency),
				Product:     stripe.String(cfg.productID),
				TaxBehavior: stripe.String("exclusive"),
				UnitAmount:  new(split.NewYorkCents),
			},
		})
	}
	if split.RemainderCents > 0 {
		items = append(items, &stripe.CheckoutSessionCreateLineItemParams{
			Quantity: new(int64(1)),
			PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
				Currency:    stripe.String(cfg.currency),
				TaxBehavior: stripe.String("exclusive"),
				UnitAmount:  new(split.RemainderCents),
				ProductData: &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{
					Name:    stripe.String(remainderProductName),
					TaxCode: stripe.String(nontaxableTaxCode),
				},
			},
		})
	}
	return items
}
