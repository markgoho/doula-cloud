package billing_test

import (
	"bytes"
	"net/http"
	"slices"
	"testing"

	"doula-cloud/api/internal/reqtest"
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
	e := newWebhookEnv(t)

	e.deliver(checkoutCompletedPayload(t, "evt_paid", e.Practice, 3, paidStatus), webhookTestSecret, http.StatusOK)
	e.deliver(buildEventPayload(t, "evt_other", "payment_intent.succeeded", map[string]any{"id": "pi_test", objectKey: "payment_intent"}), webhookTestSecret, http.StatusOK)
	e.deliver(checkoutCompletedPayload(t, "evt_forged", e.Practice, 3, paidStatus), "whsec_wrong", http.StatusBadRequest)
	e.deliver(liveEventPayload(t, e.Practice), webhookTestSecret, http.StatusBadRequest)
	e.deliver(checkoutCompletedPayload(t, "evt_bad_quantity", e.Practice, 0, paidStatus), webhookTestSecret, http.StatusBadRequest)
	e.deliver(buildEventPayload(t, "evt_malformed", stripeEventTypeCheckoutCompleted, map[string]any{metadataKey: 12345}), webhookTestSecret, http.StatusInternalServerError)
	reqtest.ExpectStatus(t, http.MethodPost, e.Srv.URL+"/stripe/webhook", http.StatusBadRequest, reqtest.Body("", bytes.Repeat([]byte("a"), (1<<20)+1)))

	want := []arrival{
		{webhookarrival.Accepted, "evt_paid", stripeEventTypeCheckoutCompleted},
		{webhookarrival.Dropped, "evt_other", "payment_intent.succeeded"},
		{webhookarrival.SignatureRefused, "evt_forged", stripeEventTypeCheckoutCompleted},
		{webhookarrival.LivemodeRefused, "evt_live", stripeEventTypeCheckoutCompleted},
		{webhookarrival.PayloadRefused, "evt_bad_quantity", stripeEventTypeCheckoutCompleted},
		{webhookarrival.Failed, "evt_malformed", stripeEventTypeCheckoutCompleted},
		{webhookarrival.BodyRefused, "", ""},
	}
	if got := arrivals(t, e.DB); !slices.Equal(got, want) {
		t.Fatalf("arrivals =\n%v\nwant\n%v", got, want)
	}
}
