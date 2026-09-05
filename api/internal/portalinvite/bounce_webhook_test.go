package portalinvite_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/testdb"
)

const bounceWebhookTestKey = "mailgun-signing-key-test"

func newBounceWebhookServer(db *testdb.DB, signingKey string) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /mailgun/webhook", portalinvite.PostBounceWebhookHandler(db.App, signingKey))
	return httptest.NewServer(mux)
}

// signedMailgunPayload builds a Mailgun webhook body around event/severity/
// recipient/eventID, signed with signingKey the same way Mailgun signs a
// real delivery -- hex HMAC-SHA256 of timestamp+token
// (documentation.mailgun.com/docs/mailgun/user-manual/webhooks/securing-webhooks).
// A wrong signingKey produces a body whose signature won't verify, for the
// invalid-signature test.
func signedMailgunPayload(t *testing.T, signingKey, event, severity, recipient, eventID string) []byte {
	t.Helper()
	timestamp := "1700000000"
	token := "test-token-0123456789"
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(timestamp + token))
	sig := hex.EncodeToString(mac.Sum(nil))

	body := map[string]any{
		signatureField: map[string]any{
			"timestamp":    timestamp,
			"token":        token,
			signatureField: sig,
		},
		"event-data": map[string]any{
			"id":        eventID,
			"event":     event,
			"recipient": recipient,
			"severity":  severity,
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

func postBounceWebhook(t *testing.T, srv *httptest.Server, payload []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/mailgun/webhook", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// seedSentOutboxRow seeds a Client (via seedPendingPortalInvite) and marks
// its portal_invite_outbox row 'sent', the state a bounce/complaint
// webhook always arrives after. Returns the row id and the Client's email
// -- the recipient address the webhook payload must carry to match it.
func seedSentOutboxRow(t *testing.T, db *testdb.DB) (outboxID, email string) {
	t.Helper()
	clientID, _ := seedPendingPortalInvite(t, db)
	portalUserID := portalUserIDForClient(t, db, clientID)
	outboxID = seedOutboxRow(t, db, portalUserID, 0, time.Now())
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE portal_invite_outbox SET status = 'sent', sent_at = now() WHERE id = $1`, outboxID,
	); err != nil {
		t.Fatalf("mark outbox row sent: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT email FROM clients WHERE id = $1`, clientID).Scan(&email); err != nil {
		t.Fatalf("look up client email: %v", err)
	}
	return outboxID, email
}

func TestPostBounceWebhookHandler_HardBounceMarksRowBounced(t *testing.T) {
	db := testdb.New(t)
	outboxID, email := seedSentOutboxRow(t, db)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedMailgunPayload(t, bounceWebhookTestKey, "failed", "permanent", email, "event-bounce-1")
	resp := postBounceWebhook(t, srv, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _ := outboxRowState(t, db, outboxID)
	if status != "bounced" {
		t.Fatalf("status = %q, want bounced", status)
	}
}

func TestPostBounceWebhookHandler_ComplaintMarksRowComplained(t *testing.T) {
	db := testdb.New(t)
	outboxID, email := seedSentOutboxRow(t, db)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedMailgunPayload(t, bounceWebhookTestKey, "complained", "", email, "event-complaint-1")
	resp := postBounceWebhook(t, srv, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _ := outboxRowState(t, db, outboxID)
	if status != "complained" {
		t.Fatalf("status = %q, want complained", status)
	}
}

func TestPostBounceWebhookHandler_TemporaryFailureLeavesRowUntouched(t *testing.T) {
	db := testdb.New(t)
	outboxID, email := seedSentOutboxRow(t, db)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedMailgunPayload(t, bounceWebhookTestKey, "failed", "temporary", email, "event-temp-1")
	resp := postBounceWebhook(t, srv, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testOutboxStatusSent)
	}
}

func TestPostBounceWebhookHandler_UnrelatedEventTypeLeavesRowUntouched(t *testing.T) {
	db := testdb.New(t)
	outboxID, email := seedSentOutboxRow(t, db)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedMailgunPayload(t, bounceWebhookTestKey, "delivered", "", email, "event-delivered-1")
	resp := postBounceWebhook(t, srv, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s", status, testOutboxStatusSent)
	}
}

func TestPostBounceWebhookHandler_UnknownRecipientIsNoOp(t *testing.T) {
	db := testdb.New(t)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedMailgunPayload(t, bounceWebhookTestKey, "failed", "permanent", "nobody@example.test", "event-unknown-1")
	resp := postBounceWebhook(t, srv, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestPostBounceWebhookHandler_ReplayedEventIsNotDoubleProcessed(t *testing.T) {
	db := testdb.New(t)
	outboxID, email := seedSentOutboxRow(t, db)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedMailgunPayload(t, bounceWebhookTestKey, "failed", "permanent", email, "event-replay-1")
	resp1 := postBounceWebhook(t, srv, payload)
	_ = resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first delivery status = %d, want %d", resp1.StatusCode, http.StatusOK)
	}

	// Manually revert to 'sent' -- if the replay were reprocessed, this
	// would flip back to 'bounced' and the test below would pass for the
	// wrong reason.
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE portal_invite_outbox SET status = 'sent' WHERE id = $1`, outboxID); err != nil {
		t.Fatalf("revert outbox row: %v", err)
	}

	resp2 := postBounceWebhook(t, srv, payload)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("replayed delivery status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}
	status, _ := outboxRowState(t, db, outboxID)
	if status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s (replay must not reprocess)", status, testOutboxStatusSent)
	}
}

func TestPostBounceWebhookHandler_InvalidSignatureRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedMailgunPayload(t, "wrong-key", "failed", "permanent", "someone@example.test", "event-bad-sig")
	resp := postBounceWebhook(t, srv, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPostBounceWebhookHandler_EmptyConfiguredKeyAlwaysRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newBounceWebhookServer(db, "")
	defer srv.Close()

	payload := signedMailgunPayload(t, "", "failed", "permanent", "someone@example.test", "event-empty-key")
	resp := postBounceWebhook(t, srv, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPostBounceWebhookHandler_MissingSignatureFieldsRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := []byte(`{"event-data": {"id": "event-no-sig", "event": "failed", "severity": "permanent", "recipient": "someone@example.test"}}`)
	resp := postBounceWebhook(t, srv, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPostBounceWebhookHandler_MalformedBodyRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	resp := postBounceWebhook(t, srv, []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPostBounceWebhookHandler_OversizedBodyRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	oversized := []byte(strings.Repeat("a", (1<<20)+1))
	resp := postBounceWebhook(t, srv, oversized)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
