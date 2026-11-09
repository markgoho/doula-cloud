package portalinvite_test

import (
	"bytes"
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

func TestPostBounceWebhookHandler_RecordsAnArrivalForEachOutcome(t *testing.T) {
	e := newBounceEnv(t, bounceWebhookTestKey)
	_, email := seedSentOutboxRow(t, e.DB)

	for _, payload := range [][]byte{
		signedMailgunPayload(t, bounceWebhookTestKey, "failed", "permanent", email, "mg_bounce"),
		signedMailgunPayload(t, bounceWebhookTestKey, "delivered", "", email, "mg_delivered"),
		signedMailgunPayload(t, "wrong-key", "failed", "permanent", email, "mg_forged"),
		[]byte(`not json`),
		bytes.Repeat([]byte("a"), (1<<20)+1),
	} {
		resp := reqtest.Post(t, e.Srv.URL+"/mailgun/webhook", reqtest.Body("", payload))
		_ = resp.Body.Close()
	}

	got := []arrival{}
	for _, a := range testdb.WebhookArrivals(t, e.DB) {
		if a.Route != "mailgun" {
			t.Fatalf("arrival on route %q, want mailgun", a.Route)
		}
		got = append(got, arrival{webhookarrival.Outcome(a.Outcome), a.EventID, a.EventType})
	}
	want := []arrival{
		{webhookarrival.Accepted, "mg_bounce", "failed"},
		{webhookarrival.Dropped, "mg_delivered", "delivered"},
		{webhookarrival.SignatureRefused, "mg_forged", "failed"},
		{webhookarrival.BodyRefused, "", ""},
		{webhookarrival.BodyRefused, "", ""},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("arrivals =\n%v\nwant\n%v", got, want)
	}
}
