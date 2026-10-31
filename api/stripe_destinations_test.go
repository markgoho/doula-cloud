package main

import (
	"encoding/json"
	"maps"
	"os"
	"regexp"
	"slices"
	"testing"
)

var stripeRoute = regexp.MustCompile(`"POST (/api/stripe/[^"]+)"`)

// Each route's events are held to its handler in internal/billing and
// internal/payments; this holds the set of routes to routes_webhook.go.
func TestStripeDestinationsCoverEveryStripeRoute(t *testing.T) {
	src, err := os.ReadFile("routes_webhook.go")
	if err != nil {
		t.Fatalf("read routes_webhook.go: %v", err)
	}
	var mounted []string
	for _, m := range stripeRoute.FindAllSubmatch(src, -1) {
		mounted = append(mounted, string(m[1]))
	}

	raw, err := os.ReadFile("../terraform/stripe/destinations.json")
	if err != nil {
		t.Fatalf("read destinations.json: %v", err)
	}
	var destinations map[string]json.RawMessage
	if err := json.Unmarshal(raw, &destinations); err != nil {
		t.Fatalf("parse destinations.json: %v", err)
	}

	slices.Sort(mounted)
	declared := slices.Sorted(maps.Keys(destinations))
	if !slices.Equal(mounted, declared) {
		t.Errorf("routes_webhook.go mounts %q; terraform/stripe/destinations.json declares %q", mounted, declared)
	}
}
