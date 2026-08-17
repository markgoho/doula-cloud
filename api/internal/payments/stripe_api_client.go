package payments

import (
	"context"
	"errors"
	"fmt"

	"github.com/stripe/stripe-go/v86"
)

// errInvoicingNotImplemented is returned by CreateInvoice and
// FinalizeInvoice: #79's ticket body scopes this ticket to implementing
// only Account Link creation and Account retrieve "for real" -- Invoicing
// is #81/#82's territory. Both methods exist now only so StripeAPIClient
// satisfies the Client interface defined for the whole of #78.
var errInvoicingNotImplemented = errors.New("payments: invoicing is not implemented yet (see #81/#82)")

// StripeAPIClient is the production Client, backed by the real Stripe API
// via stripe-go -- the same bucket/pusher-vs-client shape as
// billing.StripeAPIClient.
type StripeAPIClient struct {
	client     *stripe.Client
	appBaseURL string // used to build the Account Link's return/refresh redirect targets
}

// NewStripeAPIClient builds a StripeAPIClient from a Stripe secret API key
// and the app's own base URL. Exercised directly by
// stripe_api_client_test.go's VerifyWebhookSignature tests (a pure
// computation needing no real API key), even though every other method on
// the returned client needs a real Stripe account and network access.
func NewStripeAPIClient(apiKey, appBaseURL string) *StripeAPIClient {
	return &StripeAPIClient{client: stripe.NewClient(apiKey), appBaseURL: appBaseURL}
}

// CreateAccount creates a Stripe Connect Standard Account tagged with
// practiceID.
func (c *StripeAPIClient) CreateAccount(ctx context.Context, practiceID string) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	acct, err := c.client.V1Accounts.Create(ctx, &stripe.AccountCreateParams{
		Type:     stripe.String(string(stripe.AccountTypeStandard)),
		Metadata: map[string]string{"practice_id": practiceID},
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("payments: create stripe connect account: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return acct.ID, nil
}

// CreateAccountLink creates an Account Link for accountID's hosted Standard
// onboarding, redirecting back to practiceID's payments settings screen on
// both completion and interruption.
func (c *StripeAPIClient) CreateAccountLink(ctx context.Context, accountID, practiceID string) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	settingsURL := c.appBaseURL + "/practices/" + practiceID + "/settings/payments"
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	link, err := c.client.V1AccountLinks.Create(ctx, &stripe.AccountLinkCreateParams{
		Account:    stripe.String(accountID),
		Type:       stripe.String(string(stripe.AccountLinkTypeAccountOnboarding)),
		ReturnURL:  stripe.String(settingsURL + "?connect=return"),
		RefreshURL: stripe.String(settingsURL + "?connect=refresh"),
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("payments: create account link: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return link.URL, nil
}

// RetrieveAccount fetches accountID's current capability status directly
// from Stripe.
func (c *StripeAPIClient) RetrieveAccount(ctx context.Context, accountID string) (AccountStatus, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	acct, err := c.client.V1Accounts.GetByID(ctx, accountID, &stripe.AccountRetrieveParams{})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return AccountStatus{}, fmt.Errorf("payments: retrieve stripe connect account: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return AccountStatus{
		ChargesEnabled:   acct.ChargesEnabled,
		PayoutsEnabled:   acct.PayoutsEnabled,
		DetailsSubmitted: acct.DetailsSubmitted,
	}, nil
}

// CreateInvoice is not implemented yet -- see errInvoicingNotImplemented.
func (c *StripeAPIClient) CreateInvoice(_ context.Context, _, _, _ string, _ int64) (string, error) {
	return "", errInvoicingNotImplemented
}

// FinalizeInvoice is not implemented yet -- see errInvoicingNotImplemented.
func (c *StripeAPIClient) FinalizeInvoice(_ context.Context, _, _ string) (string, error) {
	return "", errInvoicingNotImplemented
}

// VerifyWebhookSignature verifies payload against Stripe's HMAC-SHA256
// Stripe-Signature scheme -- a pure computation with no network call
// (mirrors billing's direct use of stripe.ConstructEvent), so unlike this
// package's other methods it is exercised for real by
// stripe_api_client_test.go rather than ignored from coverage.
func (c *StripeAPIClient) VerifyWebhookSignature(payload []byte, sigHeader, secret string) (WebhookEvent, error) {
	event, err := stripe.ConstructEvent(payload, sigHeader, secret, stripe.WithIgnoreAPIVersionMismatch())
	if err != nil {
		return WebhookEvent{}, fmt.Errorf("payments: verify webhook signature: %w", err)
	}
	out := WebhookEvent{ID: event.ID, Type: string(event.Type), Account: event.Account}
	if event.Data != nil {
		out.Data = event.Data.Raw
	}
	return out, nil
}
