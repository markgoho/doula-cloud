package billing

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/stripe/stripe-go/v86"
)

// StripeAPIClient is the production StripeClient, backed by the real
// Stripe API via stripe-go -- the same bucket/pusher-vs-client shape as
// objectstore.GCSStore and push.VAPIDPusher.
type StripeAPIClient struct {
	client        *stripe.Client
	creditPriceID string
	appBaseURL    string

	mu            sync.Mutex
	cachedPricing *creditPricing
}

// NewStripeAPIClient builds a StripeAPIClient from a Stripe secret API
// key, the Stripe Price id representing one credit's flat fee, and the
// app's own base URL (used to build the Checkout Session's success/cancel
// redirect targets).
func NewStripeAPIClient(apiKey, creditPriceID, appBaseURL string) *StripeAPIClient {
	// coverage:ignore reason: only called from main() to build the real client; not exercised by unit tests, which inject billing.FakeStripeClient instead
	return &StripeAPIClient{client: stripe.NewClient(apiKey), creditPriceID: creditPriceID, appBaseURL: appBaseURL}
}

// pricing reads the configured credit Price back from Stripe, once, and
// remembers it.
//
// The amount is fetched rather than configured as a second environment
// variable because an apportioned Session has to state cent amounts
// itself, and a copy of the price outside Stripe is a copy that can
// disagree with the Price the ordinary path charges. It is fetched here
// rather than at construction because the BFF has to boot without Stripe
// credentials -- end-to-end tests and the image's boot smoke test both
// start it with none -- so a missing or wrong price id fails the purchase
// that needs it, not the process.
func (c *StripeAPIClient) pricing(ctx context.Context) (creditPricing, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	c.mu.Lock()
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	defer c.mu.Unlock()
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if c.cachedPricing != nil {
		return *c.cachedPricing, nil
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	price, err := c.client.V1Prices.Retrieve(ctx, c.creditPriceID, nil)
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return creditPricing{}, fmt.Errorf("billing: retrieve credit price %q: %w", c.creditPriceID, err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if price.Product == nil {
		return creditPricing{}, fmt.Errorf("billing: credit price %q has no product", c.creditPriceID)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	c.cachedPricing = &creditPricing{
		priceID:         price.ID,
		productID:       price.Product.ID,
		currency:        string(price.Currency),
		unitAmountCents: price.UnitAmount,
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return *c.cachedPricing, nil
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
	pricing, err := c.pricing(ctx)
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", err
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	sess, err := c.client.V1CheckoutSessions.Create(ctx, &stripe.CheckoutSessionCreateParams{
		Customer:     stripe.String(req.CustomerID),
		Mode:         stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems:    creditLineItems(pricing, req),
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

// RefundPayment refunds amountCents against paymentIntentID.
//
// The idempotency key is Stripe's own replay guard: a retry carrying the
// same key returns the refund the first call made, rather than moving the
// money twice.
//
// Naming the PaymentIntent rather than building a payout is deliberate:
// Stripe Tax reverses the tax it reported on the original payment in
// proportion to what is refunded, so the ST-100 stays correct without a
// second act. The amount includes the tax share the ledger computed, so
// what Stripe reverses and what credit_ledger records are the same
// number.
func (c *StripeAPIClient) RefundPayment(ctx context.Context, paymentIntentID, idempotencyKey string, amountCents int64) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	params := &stripe.RefundCreateParams{
		PaymentIntent: stripe.String(paymentIntentID),
		Amount:        new(amountCents),
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	params.SetIdempotencyKey(idempotencyKey)
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	refund, err := c.client.V1Refunds.Create(ctx, params)
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("billing: refund payment %q: %w", paymentIntentID, err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return refund.ID, nil
}
