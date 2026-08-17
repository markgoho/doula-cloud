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
	client      *stripe.Client
	creditPrice string // Stripe Price id for one credit; CreateCheckoutSession multiplies it by quantity
	appBaseURL  string
}

// NewStripeAPIClient builds a StripeAPIClient from a Stripe secret API
// key, the Stripe Price id representing one credit's flat fee, and the
// app's own base URL (used to build the Checkout Session's success/cancel
// redirect targets).
func NewStripeAPIClient(apiKey, creditPrice, appBaseURL string) *StripeAPIClient {
	// coverage:ignore reason: only called from main() to build the real client; not exercised by unit tests, which inject billing.FakeStripeClient instead
	return &StripeAPIClient{client: stripe.NewClient(apiKey), creditPrice: creditPrice, appBaseURL: appBaseURL}
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

// CreateCheckoutSession creates a Checkout Session for quantity credits
// (quantity x the flat per-credit Price), tagged with practiceID and
// quantity in metadata -- the same pair PostPurchaseWebhookHandler reads
// back out of the confirmed event to credit the right Practice's ledger.
// No clinical/care content is ever sent, per the platform's no-PHI-to-
// Stripe rule -- only the Practice id and a credit count.
func (c *StripeAPIClient) CreateCheckoutSession(ctx context.Context, customerID, practiceID string, quantity int) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	sess, err := c.client.V1CheckoutSessions.Create(ctx, &stripe.CheckoutSessionCreateParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{Price: stripe.String(c.creditPrice), Quantity: new(int64(quantity))},
		},
		SuccessURL: stripe.String(c.appBaseURL + "/practices/" + practiceID + "/billing?checkout=success"),
		CancelURL:  stripe.String(c.appBaseURL + "/practices/" + practiceID + "/billing?checkout=cancelled"),
		Metadata: map[string]string{
			"practice_id": practiceID,
			"quantity":    strconv.Itoa(quantity),
		},
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("billing: create checkout session: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return sess.URL, nil
}
