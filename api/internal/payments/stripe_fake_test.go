package payments_test

import (
	"testing"

	"doula-cloud/api/internal/payments"
)

// TestFakeClient_UnwiredMethods exercises FakeClient's
// VerifyWebhookSignature directly -- no handler in this ticket calls it
// yet (#82's webhook extension does), but it's still production
// test-infrastructure code, not a real Stripe network call, so it's
// covered here rather than coverage:ignore'd. CreateInvoice and
// FinalizeInvoice are exercised through invoice_test.go's real handler
// calls instead, since PostInvoiceHandler now wires both.
func TestFakeClient_UnwiredMethods(t *testing.T) {
	client := payments.NewFakeClient()

	event, err := client.VerifyWebhookSignature([]byte(`{}`), "t=1,v1=deadbeef", "whsec_test")
	if err != nil {
		t.Fatalf("VerifyWebhookSignature: %v", err)
	}
	if event.ID != "" || event.Type != "" || event.Account != "" || event.Data != nil {
		t.Fatalf("event = %+v, want zero value", event)
	}
}
