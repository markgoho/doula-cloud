package main

import (
	"testing"

	"doula-cloud/api/internal/stripemode"
)

func TestStripeKeyMode_ReadsTheKeysPrefixAndIsEmptyForNoKey(t *testing.T) {
	cases := map[string]stripemode.Mode{
		"sk_test_abc": stripemode.Test,
		"rk_live_abc": stripemode.Live,
		"":            "",
		"whsec_abc":   "",
	}
	for key, want := range cases {
		if got := stripeKeyMode(key); got != want {
			t.Errorf("stripeKeyMode(%q) = %q, want %q", key, got, want)
		}
	}
}
