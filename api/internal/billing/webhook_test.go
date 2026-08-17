package billing_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/testdb"
)

//nolint:gosec // test fixture value, not a real credential
const webhookTestSecret = "whsec_test_secret"

const (
	stripeObjectEvent                = "event"
	stripeObjectCheckoutSession      = "checkout.session"
	stripeEventTypeCheckoutCompleted = "checkout.session.completed"
	stripeTestSessionID              = "cs_test_session"
	metadataKey                      = "metadata"
	objectKey                        = "object"
)

func newWebhookServer(db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /stripe/webhook", billing.PostPurchaseWebhookHandler(db.App, webhookTestSecret))
	return httptest.NewServer(mux)
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
		"payment_status": paymentStatus,
		metadataKey: map[string]any{
			"practice_id": practiceID,
			"quantity":    strconv.Itoa(quantity),
		},
	})
}

func otherEventPayload(t *testing.T, eventID string) []byte {
	t.Helper()
	return buildEventPayload(t, eventID, "payment_intent.succeeded", map[string]any{"id": "pi_test", objectKey: "payment_intent"})
}

func postWebhook(t *testing.T, srv *httptest.Server, payload []byte, signingSecret string) *http.Response {
	t.Helper()
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: signingSecret})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/stripe/webhook", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Stripe-Signature", signed.Header)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func sumLedger(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var sum int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT COALESCE(SUM(quantity), 0) FROM credit_ledger WHERE practice_id = $1`, practiceID,
	).Scan(&sum); err != nil {
		t.Fatalf("sum credit_ledger: %v", err)
	}
	return sum
}

// TestPostPurchaseWebhookHandler_CreditsLedgerOnConfirmedCheckout proves a
// validly-signed, paid checkout.session.completed event appends a +N
// purchase row for the Practice named in its metadata.
func TestPostPurchaseWebhookHandler_CreditsLedgerOnConfirmedCheckout(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Webhook Practice")
	srv := newWebhookServer(db)
	defer srv.Close()

	payload := checkoutCompletedPayload(t, "evt_credit_once", practiceID, 7, "paid")
	resp := postWebhook(t, srv, payload, webhookTestSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := sumLedger(t, db, practiceID); got != 7 {
		t.Fatalf("ledger sum = %d, want 7", got)
	}

	var origin string
	var quantity int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT origin, quantity FROM credit_ledger WHERE practice_id = $1`, practiceID,
	).Scan(&origin, &quantity); err != nil {
		t.Fatalf("query ledger row: %v", err)
	}
	if origin != "purchase" || quantity != 7 {
		t.Fatalf("ledger row = (%q, %d), want (\"purchase\", 7)", origin, quantity)
	}
}

// TestPostPurchaseWebhookHandler_ReplayedEventCreditsExactlyOnce is the
// explicit idempotency test AC #3 calls for: replaying the same Stripe
// event id must not double-credit.
func TestPostPurchaseWebhookHandler_ReplayedEventCreditsExactlyOnce(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Replay Practice")
	srv := newWebhookServer(db)
	defer srv.Close()

	payload := checkoutCompletedPayload(t, "evt_replayed", practiceID, 4, "paid")

	first := postWebhook(t, srv, payload, webhookTestSecret)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first delivery status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := postWebhook(t, srv, payload, webhookTestSecret)
	_ = second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("replayed delivery status = %d, want %d", second.StatusCode, http.StatusOK)
	}

	if got := sumLedger(t, db, practiceID); got != 4 {
		t.Fatalf("ledger sum after replay = %d, want 4 (credited exactly once)", got)
	}
}

// TestPostPurchaseWebhookHandler_InvalidSignatureRejected proves a
// payload signed with the wrong secret is rejected and never credits.
func TestPostPurchaseWebhookHandler_InvalidSignatureRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Bad Signature Practice")
	srv := newWebhookServer(db)
	defer srv.Close()

	payload := checkoutCompletedPayload(t, "evt_bad_sig", practiceID, 9, "paid")
	resp := postWebhook(t, srv, payload, "whsec_wrong_secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if got := sumLedger(t, db, practiceID); got != 0 {
		t.Fatalf("ledger sum = %d, want 0", got)
	}
}

