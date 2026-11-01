package payments_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/webhookarrival"
)

type arrival struct {
	outcome            webhookarrival.Outcome
	eventID, eventType string
}

func arrivalsOn(t *testing.T, db *testdb.DB, route string) []arrival {
	t.Helper()
	out := []arrival{}
	for _, a := range testdb.WebhookArrivals(t, db) {
		if a.Route != route {
			t.Fatalf("arrival on route %q, want %s", a.Route, route)
		}
		out = append(out, arrival{webhookarrival.Outcome(a.Outcome), a.EventID, a.EventType})
	}
	return out
}

func postUnsigned(t *testing.T, srv *httptest.Server, path string, body []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()
}

func TestPostConnectWebhookHandler_RecordsAnArrivalForEachOutcome(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Connect Arrival Practice", "acct_arrival")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	seedInvoice(t, db, practiceID, contractID, "in_arrival", invoiceStatusOpen, 5000, time.Now())
	srv := newConnectWebhookServerWith(db, &referenceClient{
		StripeAPIClient: payments.NewStripeAPIClient("sk_test_unused", "https://app.test"),
		reference:       "pi_arrival",
	})
	defer srv.Close()

	post := func(payload []byte, secret string) {
		resp := postConnectWebhook(t, srv, payload, secret)
		_ = resp.Body.Close()
	}
	post(invoicePaidPayload(t, "evt_paid", "acct_arrival", "in_arrival", time.Now()), stripeConnectWebhookSecret)
	post(otherConnectEventPayload(t, "evt_other", "acct_arrival"), stripeConnectWebhookSecret)
	post(invoicePaidPayload(t, "evt_unknown", "acct_arrival", "in_unknown", time.Now()), stripeConnectWebhookSecret)
	post(invoicePaidPayload(t, "evt_forged", "acct_arrival", "in_arrival", time.Now()), "whsec_wrong")
	post(asLive(t, invoicePaidPayload(t, "evt_live", "acct_arrival", "in_arrival", time.Now())), stripeConnectWebhookSecret)
	post(buildConnectEventPayload(t, "evt_malformed", stripeEventTypeInvoiceMarkedUncollectible, "acct_arrival", map[string]any{"id": 12345}), stripeConnectWebhookSecret)
	postUnsigned(t, srv, "/stripe/connect-webhook", bytes.Repeat([]byte("a"), (1<<20)+1))

	want := []arrival{
		{webhookarrival.Accepted, "evt_paid", stripeEventTypeInvoicePaid},
		{webhookarrival.Dropped, "evt_other", stripeEventTypeUnhandled},
		{webhookarrival.Dropped, "evt_unknown", stripeEventTypeInvoicePaid},
		{webhookarrival.SignatureRefused, "evt_forged", stripeEventTypeInvoicePaid},
		{webhookarrival.LivemodeRefused, "evt_live", stripeEventTypeInvoicePaid},
		{webhookarrival.Failed, "evt_malformed", stripeEventTypeInvoiceMarkedUncollectible},
		{webhookarrival.BodyRefused, "", ""},
	}
	if got := arrivalsOn(t, db, "stripe_connect"); !slices.Equal(got, want) {
		t.Fatalf("arrivals =\n%v\nwant\n%v", got, want)
	}
}

func TestPostAccountWebhookHandler_RecordsAnArrivalForEachOutcome(t *testing.T) {
	db := testdb.New(t)
	seedConnectedPractice(t, db, "Account Arrival Practice", "acct_account_arrival")
	client := activeAccountClient()
	srv := newAccountWebhookServer(db, client)
	defer srv.Close()

	post := func(payload []byte, secret string) {
		resp := postAccountWebhook(t, srv, payload, secret)
		_ = resp.Body.Close()
	}
	post(capabilityStatusPayload(t, "evt_applied", "acct_account_arrival"), stripeAccountWebhookSecret)
	post(accountEventPayload(t, "evt_created", stripeEventTypeAccountCreated, "acct_account_arrival"), stripeAccountWebhookSecret)
	post(capabilityStatusPayload(t, "evt_unrecognized", "acct_unrecognized"), stripeAccountWebhookSecret)
	post(capabilityStatusPayload(t, "evt_forged", "acct_account_arrival"), "whsec_wrong")
	post(asLive(t, capabilityStatusPayload(t, "evt_live", "acct_account_arrival")), stripeAccountWebhookSecret)
	client.err = errRetrieveFailed
	post(capabilityStatusPayload(t, "evt_unreachable", "acct_account_arrival"), stripeAccountWebhookSecret)
	postUnsigned(t, srv, "/stripe/account-webhook", bytes.Repeat([]byte("a"), (1<<20)+1))

	want := []arrival{
		{webhookarrival.Accepted, "evt_applied", stripeEventTypeCapabilityStatus},
		{webhookarrival.Dropped, "evt_created", stripeEventTypeAccountCreated},
		{webhookarrival.Dropped, "evt_unrecognized", stripeEventTypeCapabilityStatus},
		{webhookarrival.SignatureRefused, "evt_forged", stripeEventTypeCapabilityStatus},
		{webhookarrival.LivemodeRefused, "evt_live", stripeEventTypeCapabilityStatus},
		{webhookarrival.Failed, "evt_unreachable", stripeEventTypeCapabilityStatus},
		{webhookarrival.BodyRefused, "", ""},
	}
	if got := arrivalsOn(t, db, "stripe_account"); !slices.Equal(got, want) {
		t.Fatalf("arrivals =\n%v\nwant\n%v", got, want)
	}
}
