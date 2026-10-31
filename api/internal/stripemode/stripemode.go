// Package stripemode reads which Stripe mode, test or live, the BFF's keys
// are in, and refuses a configuration whose keys or environment disagree
// about it (#2361). An error names the variables involved and never a
// key's value.
package stripemode

import (
	"errors"
	"fmt"
	"strings"
)

// Mode is the Stripe mode the BFF's keys are in. The zero value means no
// STRIPE_API_KEY is configured.
type Mode string

// The two modes a Stripe key can be in.
const (
	Test Mode = "test"
	Live Mode = "live"
)

// The variables Load reads.
const (
	APIKeyVar         = "STRIPE_API_KEY" //nolint:gosec // a variable name, not a credential
	PublishableKeyVar = "STRIPE_PUBLISHABLE_KEY"
	ModeVar           = "STRIPE_MODE"
	CloudRunVar       = "K_SERVICE"
)

// Matches reports whether an event's livemode flag belongs to m. With no
// key configured nothing can be checked, so every event matches.
func (m Mode) Matches(livemode bool) bool {
	if m == "" {
		return true
	}
	return livemode == (m == Live)
}

func keyMode(value string, test, live []string) (Mode, bool) {
	for _, p := range test {
		if strings.HasPrefix(value, p) {
			return Test, true
		}
	}
	for _, p := range live {
		if strings.HasPrefix(value, p) {
			return Live, true
		}
	}
	return "", false
}

// Load reads the mode from getenv, refusing an unrecognized key prefix, a
// publishable key in the other mode, a STRIPE_MODE that is invalid or
// disagrees with the key, a Cloud Run service with no STRIPE_MODE or no
// publishable key, and a live key anywhere but Cloud Run.
func Load(getenv func(string) string) (Mode, error) {
	declared := Mode(strings.TrimSpace(getenv(ModeVar)))
	if declared != "" && declared != Test && declared != Live {
		return "", fmt.Errorf("%s must be %q or %q", ModeVar, Test, Live)
	}

	apiKey := strings.TrimSpace(getenv(APIKeyVar))
	if apiKey == "" {
		return "", nil
	}
	mode, ok := keyMode(apiKey, []string{"sk_test_", "rk_test_"}, []string{"sk_live_", "rk_live_"})
	if !ok {
		return "", fmt.Errorf("%s has an unrecognized prefix: want sk_test_, rk_test_, sk_live_ or rk_live_", APIKeyVar)
	}

	onCloudRun := getenv(CloudRunVar) != ""
	publishable := strings.TrimSpace(getenv(PublishableKeyVar))
	if onCloudRun && publishable == "" {
		return "", fmt.Errorf("%s must be set on Cloud Run (%s is set)", PublishableKeyVar, CloudRunVar)
	}
	if publishable != "" {
		pkMode, ok := keyMode(publishable, []string{"pk_test_"}, []string{"pk_live_"})
		if !ok {
			return "", fmt.Errorf("%s has an unrecognized prefix: want pk_test_ or pk_live_", PublishableKeyVar)
		}
		if pkMode != mode {
			return "", fmt.Errorf("%s is a %s key but %s is a %s key", APIKeyVar, mode, PublishableKeyVar, pkMode)
		}
	}

	if onCloudRun && declared == "" {
		return "", fmt.Errorf("%s must be set on Cloud Run (%s is set)", ModeVar, CloudRunVar)
	}
	if declared != "" && declared != mode {
		return "", fmt.Errorf("%s is %s but %s is a %s key", ModeVar, declared, APIKeyVar, mode)
	}
	if !onCloudRun && mode == Live {
		return "", errors.New(APIKeyVar + " is a live key but " + CloudRunVar + " is unset: only Cloud Run runs on the live account")
	}
	return mode, nil
}
