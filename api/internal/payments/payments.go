// Package payments owns a Practice's Stripe Connect linkage -- the
// Client -> Practice side of Doula Cloud's Stripe integration, distinct
// from billing's Practice -> Doula Cloud side. #79 establishes the schema
// (columns on practices, mirroring billing's stripe_customer_id pattern),
// the Client Stripe-client port, and Owner-only onboarding via a
// Stripe-hosted Account Link. #80 adds the webhook that keeps the
// capability state live. #81/#82 add Invoice creation.
//
// #247 moved the account leg from Stripe's Accounts v1 to Accounts v2:
// Stripe refuses to create v1 accounts for new integrations. A v2 Account
// carries named *configurations* rather than a single `type`, and this
// package uses exactly one of them, `merchant` -- the Merchant of Record
// shape that direct charges require, matching the Params.StripeAccount
// header every Invoice call already sets. The Invoice leg stays on v1
// APIs, which accept a v2 account id unchanged.
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

// CapabilityStatus is the status Stripe reports for one capability on a
// v2 Account's merchant configuration -- four-valued, where v1's
// equivalent was a boolean. Only Active means the capability actually
// works; Pending is Stripe reviewing something the Owner has already
// supplied, and Restricted means something is still outstanding (see
// AccountStatus.RequirementsDue).
type CapabilityStatus string

// The four values Stripe's capability `status` field takes.
const (
	CapabilityActive      CapabilityStatus = "active"
	CapabilityPending     CapabilityStatus = "pending"
	CapabilityRestricted  CapabilityStatus = "restricted"
	CapabilityUnsupported CapabilityStatus = "unsupported"
)

// AccountStatus mirrors the subset of Stripe's v2 Account object payments
// cares about, and is what practices.stripe_connect_card_payments_status,
// stripe_connect_payouts_status and stripe_connect_requirements_due hold
// (00029_stripe_connect_accounts_v2.sql).
//
// CardPayments is
// configuration.merchant.capabilities.card_payments.status -- whether the
// Practice can be paid by card at all, replacing v1's charges_enabled.
// Payouts is
// configuration.merchant.capabilities.stripe_balance.payouts.status --
// whether that money can reach the Practice's bank, replacing v1's
// payouts_enabled. They move independently: an account can take cards
// while its payouts are still restricted.
//
// RequirementsDue is what replaces v1's details_submitted, which has no v2
// equivalent. It holds the `description` of every requirements entry still
// awaiting the account holder -- dotted Stripe field paths such as
// "configuration.merchant.mcc". Empty means nothing is outstanding.
type AccountStatus struct {
	CardPayments    CapabilityStatus
	Payouts         CapabilityStatus
	RequirementsDue []string
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
	// CreateAccount creates a Stripe Connect v2 Account with the merchant
	// configuration for practiceID and returns its Stripe account id.
	// Called at most once per Practice -- the caller persists the id on
	// practices.stripe_connect_account_id and reuses it on every later
	// onboarding attempt.
	CreateAccount(ctx context.Context, practiceID string) (accountID string, err error)
	// CreateAccountLink creates a single-use Stripe v2 Account Link for
	// accountID's hosted merchant onboarding flow, tagged so its
	// return/refresh redirects land back on practiceID's payments settings
	// screen. Returns the onboarding URL the Owner's browser should be
	// sent to.
	CreateAccountLink(ctx context.Context, accountID, practiceID string) (onboardingURL string, err error)
	// RetrieveAccount fetches accountID's current capability statuses and
	// outstanding requirements directly from Stripe. It serves two
	// callers: GetConnectStatusHandler's on-demand read (#79 --
	// deliberately not backed by the persisted columns), and the account
	// webhook, which gets only an account id from a thin event and must
	// fetch the state itself (#247).
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
	// the decoded event's raw JSON on success. This is the v1 *snapshot*
	// event path only -- it carries the Invoice events surface B still
	// receives in the v1 shape. stripe.ConstructEvent rejects a v2 thin
	// event outright, so account events go through ParseAccountEvent
	// instead.
	VerifyWebhookSignature(payload []byte, sigHeader, secret string) (WebhookEvent, error)
	// ParseAccountEvent verifies payload as a Stripe v2 *thin* event
	// notification, delivered to an event destination rather than a v1
	// webhook endpoint. A thin notification carries no object -- only an
	// id, a type, and a reference to the resource that changed -- so this
	// returns the account id to fetch rather than any capability state.
	ParseAccountEvent(payload []byte, sigHeader, secret string) (AccountEvent, error)
}

// AccountEvent is a verified v2 thin event notification about one Connect
// account: its id (for the same stripe_webhook_events idempotency the
// snapshot path uses), its type, and the account the change concerns,
// read off the notification's related_object. There is no payload to
// unmarshal -- Stripe sends none, by design, so the handler calls
// RetrieveAccount for the current state.
type AccountEvent struct {
	ID        string
	Type      string
	AccountID string
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
