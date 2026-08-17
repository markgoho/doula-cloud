package payments

import (
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v86"
)

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

// CreateInvoice creates a draft Stripe Invoice on behalf of accountID's
// connected account: a Customer (tagged with the Client's name/email,
// nothing else -- no metadata, per #78's no-PHI-to-Stripe rule), a draft
// Invoice billing that Customer via collection_method=send_invoice (so
// Stripe emails it once finalized rather than auto-charging a saved card),
// and a single InvoiceItem for amountCents described as description. Every
// call is made with the Params.StripeAccount on-behalf-of header set to
// accountID, per #78's ticket body ("using the Stripe-Account association,
// not a separate OAuth token per Practice"), rather than a platform-level
// call. Returns the draft Invoice's id; FinalizeInvoice makes it payable.
func (c *StripeAPIClient) CreateInvoice(ctx context.Context, accountID, customerEmail, customerName, description string, amountCents int64) (string, error) {
	onBehalfOf := stripe.Params{StripeAccount: stripe.String(accountID)}

	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	cust, err := c.client.V1Customers.Create(ctx, &stripe.CustomerCreateParams{
		Params: onBehalfOf,
		Email:  stripe.String(customerEmail),
		Name:   stripe.String(customerName),
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("payments: create stripe customer for invoice: %w", err)
	}

	// DaysUntilDue is not a Doula Cloud payment-terms policy -- Stripe's
	// API rejects collection_method=send_invoice without either
	// days_until_due or due_date set, so a value is mandatory here purely
	// to satisfy that constraint. 30 is a fixed, non-configurable
	// placeholder; unlike the "Professional services" description, #78/#81
	// make no claim about what this should be.
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	inv, err := c.client.V1Invoices.Create(ctx, &stripe.InvoiceCreateParams{
		Params:              onBehalfOf,
		Customer:            stripe.String(cust.ID),
		CollectionMethod:    stripe.String(string(stripe.InvoiceCollectionMethodSendInvoice)),
		DaysUntilDue:        stripe.Int64(30),
		StatementDescriptor: stripe.String(description),
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("payments: create stripe invoice: %w", err)
	}

	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	_, err = c.client.V1InvoiceItems.Create(ctx, &stripe.InvoiceItemCreateParams{
		Params:      onBehalfOf,
		Customer:    stripe.String(cust.ID),
		Invoice:     stripe.String(inv.ID),
		Amount:      new(amountCents),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String(description),
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("payments: create stripe invoice item: %w", err)
	}

	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return inv.ID, nil
}

// FinalizeInvoice finalizes invoiceID on accountID's connected account --
// the transition that makes it payable and triggers Stripe's hosted
// invoice email to the Customer -- and returns its hosted payment page URL.
func (c *StripeAPIClient) FinalizeInvoice(ctx context.Context, accountID, invoiceID string) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	inv, err := c.client.V1Invoices.FinalizeInvoice(ctx, invoiceID, &stripe.InvoiceFinalizeInvoiceParams{
		Params: stripe.Params{StripeAccount: stripe.String(accountID)},
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("payments: finalize stripe invoice: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return inv.HostedInvoiceURL, nil
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
