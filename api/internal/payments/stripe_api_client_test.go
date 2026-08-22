package payments_test

import (
	"testing"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/payments"
)

//nolint:gosec // test fixture value, not a real credential
const webhookTestSecret = "whsec_test_secret"

// TestStripeAPIClient_VerifyWebhookSignature_Valid proves a correctly
// signed payload verifies and decodes into a WebhookEvent -- pure HMAC
// computation, no network call, so this is exercised for real rather than
// coverage:ignore'd like this file's Stripe API calls.
func TestStripeAPIClient_VerifyWebhookSignature_Valid(t *testing.T) {
	payload := []byte(`{"id":"evt_test","object":"event","type":"account.updated","account":"acct_test","data":{"object":{"id":"acct_test","object":"account"}}}`)
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: webhookTestSecret})

	client := payments.NewStripeAPIClient("sk_test_unused", "https://app.test")
	event, err := client.VerifyWebhookSignature(payload, signed.Header, webhookTestSecret)
	if err != nil {
		t.Fatalf("VerifyWebhookSignature: %v", err)
	}
	if event.ID != "evt_test" || event.Type != "account.updated" || event.Account != "acct_test" {
		t.Fatalf("event = %+v, want id evt_test, type account.updated, account acct_test", event)
	}
	if len(event.Data) == 0 {
		t.Fatal("event.Data is empty, want the raw data.object payload")
	}
}

// TestStripeAPIClient_VerifyWebhookSignature_NoDataField proves an event
// payload with no "data" field (which real Stripe events never send, but
// the type doesn't forbid) decodes to an empty Data rather than panicking.
func TestStripeAPIClient_VerifyWebhookSignature_NoDataField(t *testing.T) {
	payload := []byte(`{"id":"evt_test","object":"event","type":"account.updated"}`)
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: webhookTestSecret})

	client := payments.NewStripeAPIClient("sk_test_unused", "https://app.test")
	event, err := client.VerifyWebhookSignature(payload, signed.Header, webhookTestSecret)
	if err != nil {
		t.Fatalf("VerifyWebhookSignature: %v", err)
	}
	if event.Data != nil {
		t.Fatalf("event.Data = %v, want nil", event.Data)
	}
}

// TestStripeAPIClient_VerifyWebhookSignature_InvalidSignatureRejected
// proves a payload signed with the wrong secret is rejected.
func TestStripeAPIClient_VerifyWebhookSignature_InvalidSignatureRejected(t *testing.T) {
	payload := []byte(`{"id":"evt_test","object":"event","type":"account.updated"}`)
	//nolint:gosec // test fixture value, not a real credential
	const wrongSecret = "whsec_wrong_secret"
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: wrongSecret})

	client := payments.NewStripeAPIClient("sk_test_unused", "https://app.test")
	if _, err := client.VerifyWebhookSignature(payload, signed.Header, webhookTestSecret); err == nil {
		t.Fatal("expected an error verifying a payload signed with the wrong secret, got nil")
	}
}

// TestParseAccountEvent_ReadsHeaderAndRelatedObject proves the thin-event
// path: a v2 notification carries no object, so the only things to
// recover are the event's own id and type and the id of the account it
// points at. stripe.ConstructEvent -- the snapshot path above -- rejects
// this same payload, which is why the second method exists at all (#247).
func TestParseAccountEvent_ReadsHeaderAndRelatedObject(t *testing.T) {
	client := payments.NewStripeAPIClient("sk_test_unused", "https://app.test")
	payload := []byte(`{"id":"evt_thin_1","object":"v2.core.event","type":"v2.core.account[configuration.merchant].capability_status_updated","created":"2026-08-22T23:24:51.000Z","livemode":false,"related_object":{"id":"acct_thin_1","type":"v2.core.account","url":"/v2/core/accounts/acct_thin_1"}}`)
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: webhookTestSecret})

	event, err := client.ParseAccountEvent(payload, signed.Header, webhookTestSecret)
	if err != nil {
		t.Fatalf("ParseAccountEvent: %v", err)
	}
	if event.ID != "evt_thin_1" {
		t.Fatalf("ID = %q, want %q", event.ID, "evt_thin_1")
	}
	if event.Type != "v2.core.account[configuration.merchant].capability_status_updated" {
		t.Fatalf("Type = %q, want the merchant capability_status_updated type", event.Type)
	}
	if event.AccountID != "acct_thin_1" {
		t.Fatalf("AccountID = %q, want %q", event.AccountID, "acct_thin_1")
	}
}

// TestParseAccountEvent_RejectsWrongSecret proves a thin event signed
// with another destination's secret is refused -- the two Connect
// surfaces have separate secrets precisely because one destination
// cannot carry both payload types.
func TestParseAccountEvent_RejectsWrongSecret(t *testing.T) {
	client := payments.NewStripeAPIClient("sk_test_unused", "https://app.test")
	payload := []byte(`{"id":"evt_thin_2","object":"v2.core.event","type":"v2.core.account.created","created":"2026-08-22T23:24:51.000Z","related_object":{"id":"acct_thin_2","type":"v2.core.account","url":"/v2/core/accounts/acct_thin_2"}}`)
	// #nosec G101 -- a made-up signing secret for a test fixture, standing in for the *other* destination's secret
	const otherDestinationSecret = "whsec_other_destination"
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: otherDestinationSecret})

	if _, err := client.ParseAccountEvent(payload, signed.Header, webhookTestSecret); err == nil {
		t.Fatal("ParseAccountEvent accepted a payload signed with another secret, want an error")
	}
}
