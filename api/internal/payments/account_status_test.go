package payments

import (
	"testing"

	"github.com/stripe/stripe-go/v86"
)

// accountStatusFrom is the whole of the v2-shape-to-our-shape mapping,
// and the only part of RetrieveAccount that can be exercised without a
// real Stripe account. These tests pin the two things the mapping
// decides: how a missing level reads, and which requirements entries
// count as the Owner's to clear.

func merchantAccount(cardPayments string, payouts *string) *stripe.V2CoreAccount {
	capabilities := &stripe.V2CoreAccountConfigurationMerchantCapabilities{
		CardPayments: &stripe.V2CoreAccountConfigurationMerchantCapabilitiesCardPayments{
			Status: stripe.V2CoreAccountConfigurationMerchantCapabilitiesCardPaymentsStatus(cardPayments),
		},
	}
	if payouts != nil {
		capabilities.StripeBalance = &stripe.V2CoreAccountConfigurationMerchantCapabilitiesStripeBalance{
			Payouts: &stripe.V2CoreAccountConfigurationMerchantCapabilitiesStripeBalancePayouts{
				Status: stripe.V2CoreAccountConfigurationMerchantCapabilitiesStripeBalancePayoutsStatus(*payouts),
			},
		}
	}
	return &stripe.V2CoreAccount{
		Configuration: &stripe.V2CoreAccountConfiguration{Merchant: &stripe.V2CoreAccountConfigurationMerchant{Capabilities: capabilities}},
	}
}

// TestAccountStatusFrom_ReadsBothCapabilities is the happy path: an
// account with both capabilities granted.
func TestAccountStatusFrom_ReadsBothCapabilities(t *testing.T) {
	payouts := "active"
	got := accountStatusFrom(merchantAccount("active", &payouts))

	if got.CardPayments != CapabilityActive || got.Payouts != CapabilityActive {
		t.Fatalf("statuses = (%q, %q), want both active", got.CardPayments, got.Payouts)
	}
	if got.RequirementsDue == nil {
		t.Fatalf("RequirementsDue = nil, want an empty slice so callers need no null-check")
	}
}

// TestAccountStatusFrom_MissingLevelsReadAsUnsupported proves each nil
// level -- no configuration at all, a merchant configuration with no
// capabilities, and a granted card_payments with no stripe_balance --
// reads as unsupported rather than panicking or reading as active.
// `include` controls what Stripe returns, so every level really can be
// absent.
func TestAccountStatusFrom_MissingLevelsReadAsUnsupported(t *testing.T) {
	cases := map[string]*stripe.V2CoreAccount{
		"nothing included":        {},
		"no merchant":             {Configuration: &stripe.V2CoreAccountConfiguration{}},
		"no capabilities":         {Configuration: &stripe.V2CoreAccountConfiguration{Merchant: &stripe.V2CoreAccountConfigurationMerchant{}}},
		"no card_payments":        {Configuration: &stripe.V2CoreAccountConfiguration{Merchant: &stripe.V2CoreAccountConfigurationMerchant{Capabilities: &stripe.V2CoreAccountConfigurationMerchantCapabilities{}}}},
		"no stripe_balance":       merchantAccount("active", nil),
		"stripe_balance no field": {Configuration: &stripe.V2CoreAccountConfiguration{Merchant: &stripe.V2CoreAccountConfigurationMerchant{Capabilities: &stripe.V2CoreAccountConfigurationMerchantCapabilities{StripeBalance: &stripe.V2CoreAccountConfigurationMerchantCapabilitiesStripeBalance{}}}}},
	}
	for name, acct := range cases {
		t.Run(name, func(t *testing.T) {
			got := accountStatusFrom(acct)
			if got.Payouts != CapabilityUnsupported {
				t.Fatalf("Payouts = %q, want %q", got.Payouts, CapabilityUnsupported)
			}
			if name != "no stripe_balance" && got.CardPayments != CapabilityUnsupported {
				t.Fatalf("CardPayments = %q, want %q", got.CardPayments, CapabilityUnsupported)
			}
		})
	}
}

// TestAccountStatusFrom_KeepsOnlyEntriesAwaitingTheAccountHolder is the
// decision in the mapping: Stripe also raises requirements awaiting
// itself (a review in progress) and awaiting the platform, and neither
// is something the Owner can clear by reopening onboarding. Only "user"
// entries belong in RequirementsDue.
func TestAccountStatusFrom_KeepsOnlyEntriesAwaitingTheAccountHolder(t *testing.T) {
	payouts := "restricted"
	acct := merchantAccount("restricted", &payouts)
	acct.Requirements = &stripe.V2CoreAccountRequirements{
		Entries: []*stripe.V2CoreAccountRequirementsEntry{
			{AwaitingActionFrom: stripe.V2CoreAccountRequirementsEntryAwaitingActionFromUser, Description: "configuration.merchant.mcc"},
			{AwaitingActionFrom: "stripe", Description: "identity.verification.document"},
			nil,
			{AwaitingActionFrom: stripe.V2CoreAccountRequirementsEntryAwaitingActionFromUser, Description: "configuration.merchant.support.phone"},
		},
	}

	got := accountStatusFrom(acct)

	want := []string{"configuration.merchant.mcc", "configuration.merchant.support.phone"}
	if len(got.RequirementsDue) != len(want) {
		t.Fatalf("RequirementsDue = %v, want %v", got.RequirementsDue, want)
	}
	for i, description := range want {
		if got.RequirementsDue[i] != description {
			t.Fatalf("RequirementsDue[%d] = %q, want %q", i, got.RequirementsDue[i], description)
		}
	}
}
