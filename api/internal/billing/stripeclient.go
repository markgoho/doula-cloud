package billing

import "context"

// CheckoutSessionRequest is everything CreateCheckoutSession needs to
// build one credit purchase.
//
// NewYorkStaff and TotalStaff ride along because the taxable share of the
// sale is a headcount of where the Practice's Staff work (#389), and only
// the handler can count them -- it holds the request's transaction, and
// row-level security scopes the count to the Practice being billed.
type CheckoutSessionRequest struct {
	CustomerID   string
	PracticeID   string
	Quantity     int
	NewYorkStaff int
	TotalStaff   int
}

// StripeClient is the seam over the outbound Stripe API calls that
// PostPurchaseHandler makes -- creating a Practice's Stripe Customer and
// its Checkout Session -- so tests can inject FakeStripeClient instead of
// reaching a real Stripe account. StripeAPIClient is the production,
// stripe-go-backed implementation. Mirrors push.Pusher's shape: a small
// interface around exactly the calls a handler needs, not the whole SDK.
//
// Webhook signature verification is deliberately not part of this
// interface -- it's pure HMAC-SHA256 with no network call, so
// PostPurchaseWebhookHandler calls stripe.ConstructEvent directly and
// tests exercise it for real with a self-signed test payload.
type StripeClient interface {
	// CreateCustomer creates a Stripe Customer tagged with practiceID and
	// returns its Stripe id.
	CreateCustomer(ctx context.Context, practiceID string) (string, error)
	// CreateCheckoutSession creates a Stripe Checkout Session for
	// req.Quantity credits against the Stripe Customer req.CustomerID,
	// tagged with the Practice id and quantity in metadata so the webhook
	// can credit the right Practice's ledger. Returns the Session's hosted
	// checkout URL.
	CreateCheckoutSession(ctx context.Context, req CheckoutSessionRequest) (string, error)
	// RefundPayment refunds amountCents against the PaymentIntent a
	// credit purchase arrived on, and returns the Stripe Refund's id.
	// idempotencyKey is what makes a retried request replay the first
	// refund instead of issuing a second one -- the endpoint is run by
	// hand, so a timeout followed by a second attempt is the expected
	// failure, not an exotic one.
	// Refunding against the original payment -- rather than paying the
	// money out some other way -- is what makes Stripe Tax reverse the
	// sales tax it reported, so the ST-100 does not overstate what New
	// York is owed (#420).
	RefundPayment(ctx context.Context, paymentIntentID, idempotencyKey string, amountCents int64) (string, error)
}
