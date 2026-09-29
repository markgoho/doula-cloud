package payments_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// These tests drive the Client-portal pay-secret route (#1020): what the
// Payment Element mounts with, for one of her own open, Stripe-rail
// Invoices, and every way it refuses.

const payAccountID = "acct_pay_fixture"

func clientPaymentPath(engagementID, invoiceID string) string {
	return clientInvoicesPath(engagementID, "/"+invoiceID+"/payment")
}

func TestClientGetPayment_ReturnsTheSecretForHerOpenStripeInvoice(t *testing.T) {
	db := testdb.New(t)
	const uid = "pay-success"
	me := seedClientMoney(t, db, uid, "Jordan Client", "jordan@example.com")
	seedConnectAccount(t, db, me.practiceID, payAccountID)
	invoiceID := seedInvoice(t, db, me.practiceID, me.contractID, "in_pay_open", invoiceStatusOpen, 5000, time.Now())
	fake := payments.NewFakeClient()
	srv := newClientMoneyServerWith(t, db, fake)

	status, body := getPortal(t, srv, portalSession(t, db, uid), clientPaymentPath(me.engagementID, invoiceID))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, body)
	}
	var out payments.ClientPaymentResponse
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := payments.ClientPaymentResponse{
		ClientSecret:    payments.FakeClientSecret("in_pay_open"),
		StripeAccountID: payAccountID,
		PublishableKey:  testPublishableKey,
	}
	if out != want {
		t.Fatalf("response = %+v, want %+v", out, want)
	}
	if len(fake.ClientSecretCalls) != 1 || fake.ClientSecretCalls[0] != (payments.FakeClientSecretCall{AccountID: payAccountID, InvoiceID: "in_pay_open"}) {
		t.Fatalf("stripe calls = %+v, want one for her Invoice on the Practice's own account", fake.ClientSecretCalls)
	}
	// Nothing broader than the three fields crosses.
	for _, leak := range []string{"sk_", "stripe_customer", "contract"} {
		if strings.Contains(body, leak) {
			t.Fatalf("body leaks %q: %s", leak, body)
		}
	}
}

func TestClientGetPayment_SetsNoStoreOnTheCredential(t *testing.T) {
	db := testdb.New(t)
	const uid = "pay-no-store"
	me := seedClientMoney(t, db, uid, "Jordan Client", "jordan@example.com")
	seedConnectAccount(t, db, me.practiceID, payAccountID)
	invoiceID := seedInvoice(t, db, me.practiceID, me.contractID, "in_pay_cache", invoiceStatusOpen, 5000, time.Now())
	srv := newClientMoneyServer(t, db)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+clientPaymentPath(me.engagementID, invoiceID), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, portalSession(t, db, uid))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if resp.Header.Get("RateLimit-Limit") == "" {
		t.Fatal("no RateLimit-Limit header: the heavy endpoint is not rate limited")
	}
}
