package billing_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/stripemode"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/webhookarrival"
)

//nolint:gosec // test fixture value, not a real credential
const webhookTestSecret = "whsec_test_secret"

const (
	stripeObjectEvent                = "event"
	stripeObjectCheckoutSession      = "checkout.session"
	stripeEventTypeCheckoutCompleted = "checkout.session.completed"
	stripeTestSessionID              = "cs_test_session"
	stripeTestPaymentIntentID        = "pi_test_payment"
	paymentStatusKey                 = "payment_status"
	practiceIDKey                    = "practice_id"
	quantityKey                      = "quantity"
	metadataKey                      = "metadata"
	objectKey                        = "object"

	paidStatus = "paid"
)

// webhookEnv is what a webhook test starts from: a Practice and a server
// carrying the Stripe webhook route. Nobody is signed in; the signature
// is the whole credential.
type webhookEnv struct {
	T        *testing.T
	DB       *testdb.DB
	Srv      *httptest.Server
	Practice string
}

func newWebhookEnv(t *testing.T) *webhookEnv {
	t.Helper()
	db := testdb.New(t)
	mux := http.NewServeMux()
	mux.Handle("POST /stripe/webhook", webhookarrival.Record(db.App, webhookarrival.StripePlatform, billing.PostPurchaseWebhookHandler(db.App, webhookTestSecret, stripemode.Test)))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &webhookEnv{T: t, DB: db, Srv: srv, Practice: testdb.SeedPractice(t, db, "Webhook Practice")}
}

// deliver posts payload signed with signingSecret and requires the status.
func (e *webhookEnv) deliver(payload []byte, signingSecret string, want int) {
	e.T.Helper()
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: signingSecret})
	reqtest.ExpectStatus(e.T, http.MethodPost, e.Srv.URL+"/stripe/webhook", want,
		reqtest.Body("", payload), reqtest.Header("Stripe-Signature", signed.Header))
}

// credited is the Practice's ledger sum, what the webhook has added to it.
func (e *webhookEnv) credited() int {
	e.T.Helper()
	return testdb.Scalar[int](e.T, e.DB.Admin, `SELECT COALESCE(SUM(quantity), 0) FROM credit_ledger WHERE practice_id = $1`, e.Practice)
}

