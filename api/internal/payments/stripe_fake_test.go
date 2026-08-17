package payments_test

import (
	"testing"

	"doula-cloud/api/internal/payments"
)

// TestFakeClient_UnwiredMethods exercises FakeClient's CreateInvoice,
// FinalizeInvoice, and VerifyWebhookSignature directly -- no handler in
// this ticket calls them yet (#80/#81/#82 do), but they're still
// production test-infrastructure code, not a real Stripe network call, so
// they're covered here rather than coverage:ignore'd.
func TestFakeClient_UnwiredMethods(t *testing.T) {
	client := payments.NewFakeClient()

	invoiceID, err := client.CreateInvoice(t.Context(), "acct_fake_1", "client@example.com", "Client Name", 5000)
	if err != nil {
		t.Fatalf("CreateInvoice: %v", err)
	}
	if invoiceID == "" {
		t.Fatal("CreateInvoice returned an empty invoice id")
	}

	hostedURL, err := client.FinalizeInvoice(t.Context(), "acct_fake_1", invoiceID)
	if err != nil {
		t.Fatalf("FinalizeInvoice: %v", err)
	}
	if hostedURL == "" {
		t.Fatal("FinalizeInvoice returned an empty hosted invoice URL")
	}

	event, err := client.VerifyWebhookSignature([]byte(`{}`), "t=1,v1=deadbeef", "whsec_test")
	if err != nil {
		t.Fatalf("VerifyWebhookSignature: %v", err)
	}
	if event.ID != "" || event.Type != "" || event.Account != "" || event.Data != nil {
		t.Fatalf("event = %+v, want zero value", event)
	}
}
