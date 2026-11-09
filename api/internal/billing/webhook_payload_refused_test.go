package billing_test

import (
	"bytes"
	"log"
	"net/http"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/webhookarrival"
)

const (
	failingPaymentIntent = "payment_intent"
	failingSubtotal      = "amount_subtotal"
)

func TestPostPurchaseWebhookHandler_AnOtherwiseFailedLedgerInsertStaysA500(t *testing.T) {
	e := newWebhookEnv(t)

	e.deliver(pricedCheckoutPayload(t, "evt_negative_tax", e.Practice, 5, 10000, -1, "pi_negative_tax"), webhookTestSecret, http.StatusInternalServerError)

	if got := arrivals(t, e.DB); !slices.Equal(got, []arrival{{webhookarrival.Failed, "evt_negative_tax", stripeEventTypeCheckoutCompleted}}) {
		t.Fatalf("arrivals = %v, want one failed", got)
	}
}

func TestPostPurchaseWebhookHandler_MarksAndLogsEachPayloadItCannotActOn(t *testing.T) {
	e := newWebhookEnv(t)
	var logged bytes.Buffer
	original := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(original) })

	cases := []struct {
		eventID, field string
		payload        []byte
	}{
		{"evt_no_practice", "metadata.practice_id", checkoutCompletedPayload(t, "evt_no_practice", "", 3, paidStatus)},
		{"evt_bad_quantity", "metadata.quantity", checkoutCompletedPayload(t, "evt_bad_quantity", e.Practice, 0, paidStatus)},
		{"evt_uneven", failingSubtotal, pricedCheckoutPayload(t, "evt_uneven", e.Practice, 3, 10000, 0, "pi_uneven")},
		{"evt_ghost", "metadata.practice_id: no such Practice", checkoutCompletedPayload(t, "evt_ghost", "00000000-0000-4000-8000-000000000000", 3, paidStatus)},
		{"evt_nopi", failingPaymentIntent, pricedCheckoutPayload(t, "evt_nopi", e.Practice, 5, 10000, 0, nil)},
	}
	want := []arrival{}
	for _, c := range cases {
		e.deliver(c.payload, webhookTestSecret, http.StatusBadRequest)
		line := "refusing event " + c.eventID + " (" + stripeEventTypeCheckoutCompleted + "): " + c.field
		if !strings.Contains(logged.String(), line) {
			t.Errorf("log = %q, want it to contain %q", logged.String(), line)
		}
		want = append(want, arrival{webhookarrival.PayloadRefused, c.eventID, stripeEventTypeCheckoutCompleted})
	}
	if got := arrivals(t, e.DB); !slices.Equal(got, want) {
		t.Fatalf("arrivals =\n%v\nwant\n%v", got, want)
	}
}
