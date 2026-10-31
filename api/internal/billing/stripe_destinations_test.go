package billing

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestStripeDestinationMatchesPurchaseHandler(t *testing.T) {
	src, err := os.ReadFile("../../../terraform/stripe/destinations.json")
	if err != nil {
		t.Fatalf("read destinations.json: %v", err)
	}
	var destinations map[string]struct {
		EnabledEvents []string `json:"enabled_events"`
	}
	if err := json.Unmarshal(src, &destinations); err != nil {
		t.Fatalf("parse destinations.json: %v", err)
	}

	subscribed := destinations["/api/stripe/webhook"].EnabledEvents
	if !slices.Equal(subscribed, []string{string(purchaseEventType)}) {
		t.Errorf("terraform/stripe/destinations.json subscribes /api/stripe/webhook to %q; PostPurchaseWebhookHandler takes only %q", subscribed, purchaseEventType)
	}
}
