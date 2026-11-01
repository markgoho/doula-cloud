package billing_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/webhookarrival"
)

type arrival struct {
	outcome            webhookarrival.Outcome
	eventID, eventType string
}

func arrivals(t *testing.T, db *testdb.DB) []arrival {
	t.Helper()
	out := []arrival{}
	for _, a := range testdb.WebhookArrivals(t, db) {
		if a.Route != "stripe_platform" {
			t.Fatalf("arrival on route %q, want stripe_platform", a.Route)
		}
		out = append(out, arrival{webhookarrival.Outcome(a.Outcome), a.EventID, a.EventType})
	}
	return out
}

func TestPostPurchaseWebhookHandler_RecordsAnArrivalForEachOutcome(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Arrival Practice")
	srv := newWebhookServer(db)
	defer srv.Close()

	post := func(payload []byte, secret string) {
		resp := postWebhook(t, srv, payload, secret)
		_ = resp.Body.Close()
	}
	post(checkoutCompletedPayload(t, "evt_paid", practiceID, 3, "paid"), webhookTestSecret)
	post(otherEventPayload(t, "evt_other"), webhookTestSecret)
	post(checkoutCompletedPayload(t, "evt_forged", practiceID, 3, "paid"), "whsec_wrong")

	var live map[string]any
	if err := json.Unmarshal(checkoutCompletedPayload(t, "evt_live", practiceID, 3, "paid"), &live); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	live["livemode"] = true
	livePayload, err := json.Marshal(live)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	post(livePayload, webhookTestSecret)
	post(checkoutCompletedPayload(t, "evt_bad_quantity", practiceID, 0, "paid"), webhookTestSecret)
	post(buildEventPayload(t, "evt_malformed", stripeEventTypeCheckoutCompleted, map[string]any{metadataKey: 12345}), webhookTestSecret)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/stripe/webhook", bytes.NewReader(bytes.Repeat([]byte("a"), (1<<20)+1)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()

	want := []arrival{
		{webhookarrival.Accepted, "evt_paid", stripeEventTypeCheckoutCompleted},
		{webhookarrival.Dropped, "evt_other", "payment_intent.succeeded"},
		{webhookarrival.SignatureRefused, "evt_forged", stripeEventTypeCheckoutCompleted},
		{webhookarrival.LivemodeRefused, "evt_live", stripeEventTypeCheckoutCompleted},
		{webhookarrival.Refused, "evt_bad_quantity", stripeEventTypeCheckoutCompleted},
		{webhookarrival.Failed, "evt_malformed", stripeEventTypeCheckoutCompleted},
		{webhookarrival.BodyRefused, "", ""},
	}
	if got := arrivals(t, db); !slices.Equal(got, want) {
		t.Fatalf("arrivals =\n%v\nwant\n%v", got, want)
	}
}
