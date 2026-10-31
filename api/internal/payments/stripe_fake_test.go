package payments_test

import (
	"errors"
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

// TestFakeClient_FailedPaymentMethodType proves the double reports the
// method a test set for an Invoice, and nothing for one it did not.
func TestFakeClient_FailedPaymentMethodType(t *testing.T) {
	client := payments.NewFakeClient()
	client.FailedPaymentMethodTypes = map[string]string{"in_1": methodUSBankAccount}

	if got, err := client.RetrieveInvoiceFailedPaymentMethodType(t.Context(), "acct_1", "in_1"); err != nil || got != methodUSBankAccount {
		t.Fatalf("RetrieveInvoiceFailedPaymentMethodType(in_1) = %q, %v, want us_bank_account", got, err)
	}
	if got, err := client.RetrieveInvoiceFailedPaymentMethodType(t.Context(), "acct_1", "in_2"); err != nil || got != "" {
		t.Fatalf("RetrieveInvoiceFailedPaymentMethodType(in_2) = %q, %v, want empty", got, err)
	}
}

// TestFakeClient_PaymentIntentInvoice proves the double reports what a
// test set for a PaymentIntent, and the zero value for one it did not.
func TestFakeClient_PaymentIntentInvoice(t *testing.T) {
	client := payments.NewFakeClient()
	want := payments.InvoicePaymentIntent{StripeInvoiceID: "in_1", Status: paymentIntentStatusProcessing, PaymentMethodType: methodUSBankAccount}
	client.PaymentIntentInvoices = map[string]payments.InvoicePaymentIntent{"pi_1": want}

	if got, err := client.RetrievePaymentIntentInvoice(t.Context(), "acct_1", "pi_1"); err != nil || got != want {
		t.Fatalf("RetrievePaymentIntentInvoice(pi_1) = %+v, %v, want %+v", got, err, want)
	}
	if got, err := client.RetrievePaymentIntentInvoice(t.Context(), "acct_1", "pi_2"); err != nil || got != (payments.InvoicePaymentIntent{}) {
		t.Fatalf("RetrievePaymentIntentInvoice(pi_2) = %+v, %v, want the zero value", got, err)
	}
}

// TestFakeClient_ErasureMethods exercises #394's two Stripe erasure acts
// on the fake. Their real callers live in package client (the erasure
// outbox worker), which cannot import this package -- payments already
// imports client -- so it drives its own double and these two go
// uncovered unless exercised here.
func TestFakeClient_ErasureMethods(t *testing.T) {
	client := payments.NewFakeClient()

	if err := client.DeleteCustomer(t.Context(), "acct_1", "cus_1"); err != nil {
		t.Fatalf("DeleteCustomer: %v", err)
	}
	if got := client.DeleteCustomerCalls; len(got) != 1 || got[0] != (payments.FakeCustomerCall{AccountID: "acct_1", CustomerID: "cus_1"}) {
		t.Fatalf("DeleteCustomerCalls = %+v, want one (acct_1, cus_1) pair", got)
	}

	jobID, err := client.CreateRedactionJob(t.Context(), "acct_1", "cus_1")
	if err != nil {
		t.Fatalf("CreateRedactionJob: %v", err)
	}
	if jobID == "" {
		t.Fatal("CreateRedactionJob returned an empty job id")
	}
	if got := client.RedactionJobCalls; len(got) != 1 || got[0] != (payments.FakeCustomerCall{AccountID: "acct_1", CustomerID: "cus_1"}) {
		t.Fatalf("RedactionJobCalls = %+v, want one (acct_1, cus_1) pair", got)
	}

	client.DeleteCustomerErr = errStripeFake
	if err := client.DeleteCustomer(t.Context(), "acct_1", "cus_2"); !errors.Is(err, errStripeFake) {
		t.Fatalf("DeleteCustomer err = %v, want the injected failure", err)
	}
	client.RedactionJobErr = errStripeFake
	if _, err := client.CreateRedactionJob(t.Context(), "acct_1", "cus_2"); !errors.Is(err, errStripeFake) {
		t.Fatalf("CreateRedactionJob err = %v, want the injected failure", err)
	}
	if len(client.DeleteCustomerCalls) != 1 || len(client.RedactionJobCalls) != 1 {
		t.Fatal("a failed call was still recorded, want the failure to record nothing")
	}
}
