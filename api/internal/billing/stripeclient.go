package billing

import "context"

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
	// CreateCheckoutSession creates a Stripe Checkout Session for quantity
	// credits against the Stripe Customer customerID, tagged with
	// practiceID and quantity in metadata so the webhook can credit the
	// right Practice's ledger. Returns the Session's hosted checkout URL.
	CreateCheckoutSession(ctx context.Context, customerID, practiceID string, quantity int) (string, error)
}
