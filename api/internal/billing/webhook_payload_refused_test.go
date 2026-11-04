package billing_test

import (
	"bytes"
	"log"
	"net/http"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/webhookarrival"
)

const (
	failingPaymentIntent = "payment_intent"
	failingSubtotal      = "amount_subtotal"
)

func TestPostPurchaseWebhookHandler_MarksAndLogsEachPayloadItCannotActOn(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Payload Refused")
	srv := newWebhookServer(db)
	defer srv.Close()

	var logged bytes.Buffer
	original := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(original) })

	cases := []struct {
		eventID, field string
		payload        []byte
	}{
		{"evt_no_practice", "metadata.practice_id", checkoutCompletedPayload(t, "evt_no_practice", "", 3, "paid")},
		{"evt_bad_quantity", "metadata.quantity", checkoutCompletedPayload(t, "evt_bad_quantity", practiceID, 0, "paid")},
		{"evt_uneven", failingSubtotal, pricedCheckoutPayload(t, "evt_uneven", practiceID, 3, 10000, 0, "pi_uneven")},
		{"evt_ghost", "metadata.practice_id: no such Practice", checkoutCompletedPayload(t, "evt_ghost", "00000000-0000-4000-8000-000000000000", 3, "paid")},
		{"evt_nopi", failingPaymentIntent, pricedCheckoutPayload(t, "evt_nopi", practiceID, 5, 10000, 0, nil)},
	}
	want := []arrival{}
	for _, c := range cases {
		resp := postWebhook(t, srv, c.payload, webhookTestSecret)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", c.eventID, resp.StatusCode)
		}
		line := "refusing event " + c.eventID + " (" + stripeEventTypeCheckoutCompleted + "): " + c.field
		if !strings.Contains(logged.String(), line) {
			t.Errorf("log = %q, want it to contain %q", logged.String(), line)
		}
		want = append(want, arrival{webhookarrival.PayloadRefused, c.eventID, stripeEventTypeCheckoutCompleted})
	}
	if got := arrivals(t, db); !slices.Equal(got, want) {
		t.Fatalf("arrivals =\n%v\nwant\n%v", got, want)
	}
}
