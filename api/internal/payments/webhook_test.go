package payments_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

const (
	stripeObjectEvent             = "event"
	stripeObjectAccount           = "account"
	stripeEventTypeAccountUpdated = "account.updated"
	stripeEventTypeOther          = "invoice.paid"
	stripeConnectWebhookSecret    = webhookTestSecret
	objectKey                     = "object"
	typeKey                       = "type"
	dataKey                       = "data"
	accountKey                    = "account"
)

func newConnectWebhookServer(db *testdb.DB) *httptest.Server {
	client := payments.NewStripeAPIClient("sk_test_unused", "https://app.test")
	mux := http.NewServeMux()
	mux.Handle("POST /stripe/connect-webhook", payments.PostConnectWebhookHandler(db.App, client, stripeConnectWebhookSecret))
	return httptest.NewServer(mux)
}

// buildConnectEventPayload assembles a raw Stripe event envelope around
// dataObject, tagged with accountID as the event's top-level account
// field -- the same field every Connect event carries.
func buildConnectEventPayload(t *testing.T, eventID, eventType, accountID string, dataObject map[string]any) []byte {
	t.Helper()
	body := map[string]any{
		"id":       eventID,
		objectKey:  stripeObjectEvent,
		typeKey:    eventType,
		accountKey: accountID,
		dataKey:    map[string]any{objectKey: dataObject},
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

// accountUpdatedPayload builds a raw account.updated event body with all
// three capability booleans set to true on its data.object -- the only
// combination this file's tests need; partial-capability states are
// already covered by connect_test.go's GetConnectStatusHandler tests.
func accountUpdatedPayload(t *testing.T, eventID, accountID string) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeAccountUpdated, accountID, map[string]any{
		"id":                accountID,
		objectKey:           stripeObjectAccount,
		"charges_enabled":   true,
		"payouts_enabled":   true,
		"details_submitted": true,
	})
}

func otherConnectEventPayload(t *testing.T, eventID, accountID string) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeOther, accountID, map[string]any{"id": "in_test", objectKey: "invoice"})
}

func postConnectWebhook(t *testing.T, srv *httptest.Server, payload []byte, signingSecret string) *http.Response {
	t.Helper()
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: signingSecret})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/stripe/connect-webhook", bytes.NewReader(payload))
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

func capabilities(t *testing.T, db *testdb.DB, practiceID string) (charges, payouts, details bool) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT stripe_connect_charges_enabled, stripe_connect_payouts_enabled, stripe_connect_details_submitted
		FROM practices WHERE id = $1`, practiceID,
	).Scan(&charges, &payouts, &details); err != nil {
		t.Fatalf("query capabilities: %v", err)
	}
	return
}

func seedConnectedPractice(t *testing.T, db *testdb.DB, name, accountID string) string {
	t.Helper()
	practiceID := seedPractice(t, db, name)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = $1 WHERE id = $2`, accountID, practiceID,
	); err != nil {
		t.Fatalf("seed connect account id: %v", err)
	}
	return practiceID
}

// TestPostConnectWebhookHandler_UpdatesCapabilitiesForRecognizedAccount
// proves a validly-signed account.updated event for a known
// stripe_connect_account_id updates all three capability booleans.
func TestPostConnectWebhookHandler_UpdatesCapabilitiesForRecognizedAccount(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Webhook Practice", "acct_recognized")
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := accountUpdatedPayload(t, "evt_update_once", "acct_recognized")
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	charges, payouts, details := capabilities(t, db, practiceID)
	if !charges || !payouts || !details {
		t.Fatalf("capabilities = (%v, %v, %v), want all true", charges, payouts, details)
	}
}

// TestPostConnectWebhookHandler_ReplayedEventIsNoOp is the explicit
// idempotency test AC calls for: replaying the same Stripe event id must
// not re-apply the status transition (an earlier "false" state should
// stay false, not flip to whatever a stale duplicate delivery carries).
func TestPostConnectWebhookHandler_ReplayedEventIsNoOp(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Replay Practice", "acct_replay")
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := accountUpdatedPayload(t, "evt_replayed", "acct_replay")

	first := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first delivery status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	// Directly revert the capabilities to prove a second delivery of the
	// same event id is a genuine no-op, not just idempotent because the
	// values already matched.
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_charges_enabled = false WHERE id = $1`, practiceID,
	); err != nil {
		t.Fatalf("revert capability: %v", err)
	}

	second := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	_ = second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("replayed delivery status = %d, want %d", second.StatusCode, http.StatusOK)
	}

	charges, _, _ := capabilities(t, db, practiceID)
	if charges {
		t.Fatal("charges_enabled = true after replay, want false (replay must be a no-op)")
	}
}

// TestPostConnectWebhookHandler_InvalidSignatureRejected proves a payload
// signed with the wrong secret is rejected without touching the DB.
func TestPostConnectWebhookHandler_InvalidSignatureRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Bad Signature Practice", "acct_bad_sig")
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := accountUpdatedPayload(t, "evt_bad_sig", "acct_bad_sig")
	resp := postConnectWebhook(t, srv, payload, "whsec_wrong_secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	charges, payouts, details := capabilities(t, db, practiceID)
	if charges || payouts || details {
		t.Fatalf("capabilities = (%v, %v, %v), want all false (DB untouched)", charges, payouts, details)
	}
}

// TestPostConnectWebhookHandler_UnrecognizedAccountDroppedButAcknowledged
// proves an account.updated event for an account id no Practice has
// stored is logged and dropped, not treated as an error -- the webhook
// still returns 200 so Stripe doesn't retry indefinitely.
func TestPostConnectWebhookHandler_UnrecognizedAccountDroppedButAcknowledged(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := accountUpdatedPayload(t, "evt_unrecognized", "acct_unrecognized")
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestPostConnectWebhookHandler_OtherEventTypesAcknowledgedNotProcessed
// proves an event type other than account.updated is acknowledged with
// 200 but never updates any Practice's capabilities.
func TestPostConnectWebhookHandler_OtherEventTypesAcknowledgedNotProcessed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Other Event Practice", "acct_other_event")
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := otherConnectEventPayload(t, "evt_other_type", "acct_other_event")
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	charges, payouts, details := capabilities(t, db, practiceID)
	if charges || payouts || details {
		t.Fatalf("capabilities = (%v, %v, %v), want all false (untouched)", charges, payouts, details)
	}
}

// TestPostConnectWebhookHandler_OversizedBodyRejected proves a request
// body over maxWebhookBodyBytes is rejected rather than read without
// bound.
func TestPostConnectWebhookHandler_OversizedBodyRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	oversized := bytes.Repeat([]byte("a"), (1<<20)+1)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/stripe/connect-webhook", bytes.NewReader(oversized))
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

// TestPostConnectWebhookHandler_MalformedAccountObjectRejected proves an
// account.updated event whose data.object can't be unmarshaled into the
// expected shape is rejected rather than silently applying zero values.
func TestPostConnectWebhookHandler_MalformedAccountObjectRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	// charges_enabled as a string (instead of a bool) fails to unmarshal
	// into accountUpdatedObject.
	payload := buildConnectEventPayload(t, "evt_malformed", stripeEventTypeAccountUpdated, "acct_malformed",
		map[string]any{"charges_enabled": "not-a-bool"})

	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