// buildEventPayload assembles a raw Stripe event envelope around
// dataObject -- the shape every test payload in this file shares.
func buildEventPayload(t *testing.T, eventID, eventType string, dataObject map[string]any) []byte {
	t.Helper()
	body := map[string]any{
		"id":      eventID,
		objectKey: stripeObjectEvent,
		"type":    eventType,
		"data":    map[string]any{objectKey: dataObject},
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

// checkoutCompletedPayload builds a raw checkout.session.completed event
// body, tagged with practiceID and quantity in the Checkout Session's
// metadata the same way StripeAPIClient.CreateCheckoutSession sets them.
func checkoutCompletedPayload(t *testing.T, eventID, practiceID string, quantity int, paymentStatus string) []byte {
	t.Helper()
	return buildEventPayload(t, eventID, stripeEventTypeCheckoutCompleted, map[string]any{
		"id":             stripeTestSessionID,
		objectKey:        stripeObjectCheckoutSession,
		paymentStatusKey: paymentStatus,
		// What the lot cost and the payment it arrived on: the webhook
		// records both on the purchase row (#420), so a Session without
		// them is not a Session this handler will credit.
		"amount_subtotal": seedUnitPriceCents * quantity,
		"payment_intent":  stripeTestPaymentIntentID,
		metadataKey: map[string]any{
			practiceIDKey: practiceID,
			quantityKey:   strconv.Itoa(quantity),
		},
	})
}

// pricedCheckoutPayload builds a paid checkout.session.completed event
// with an explicit subtotal, tax and payment -- the three facts #420's
// purchase row records, so a refund can be computed from the ledger
// alone.
func pricedCheckoutPayload(t *testing.T, eventID, practiceID string, quantity int, subtotal, tax int64, paymentIntent any) []byte {
	t.Helper()
	session := map[string]any{
		"id":              stripeTestSessionID,
		objectKey:         stripeObjectCheckoutSession,
		paymentStatusKey:  paidStatus,
		"amount_subtotal": subtotal,
		"total_details":   map[string]any{"amount_tax": tax},
		metadataKey: map[string]any{
			practiceIDKey: practiceID,
			quantityKey:   strconv.Itoa(quantity),
		},
	}
	if paymentIntent != nil {
		session["payment_intent"] = paymentIntent
	}
	return buildEventPayload(t, eventID, stripeEventTypeCheckoutCompleted, session)
}

// TestPostPurchaseWebhookHandler_CreditsLedgerOnConfirmedCheckout proves a
// validly-signed, paid checkout.session.completed event appends a +N
// purchase row for the Practice named in its metadata, recording the
// price paid, the tax charged and the payment it arrived on -- the three
// things a refund three years later is computed from.
func TestPostPurchaseWebhookHandler_CreditsLedgerOnConfirmedCheckout(t *testing.T) {
	e := newWebhookEnv(t)

	e.deliver(pricedCheckoutPayload(t, "evt_credit_once", e.Practice, 5, 10000, 686, "pi_recorded"), webhookTestSecret, http.StatusOK)

	var origin, paymentIntent string
	var quantity, unitPrice, tax int
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT origin, quantity, unit_price_cents, tax_cents, stripe_payment_intent_id FROM credit_ledger
		 WHERE practice_id = $1`, e.Practice,
	).Scan(&origin, &quantity, &unitPrice, &tax, &paymentIntent); err != nil {
		t.Fatalf("query ledger row: %v", err)
	}
	if origin != originPurchase || quantity != 5 || unitPrice != 2000 || tax != 686 || paymentIntent != "pi_recorded" {
		t.Fatalf("ledger row = (%q, %d, %d, %d, %s), want (purchase, 5, 2000, 686, pi_recorded)", origin, quantity, unitPrice, tax, paymentIntent)
	}
}

// TestPostPurchaseWebhookHandler_ReplayedEventCreditsExactlyOnce is the
// explicit idempotency test AC #3 calls for: replaying the same Stripe
// event id must not double-credit.
func TestPostPurchaseWebhookHandler_ReplayedEventCreditsExactlyOnce(t *testing.T) {
	e := newWebhookEnv(t)
	payload := checkoutCompletedPayload(t, "evt_replayed", e.Practice, 4, paidStatus)

	e.deliver(payload, webhookTestSecret, http.StatusOK)
	e.deliver(payload, webhookTestSecret, http.StatusOK)

	if got := e.credited(); got != 4 {
		t.Fatalf("ledger sum after replay = %d, want 4 (credited exactly once)", got)
	}
}

// liveEventPayload is a validly signed checkout that claims to be live.
func liveEventPayload(t *testing.T, practiceID string) []byte {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(checkoutCompletedPayload(t, "evt_live", practiceID, 3, paidStatus), &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	body["livemode"] = true
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return payload
}

// TestPostPurchaseWebhookHandler_NeverCredits covers every delivery that
// must leave the ledger as it was, with the status Stripe is told.
func TestPostPurchaseWebhookHandler_NeverCredits(t *testing.T) {
	cases := []struct {
		name    string
		payload func(t *testing.T, practiceID string) []byte
		// secret signs the payload; empty is webhookTestSecret.
		secret string
		want   int
	}{
		{
			name: "a payload signed with the wrong secret",
			payload: func(t *testing.T, id string) []byte {
				return checkoutCompletedPayload(t, "evt_bad_sig", id, 9, paidStatus)
			},
			secret: "whsec_wrong_secret",
			want:   http.StatusBadRequest,
		},
		{name: "a live event on test keys", payload: liveEventPayload, want: http.StatusBadRequest},
		{
			// An async payment method still pending.
			name: "a completed session that is not paid",
			payload: func(t *testing.T, id string) []byte {
				return checkoutCompletedPayload(t, "evt_unpaid", id, 6, "unpaid")
			},
			want: http.StatusOK,
		},
		{
			// payment_intent.succeeded fires alongside the checkout for the
			// same purchase; crediting off it would credit twice under two
			// event ids.
			name: "an event type other than checkout.session.completed",
			payload: func(t *testing.T, _ string) []byte {
				return buildEventPayload(t, "evt_payment_intent", "payment_intent.succeeded", map[string]any{"id": "pi_test", objectKey: "payment_intent"})
			},
			want: http.StatusOK,
		},
		{
			name: "a practice_id that is not a uuid",
			payload: func(t *testing.T, _ string) []byte {
				return checkoutCompletedPayload(t, "evt_bad_practice", "not-a-uuid", 3, paidStatus)
			},
			want: http.StatusBadRequest,
		},
		{
			// metadata as a number (instead of an object of strings) fails
			// to unmarshal into stripe.CheckoutSession.Metadata.
			name: "a session object that cannot be unmarshaled",
			payload: func(t *testing.T, _ string) []byte {
				return buildEventPayload(t, "evt_malformed_session", stripeEventTypeCheckoutCompleted, map[string]any{
					"id": stripeTestSessionID, objectKey: stripeObjectCheckoutSession, metadataKey: 12345,
				})
			},
			want: http.StatusInternalServerError,
		},
		{
			name: "a quantity that is not a number",
			payload: func(t *testing.T, id string) []byte {
				return buildEventPayload(t, "evt_bad_quantity", stripeEventTypeCheckoutCompleted, map[string]any{
					"id":             stripeTestSessionID,
					objectKey:        stripeObjectCheckoutSession,
					paymentStatusKey: paidStatus,
					metadataKey:      map[string]any{practiceIDKey: id, quantityKey: "not-a-number"},
				})
			},
			want: http.StatusBadRequest,
		},
		// A Session the handler cannot turn into an exact unit price, or
		// cannot tie to a payment, is refused rather than stored lossily:
		// Stripe retries, and nothing has been written.
		{
			name: "a subtotal that does not divide by the quantity",
			payload: func(t *testing.T, id string) []byte {
				return pricedCheckoutPayload(t, "evt_uneven", id, 3, 10000, 0, "pi_uneven")
			},
			want: http.StatusBadRequest,
		},
		{
			name: "no subtotal at all",
			payload: func(t *testing.T, id string) []byte {
				return pricedCheckoutPayload(t, "evt_free", id, 5, 0, 0, "pi_free")
			},
			want: http.StatusBadRequest,
		},
		{
			name: "no payment to refund against",
			payload: func(t *testing.T, id string) []byte {
				return pricedCheckoutPayload(t, "evt_nopi", id, 5, 10000, 0, nil)
			},
			want: http.StatusBadRequest,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newWebhookEnv(t)
			secret := c.secret
			if secret == "" {
				secret = webhookTestSecret
			}

			e.deliver(c.payload(t, e.Practice), secret, c.want)

			if got := e.credited(); got != 0 {
				t.Fatalf("ledger sum = %d, want 0", got)
			}
		})
	}
}

// TestPostPurchaseWebhookHandler_OversizedBodyRejected proves a request
// body over webhookarrival.MaxBodyBytes is rejected rather than read without
// bound.
func TestPostPurchaseWebhookHandler_OversizedBodyRejected(t *testing.T) {
	e := newWebhookEnv(t)

	oversized := bytes.Repeat([]byte("a"), (1<<20)+1)
	reqtest.ExpectStatus(t, http.MethodPost, e.Srv.URL+"/stripe/webhook", http.StatusBadRequest,
		reqtest.Body("", oversized), reqtest.Header("Stripe-Signature", "t=1,v1=deadbeef"))
}

// TestPostPurchaseWebhookHandler_NonexistentPracticeRejected proves a
// well-formed but nonexistent practice_id -- which should never happen,
// since we set that metadata ourselves at Checkout Session creation --
// is refused with a 400 via credit_ledger's foreign key, and
// rolls back rather than leaving a stray stripe_webhook_events row with
// no corresponding credit.
func TestPostPurchaseWebhookHandler_NonexistentPracticeRejected(t *testing.T) {
	e := newWebhookEnv(t)

	const nonexistentPracticeID = "00000000-0000-0000-0000-000000000000"
	e.deliver(checkoutCompletedPayload(t, "evt_nonexistent_practice", nonexistentPracticeID, 3, paidStatus), webhookTestSecret, http.StatusBadRequest)

	count := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM stripe_webhook_events WHERE event_id = 'evt_nonexistent_practice'`)
	if count != 0 {
		t.Fatalf("stripe_webhook_events row count = %d, want 0 (rolled back)", count)
	}
}