// TestPostPurchaseWebhookHandler_UnpaidPaymentNeverCredits proves a
// completed Checkout Session that isn't actually paid (an async payment
// method still pending) never credits the ledger.
func TestPostPurchaseWebhookHandler_UnpaidPaymentNeverCredits(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Unpaid Practice")
	srv := newWebhookServer(db)
	defer srv.Close()

	payload := checkoutCompletedPayload(t, "evt_unpaid", practiceID, 6, "unpaid")
	resp := postWebhook(t, srv, payload, webhookTestSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := sumLedger(t, db, practiceID); got != 0 {
		t.Fatalf("ledger sum = %d, want 0", got)
	}
}

// TestPostPurchaseWebhookHandler_OtherEventTypesAcknowledgedNotCredited
// proves an event type other than checkout.session.completed (e.g.
// payment_intent.succeeded, which fires alongside it for the same
// Checkout) is acknowledged with 200 but never credits -- avoiding a
// double-credit off the same purchase under two different event ids.
func TestPostPurchaseWebhookHandler_OtherEventTypesAcknowledgedNotCredited(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Other Event Practice")
	srv := newWebhookServer(db)
	defer srv.Close()

	payload := otherEventPayload(t, "evt_payment_intent")
	resp := postWebhook(t, srv, payload, webhookTestSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := sumLedger(t, db, practiceID); got != 0 {
		t.Fatalf("ledger sum = %d, want 0", got)
	}
}

// TestPostPurchaseWebhookHandler_InvalidPracticeIDMetadataRejected proves
// a malformed practice_id in metadata is rejected rather than crashing or
// silently doing nothing.
func TestPostPurchaseWebhookHandler_InvalidPracticeIDMetadataRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newWebhookServer(db)
	defer srv.Close()

	payload := checkoutCompletedPayload(t, "evt_bad_practice", "not-a-uuid", 3, "paid")
	resp := postWebhook(t, srv, payload, webhookTestSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostPurchaseWebhookHandler_OversizedBodyRejected proves a request
// body over maxWebhookBodyBytes is rejected rather than read without
// bound.
func TestPostPurchaseWebhookHandler_OversizedBodyRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newWebhookServer(db)
	defer srv.Close()

	oversized := bytes.Repeat([]byte("a"), (1<<20)+1)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/stripe/webhook", bytes.NewReader(oversized))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Stripe-Signature", "t=1,v1=deadbeef")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostPurchaseWebhookHandler_MalformedSessionObjectRejected proves a
// checkout.session.completed event whose data.object can't be unmarshaled
// into a CheckoutSession is rejected rather than crediting blindly.
func TestPostPurchaseWebhookHandler_MalformedSessionObjectRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newWebhookServer(db)
	defer srv.Close()

	// metadata as a number (instead of an object of strings) fails to
	// unmarshal into stripe.CheckoutSession.Metadata.
	payload := buildEventPayload(t, "evt_malformed_session", stripeEventTypeCheckoutCompleted, map[string]any{
		"id":        stripeTestSessionID,
		objectKey:   stripeObjectCheckoutSession,
		metadataKey: 12345,
	})

	resp := postWebhook(t, srv, payload, webhookTestSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// TestPostPurchaseWebhookHandler_NonexistentPracticeRejected proves a
// well-formed but nonexistent practice_id -- which should never happen,
// since we set that metadata ourselves at Checkout Session creation --
// surfaces as an internal error via credit_ledger's foreign key, and
// rolls back rather than leaving a stray stripe_webhook_events row with
// no corresponding credit.
func TestPostPurchaseWebhookHandler_NonexistentPracticeRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newWebhookServer(db)
	defer srv.Close()

	const nonexistentPracticeID = "00000000-0000-0000-0000-000000000000"
	payload := checkoutCompletedPayload(t, "evt_nonexistent_practice", nonexistentPracticeID, 3, "paid")
	resp := postWebhook(t, srv, payload, webhookTestSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM stripe_webhook_events WHERE event_id = 'evt_nonexistent_practice'`,
	).Scan(&count); err != nil {
		t.Fatalf("count stripe_webhook_events: %v", err)
	}
	if count != 0 {
		t.Fatalf("stripe_webhook_events row count = %d, want 0 (rolled back)", count)
	}
}

// TestPostPurchaseWebhookHandler_InvalidQuantityMetadataRejected proves a
// non-numeric quantity in metadata is rejected.
func TestPostPurchaseWebhookHandler_InvalidQuantityMetadataRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Bad Quantity Practice")
	srv := newWebhookServer(db)
	defer srv.Close()

	payload := buildEventPayload(t, "evt_bad_quantity", stripeEventTypeCheckoutCompleted, map[string]any{
		"id":             stripeTestSessionID,
		objectKey:        stripeObjectCheckoutSession,
		"payment_status": "paid",
		metadataKey: map[string]any{
			"practice_id": practiceID,
			"quantity":    "not-a-number",
		},
	})

	resp := postWebhook(t, srv, payload, webhookTestSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if got := sumLedger(t, db, practiceID); got != 0 {
		t.Fatalf("ledger sum = %d, want 0", got)
	}
}
