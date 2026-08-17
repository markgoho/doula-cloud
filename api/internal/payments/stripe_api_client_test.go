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

// TestStripeAPIClient_InvoicingNotImplemented proves CreateInvoice and
// FinalizeInvoice -- #80/#82's territory, not this ticket's -- fail
// clearly rather than silently no-opping if called before those tickets
// land.
func TestStripeAPIClient_InvoicingNotImplemented(t *testing.T) {
	client := payments.NewStripeAPIClient("sk_test_unused", "https://app.test")

	if _, err := client.CreateInvoice(t.Context(), "acct_test", "client@example.com", "Client Name", 5000); err == nil {
		t.Fatal("expected CreateInvoice to return an error, got nil")
	}
	if _, err := client.FinalizeInvoice(t.Context(), "acct_test", "in_test"); err == nil {
		t.Fatal("expected FinalizeInvoice to return an error, got nil")
	}
}
