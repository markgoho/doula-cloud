// Package payments owns a Practice's Stripe Connect (Standard tier)
// linkage -- the Client -> Practice side of Doula Cloud's Stripe
// integration, distinct from billing's Practice -> Doula Cloud side. #79
// establishes the schema (three columns on practices, mirroring billing's
// stripe_customer_id pattern), the Client Stripe-client port, and
// Owner-only onboarding via a Stripe-hosted Account Link. #80 adds the
// Connect webhook that keeps the capability booleans live. Later tickets
// in #78 (#81/#82) add Invoice creation.
package payments

import "context"

// InvoiceLineItemDescription is the fixed line-item description and
// Stripe statement descriptor every Invoice uses -- unconditional, not a
// default, per #78's no-PHI-to-Stripe rule (a Client's identity must
// never be paired with anything that implies a health condition, on the
// Stripe dashboard, a card statement, or the invoice email). Passed
// through the Client port as an explicit parameter, rather than baked
// into StripeAPIClient.CreateInvoice alone, so PostInvoiceHandler's tests
// can assert -- via FakeClient -- that this constant, and nothing
// request-supplied, is what actually reaches Stripe.
const InvoiceLineItemDescription = "Professional services"

// AccountStatus mirrors the subset of Stripe's Account object payments
// cares about -- the same three capabilities persisted (by #80's webhook
// handler) on practices.stripe_connect_charges_enabled,
// stripe_connect_payouts_enabled, and stripe_connect_details_submitted.
type AccountStatus struct {
	ChargesEnabled   bool
	PayoutsEnabled   bool
	DetailsSubmitted bool
}

// Client is the seam over the outbound Stripe API calls the payments
// package needs across all of #78 -- Connect account linkage (#79),
// webhook signature verification (#80), and Invoicing (#81/#82) -- so
// tests can inject a fake instead of reaching a real Stripe account.
// StripeAPIClient is the production, stripe-go-backed implementation.
// Mirrors billing.StripeClient's shape (a small interface around exactly
// the calls the package needs, not the whole SDK), but is defined for the
// whole feature up front per #79's ticket body, not grown incrementally --
// #79 landed CreateInvoice and FinalizeInvoice as stubs (returning
// errInvoicingNotImplemented) purely so StripeAPIClient satisfied this
// interface before #81 built them for real.
//
// Unlike billing.StripeClient, webhook signature verification is part of
// this port rather than called directly via stripe.ConstructEvent -- #78's
// spec explicitly asks for it to be injectable here, since #80's Connect
// webhook handler also needs to disambiguate events by connected account,
// not just verify them.
//
// billing.StripeClient (#77) predates this ticket but covers a narrower,
// non-overlapping set of calls (Customer + Checkout Session, for the
// Practice -> Doula Cloud billing relationship) -- not an equivalent port,
// so this is a second, separate Stripe-client seam rather than a reuse of
// billing's. Any later ticket needing this same shape of thing (an
// outbound Stripe API port) should reuse this Client interface rather than
// inventing a third one.
type Client interface {
	// CreateAccount creates a Stripe Connect Standard Account for
	// practiceID and returns its Stripe account id. Called at most once
	// per Practice -- the caller persists the id on
	// practices.stripe_connect_account_id and reuses it on every later
	// onboarding attempt.
	CreateAccount(ctx context.Context, practiceID string) (accountID string, err error)
	// CreateAccountLink creates a single-use Stripe Account Link for
	// accountID's hosted Standard onboarding flow, tagged so its
	// return/refresh redirects land back on practiceID's payments settings
	// screen. Returns the onboarding URL the Owner's browser should be
	// sent to.
	CreateAccountLink(ctx context.Context, accountID, practiceID string) (onboardingURL string, err error)
	// RetrieveAccount fetches accountID's current capability status
	// directly from Stripe -- an on-demand read, not backed by the
	// webhook-synced booleans on practices (see #79's ticket body).
	RetrieveAccount(ctx context.Context, accountID string) (AccountStatus, error)
	// CreateInvoice creates a Stripe Invoice (draft, not yet finalized) on
	// behalf of accountID's connected account, billing
	// customerEmail/customerName amountCents for a single line item read
	// as description (always InvoiceLineItemDescription in production) --
	// no clinical content is ever included, per #78's no-PHI-to-Stripe
	// rule. Returns the created Invoice's id.
	CreateInvoice(ctx context.Context, accountID, customerEmail, customerName, description string, amountCents int64) (invoiceID string, err error)
	// FinalizeInvoice finalizes invoiceID on accountID's connected
	// account, making it payable, and returns its hosted payment page URL.
	FinalizeInvoice(ctx context.Context, accountID, invoiceID string) (hostedInvoiceURL string, err error)
	// VerifyWebhookSignature verifies payload was sent by Stripe using
	// secret and the Stripe-Signature header value sigHeader, returning
	// the decoded event's raw JSON on success.
	VerifyWebhookSignature(payload []byte, sigHeader, secret string) (WebhookEvent, error)
}

// WebhookEvent is the subset of a verified Stripe event #80's Connect
// webhook handler needs: its id (for idempotency, mirroring
// stripe_webhook_events), its type, the connected account it occurred on,
// and its raw object payload to unmarshal further.
type WebhookEvent struct {
	ID      string
	Type    string
	Account string
	Data    []byte
}
