package payments

import (
	"encoding/json"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stripe/stripe-go/v86"
)

type stripeDestination struct {
	EventPayload       string   `json:"event_payload"`
	SnapshotAPIVersion *string  `json:"snapshot_api_version"`
	EnabledEvents      []string `json:"enabled_events"`
}

func readStripeDestinations(t *testing.T) map[string]stripeDestination {
	t.Helper()
	src, err := os.ReadFile("../../../terraform/stripe/destinations.json")
	if err != nil {
		t.Fatalf("read destinations.json: %v", err)
	}
	var destinations map[string]stripeDestination
	if err := json.Unmarshal(src, &destinations); err != nil {
		t.Fatalf("parse destinations.json: %v", err)
	}
	return destinations
}

func assertSameEvents(t *testing.T, route string, subscribed, handled []string) {
	t.Helper()
	for _, e := range handled {
		if !slices.Contains(subscribed, e) {
			t.Errorf("%s handles %q, but terraform/stripe/destinations.json does not subscribe it to that event", route, e)
		}
	}
	for _, e := range subscribed {
		if !slices.Contains(handled, e) {
			t.Errorf("terraform/stripe/destinations.json subscribes %s to %q, which no handler takes", route, e)
		}
	}
}

func TestStripeDestinationsMatchConnectAndAccountHandlers(t *testing.T) {
	destinations := readStripeDestinations(t)

	assertSameEvents(t, "/api/stripe/connect-webhook",
		destinations["/api/stripe/connect-webhook"].EnabledEvents,
		slices.Collect(maps.Keys(connectEventHandlers)))
	assertSameEvents(t, "/api/stripe/account-webhook",
		destinations["/api/stripe/account-webhook"].EnabledEvents,
		[]string{eventTypeMerchantCapabilityStatusUpdated})
}

func TestStripeSnapshotDestinationsPinTheSDKsAPIVersion(t *testing.T) {
	for route, d := range readStripeDestinations(t) {
		switch d.EventPayload {
		case "snapshot":
			if d.SnapshotAPIVersion == nil || *d.SnapshotAPIVersion != stripe.APIVersion {
				t.Errorf("%s: snapshot_api_version must be %q, the version stripe-go sends", route, stripe.APIVersion)
			}
		case "thin":
			if d.SnapshotAPIVersion != nil {
				t.Errorf("%s: a thin destination takes no snapshot_api_version", route)
			}
		default:
			t.Errorf("%s: event_payload %q is neither snapshot nor thin", route, d.EventPayload)
		}
	}
}

var listenEvents = regexp.MustCompile(`--events '([^']*)'`)

func TestStripeListenForwardsTheAccountRoutesThinEvents(t *testing.T) {
	src, err := os.ReadFile("../../../scripts/stripe-listen.sh")
	if err != nil {
		t.Fatalf("read stripe-listen.sh: %v", err)
	}
	matches := listenEvents.FindAllSubmatch(src, -1)
	if len(matches) != 1 {
		t.Fatalf("stripe-listen.sh has %d --events lists, want the one thin listener's", len(matches))
	}
	assertSameEvents(t, "stripe-listen.sh's thin listener",
		readStripeDestinations(t)["/api/stripe/account-webhook"].EnabledEvents,
		strings.Split(string(matches[0][1]), ","))
}
