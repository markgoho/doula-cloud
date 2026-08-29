package billing

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stripe/stripe-go/v86"
)

// StripeAPIClient is the production StripeClient, backed by the real
// Stripe API via stripe-go -- the same bucket/pusher-vs-client shape as
// objectstore.GCSStore and push.VAPIDPusher.
type StripeAPIClient struct {
	client     *stripe.Client
	pricing    creditPricing
	appBaseURL string
}

// NewStripeAPIClient builds a StripeAPIClient from a Stripe secret API
// key, the Stripe Price id representing one credit's flat fee, and the
// app's own base URL (used to build the Checkout Session's success/cancel
// redirect targets).
//
// It reads the Price back from Stripe once, here, rather than taking the
// amount as a second environment variable: an apportioned purchase has to
// state cent amounts itself, and a copy of the price outside Stripe is a
// copy that can disagree with the Price the ordinary path charges.
func NewStripeAPIClient(ctx context.Context, apiKey, creditPriceID, appBaseURL string) (*StripeAPIClient, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	client := stripe.NewClient(apiKey)
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	price, err := client.V1Prices.Retrieve(ctx, creditPriceID, nil)
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("billing: retrieve credit price %q: %w", creditPriceID, err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if price.Product == nil {
		return nil, fmt.Errorf("billing: credit price %q has no product", creditPriceID)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return &StripeAPIClient{
		client: client,
		pricing: creditPricing{
			priceID:         price.ID,
			productID:       price.Product.ID,
			currency:        string(price.Currency),
			unitAmountCents: price.UnitAmount,
		},
		appBaseURL: appBaseURL,
	}, nil
}

// CreateCustomer creates a Stripe Customer tagged with practiceID.
func (c *StripeAPIClient) CreateCustomer(ctx context.Context, practiceID string) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	cust, err := c.client.V1Customers.Create(ctx, &stripe.CustomerCreateParams{
		Metadata: map[string]string{"practice_id": practiceID},
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("billing: create stripe customer: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return cust.ID, nil
}

// CreateCheckoutSession creates a Checkout Session for req.Quantity
// credits, tagged with the Practice id and quantity in metadata -- the
// same pair PostPurchaseWebhookHandler reads back out of the confirmed
// event to credit the right Practice's ledger. No clinical/care content is
// ever sent, per the platform's no-PHI-to-Stripe rule -- only the Practice
// id, a credit count, and how many of its Staff work in New York.
//
// Automatic tax is on, and it computes nothing until a tax registration
// exists: with none, Stripe returns not_collecting and zero tax, silently.
// Adding New York's registration is one Dashboard act and it belongs to
// #388, on the day the Certificate of Authority issues.
//
// customer_update[address]=auto keeps the address Checkout collects for
// the tax calculation, instead of discarding it -- it is the evidence for
// which state's tax was charged.
func (c *StripeAPIClient) CreateCheckoutSession(ctx context.Context, req CheckoutSessionRequest) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	sess, err := c.client.V1CheckoutSessions.Create(ctx, &stripe.CheckoutSessionCreateParams{
		Customer:     stripe.String(req.CustomerID),
		Mode:         stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems:    creditLineItems(c.pricing, req),
		AutomaticTax: &stripe.CheckoutSessionCreateAutomaticTaxParams{Enabled: new(true)},
		CustomerUpdate: &stripe.CheckoutSessionCreateCustomerUpdateParams{
			Address: stripe.String("auto"),
		},
		SuccessURL: stripe.String(c.appBaseURL + "/practices/" + req.PracticeID + "/billing?checkout=success"),
		CancelURL:  stripe.String(c.appBaseURL + "/practices/" + req.PracticeID + "/billing?checkout=cancelled"),
		Metadata: map[string]string{
			"practice_id": req.PracticeID,
			"quantity":    strconv.Itoa(req.Quantity),
			"ny_staff":    strconv.Itoa(req.NewYorkStaff),
			"total_staff": strconv.Itoa(req.TotalStaff),
		},
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("billing: create checkout session: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return sess.URL, nil
}
