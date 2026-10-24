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
	cachedPricing map[int64]creditPricing
}

// NewStripeAPIClient builds a StripeAPIClient from a Stripe secret API
// key, a Stripe Price on the credit Product (the variable names the
// Product through one of its Prices, and never has to move when the
// price of a Credit does, #1834), and the app's own base URL (used to
// build the Checkout Session's success/cancel redirect targets).
func NewStripeAPIClient(apiKey, creditPriceID, appBaseURL string) *StripeAPIClient {
	// coverage:ignore reason: only called from main() to build the real client; not exercised by unit tests, which inject billing.FakeStripeClient instead
	return &StripeAPIClient{client: stripe.NewClient(apiKey), creditPriceID: creditPriceID, appBaseURL: appBaseURL}
}

// pricing finds the active Price on the credit Product whose unit amount
// is unitAmountCents, the price of a Credit the Terms in force state
// (#1834), and remembers it.
//
// The amount comes from the Terms, not from the Price: a Price is
// immutable, so a new price of a Credit is a new Price on the same
// Product, created before its version of the Terms takes effect, and
// this picks it on the date by its amount. Nothing has to move an
// environment variable on the day, and no Price can charge more than
// the Terms allow: with no active Price at the amount, the purchase is
// refused rather than charged at another one. The Price must be
// tax_behavior exclusive, or automatic_tax refuses to compute.
//
// It is read here rather than at construction because the BFF has to
// boot without Stripe credentials -- end-to-end tests and the image's
// boot smoke test both start it with none -- so a missing or wrong price
// id fails the purchase that needs it, not the process.
func (c *StripeAPIClient) pricing(ctx context.Context, unitAmountCents int64) (creditPricing, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	c.mu.Lock()
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	defer c.mu.Unlock()
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if cached, ok := c.cachedPricing[unitAmountCents]; ok {
		return cached, nil
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	configured, err := c.client.V1Prices.Retrieve(ctx, c.creditPriceID, nil)
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return creditPricing{}, fmt.Errorf("billing: retrieve credit price %q: %w", c.creditPriceID, err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if configured.Product == nil {
		return creditPricing{}, fmt.Errorf("billing: credit price %q has no product", c.creditPriceID)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	var match *stripe.Price
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	for price, err := range c.client.V1Prices.List(ctx, &stripe.PriceListParams{
		Product:  stripe.String(configured.Product.ID),
		Active:   new(true),
		Currency: stripe.String(creditCurrency),
		Type:     stripe.String(string(stripe.PriceTypeOneTime)),
	}).All(ctx) {
		// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
		if err != nil {
			return creditPricing{}, fmt.Errorf("billing: list prices of credit product %q: %w", configured.Product.ID, err)
		}
		// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
		if price.UnitAmount == unitAmountCents && price.TaxBehavior == stripe.PriceTaxBehaviorExclusive {
			match = price
			break
		}
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if match == nil {
		return creditPricing{}, fmt.Errorf("billing: credit product %q has no active exclusive-tax Price of %d cents, which the Terms in force charge; create one (docs/environment.md, \"A rise in the price of a Credit\")", configured.Product.ID, unitAmountCents)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	pricing := creditPricing{
		priceID:         match.ID,
		productID:       configured.Product.ID,
		currency:        creditCurrency,
		unitAmountCents: match.UnitAmount,
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if c.cachedPricing == nil {
		c.cachedPricing = map[int64]creditPricing{}
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	c.cachedPricing[unitAmountCents] = pricing
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return pricing, nil
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
	pricing, err := c.pricing(ctx, req.UnitAmountCents)
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
		CancelURL:  stripe.String(c.appBaseURL + "/practices/" + req.PracticeID + "/billing?checkout=canceled"),
		Metadata: map[string]string{
			"practice_id": req.PracticeID,
			"quantity":    strconv.Itoa(req.Quantity),
			"ny_staff":    strconv.Itoa(req.NewYorkStaff),
			"total_staff": strconv.Itoa(req.TotalStaff),
			// Which version of the Terms set the price this Session
			// charges, so a paid Session answers "why this price?"
			// beside the lot's own unit_price_cents (#1834).
			"terms_version": req.TermsVersion,
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
