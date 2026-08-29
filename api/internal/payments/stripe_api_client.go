package payments

import (
	"context"
	"encoding/json"
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

// connectAccountCountry is the ISO country every Connect account is
// created in. Accounts v2 requires identity.country at create time --
// before the Practice ever reaches Stripe's hosted form -- where v1
// inferred it during onboarding, so a value has to be chosen here. "us"
// matches everything else already committed: the USD credit Price and the
// USD currency on every InvoiceItem below. A non-US Practice needs a
// country on the Practice itself, which nothing in the pilot asks for
// (#247).
const connectAccountCountry = "us"

// connectMerchantConfiguration is the one v2 Account configuration this
// integration uses. A v2 Account can carry `customer`, `merchant` and
// `recipient`; `merchant` is the Merchant of Record shape, which is what
// direct charges mean -- the Client's money lands in the Practice's
// balance and never passes through ours.
const connectMerchantConfiguration = "merchant"

// CreateAccount creates a Stripe Connect v2 Account tagged with
// practiceID, carrying the merchant configuration and a full Stripe
// dashboard (the v2 equivalent of v1's type=standard).
//
// defaults.responsibilities is mandatory for a merchant configuration and
// has no default: both collectors are "stripe", meaning the Practice's own
// account is billed Stripe's processing fee and absorbs a disputed charge.
// That is what v1's type=standard did implicitly, and it is what keeps
// Doula Cloud off the money-transmitter path -- there is no
// ApplicationFeeAmount anywhere in this package (docs/environment.md).
func (c *StripeAPIClient) CreateAccount(ctx context.Context, profile AccountProfile) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	merchant := &stripe.V2CoreAccountCreateConfigurationMerchantParams{
		Capabilities: &stripe.V2CoreAccountCreateConfigurationMerchantCapabilitiesParams{
			// card_payments is the only capability requested by
			// name. stripe_balance.payouts is not requestable on
			// create -- stripe-go carries no param for it -- and
			// Stripe grants it alongside a merchant configuration
			// anyway: a freshly created account already reports a
			// stripe_balance.payouts status (verified in the
			// Sandbox, #247).
			CardPayments: &stripe.V2CoreAccountCreateConfigurationMerchantCapabilitiesCardPaymentsParams{
				Requested: new(true),
			},
		},
		MCC: stripe.String(DoulaMCC),
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if profile.StatementDescriptor != "" {
		// Omitted rather than sent empty when the Practice's name cannot
		// make a legal one: Stripe refuses a descriptor under five
		// characters outright, and a refused create is worse than the
		// extra field she would otherwise have filled in anyway (#442).
		merchant.StatementDescriptor = &stripe.V2CoreAccountCreateConfigurationMerchantStatementDescriptorParams{
			Descriptor: stripe.String(profile.StatementDescriptor),
		}
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	stripeProfile := &stripe.V2CoreAccountCreateDefaultsProfileParams{
		BusinessURL: stripe.String(profile.BusinessURL),
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if profile.ProductDescription != "" {
		stripeProfile.ProductDescription = stripe.String(profile.ProductDescription)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	acct, err := c.client.V2CoreAccounts.Create(ctx, &stripe.V2CoreAccountCreateParams{
		// The Practice's own name, so the Client's hosted invoice reads
		// "From <the Practice they hired>". Stripe falls back to the
		// statement descriptor when this is unset, which showed a walked
		// invoice as being from "DOULA.CLOU" (#247).
		DisplayName: stripe.String(profile.PracticeName),
		Identity: &stripe.V2CoreAccountCreateIdentityParams{
			Country: stripe.String(connectAccountCountry),
		},
		Configuration: &stripe.V2CoreAccountCreateConfigurationParams{
			Merchant: merchant,
		},
		Defaults: &stripe.V2CoreAccountCreateDefaultsParams{
			Responsibilities: &stripe.V2CoreAccountCreateDefaultsResponsibilitiesParams{
				FeesCollector:   stripe.String(string(stripe.V2CoreAccountDefaultsResponsibilitiesFeesCollectorStripe)),
				LossesCollector: stripe.String(string(stripe.V2CoreAccountDefaultsResponsibilitiesLossesCollectorStripe)),
			},
			Profile: stripeProfile,
		},
		Dashboard: stripe.String(string(stripe.V2CoreAccountDashboardFull)),
		Metadata:  map[string]string{"practice_id": profile.PracticeID},
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("payments: create stripe connect account: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return acct.ID, nil
}

// CreateAccountLink creates a v2 Account Link for accountID's hosted
// merchant onboarding, redirecting back to practiceID's payments settings
// screen on both completion and interruption.
func (c *StripeAPIClient) CreateAccountLink(ctx context.Context, accountID, practiceID string) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	settingsURL := c.appBaseURL + "/practices/" + practiceID + "/settings/payments"
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	link, err := c.client.V2CoreAccountLinks.Create(ctx, &stripe.V2CoreAccountLinkCreateParams{
		Account: stripe.String(accountID),
		UseCase: &stripe.V2CoreAccountLinkCreateUseCaseParams{
			Type: stripe.String("account_onboarding"),
			AccountOnboarding: &stripe.V2CoreAccountLinkCreateUseCaseAccountOnboardingParams{
				Configurations: []*string{stripe.String(connectMerchantConfiguration)},
				ReturnURL:      stripe.String(settingsURL + "?connect=return"),
				RefreshURL:     stripe.String(settingsURL + "?connect=refresh"),
			},
		},
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("payments: create account link: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return link.URL, nil
}

// RetrieveAccount fetches accountID's current merchant capability
// statuses and outstanding requirements directly from Stripe. Both are
// `include`-gated on v2: the Account comes back without
// configuration.merchant or requirements unless they are asked for by
// name.
func (c *StripeAPIClient) RetrieveAccount(ctx context.Context, accountID string) (AccountStatus, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	acct, err := c.client.V2CoreAccounts.Retrieve(ctx, accountID, &stripe.V2CoreAccountRetrieveParams{
		Include: []*string{
			stripe.String("configuration.merchant"),
			stripe.String("requirements"),
		},
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return AccountStatus{}, fmt.Errorf("payments: retrieve stripe connect account: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return accountStatusFrom(acct), nil
}

// ParseAccountEvent verifies payload as a v2 thin event notification and
// reports which account it concerns. Like VerifyWebhookSignature this is
// signature verification plus a decode with no network call, so it is
// exercised for real by stripe_api_client_test.go rather than ignored
// from coverage.
//
// stripe-go models a thin notification as a union: the typed variants
// carry a RelatedObject field, but they do not share an interface that
// exposes it (EventNotificationContainer reaches only the common header).
// Rather than type-switch over every account event Stripe might add, the
// verified bytes are decoded once more into the one field this handler
// needs.
func (c *StripeAPIClient) ParseAccountEvent(payload []byte, sigHeader, secret string) (AccountEvent, error) {
	notification, err := c.client.ParseEventNotification(payload, sigHeader, secret, stripe.WithIgnoreAPIVersionMismatch())
	if err != nil {
		return AccountEvent{}, fmt.Errorf("payments: verify account event signature: %w", err)
	}
	header := notification.GetEventNotification()

	var related struct {
		RelatedObject struct {
			ID string `json:"id"`
		} `json:"related_object"`
	}
	if err := json.Unmarshal(payload, &related); err != nil {
		// coverage:ignore reason: ParseEventNotification already decoded this same payload as JSON, so a second Unmarshal of it cannot fail
		return AccountEvent{}, fmt.Errorf("payments: decode account event related object: %w", err)
	}

	return AccountEvent{
		ID:        header.ID,
		Type:      header.Type,
		AccountID: related.RelatedObject.ID,
	}, nil
}

// accountStatusFrom projects a v2 Account onto the AccountStatus the rest
// of the package works in. Split out from RetrieveAccount, which cannot be
// unit-tested without a real Stripe account, so the v2-shape-to-our-shape
// mapping -- the part with actual decisions in it -- is testable on its
// own.
//
// Every level is nil-checked rather than assumed: `include` controls what
// Stripe returns, a configuration the account does not carry comes back
// null, and an un-requested capability is null inside a configuration that
// is present. A missing capability reads as CapabilityUnsupported, which
// is the same thing Stripe means by it.
func accountStatusFrom(acct *stripe.V2CoreAccount) AccountStatus {
	out := AccountStatus{
		CardPayments:    CapabilityUnsupported,
		Payouts:         CapabilityUnsupported,
		RequirementsDue: []string{},
	}

	if acct.Configuration != nil && acct.Configuration.Merchant != nil && acct.Configuration.Merchant.Capabilities != nil {
		capabilities := acct.Configuration.Merchant.Capabilities
		if capabilities.CardPayments != nil {
			out.CardPayments = CapabilityStatus(capabilities.CardPayments.Status)
		}
		if capabilities.StripeBalance != nil && capabilities.StripeBalance.Payouts != nil {
			out.Payouts = CapabilityStatus(capabilities.StripeBalance.Payouts.Status)
		}
	}

	if acct.Requirements != nil {
		for _, entry := range acct.Requirements.Entries {
			// Only entries the account holder can act on belong here.
			// Stripe also raises entries awaiting itself (a review in
			// progress) and awaiting the platform, and neither is
			// something the Owner can clear by reopening onboarding.
			if entry != nil && entry.AwaitingActionFrom == stripe.V2CoreAccountRequirementsEntryAwaitingActionFromUser {
				out.RequirementsDue = append(out.RequirementsDue, entry.Description)
			}
		}
	}

	return out
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

// RetrieveInvoicePaymentReference reports the PaymentIntent id behind
// invoiceID's payment, read from the InvoicePayment list rather than the
// Invoice itself: under this SDK's API version an Invoice carries neither
// payment_intent nor charge.
func (c *StripeAPIClient) RetrieveInvoicePaymentReference(ctx context.Context, accountID, invoiceID string) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	params := &stripe.InvoicePaymentListParams{Invoice: stripe.String(invoiceID)}
	params.StripeAccount = stripe.String(accountID)
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	for payment, err := range c.client.V1InvoicePayments.List(ctx, params).All(ctx) {
		// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
		if err != nil {
			return "", fmt.Errorf("payments: list invoice payments: %w", err)
		}
		// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
		if payment.Payment != nil && payment.Payment.PaymentIntent != nil {
			return payment.Payment.PaymentIntent.ID, nil
		}
	}
	// No payment recorded against the invoice. Not an error -- the caller
	// stores an empty reference rather than rejecting the webhook.
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return "", nil
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
