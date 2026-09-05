package portalinvite_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/testdb"
)

// signatureField is Mailgun's name for both the signature envelope and
// the HMAC inside it, in every payload this package's tests build.
const signatureField = "signature"

// signedPermanentFailurePayload is signedMailgunPayload's
// permanent-failure case plus the event-data.reason field ADR-0029 reads
// to tell a first-time SMTP rejection from a send Mailgun refused
// because the address was already on its own suppression list. Only a
// permanent failure carries a reason worth reading, so the event and
// severity are fixed rather than passed.
func signedPermanentFailurePayload(t *testing.T, recipient, eventID, reason string) []byte {
	t.Helper()
	timestamp := "1700000000"
	token := "test-token-0123456789"
	mac := hmac.New(sha256.New, []byte(bounceWebhookTestKey))
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
			"event":     "failed",
			"recipient": recipient,
			"severity":  "permanent",
			"reason":    reason,
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

// suppressionState reads the row, if any, the webhook wrote for address.
func suppressionState(t *testing.T, db *testdb.DB, address string) (found bool, cause, eventID string) {
	t.Helper()
	err := db.Admin.QueryRowContext(t.Context(),
		`SELECT cause, coalesce(mailgun_event_id, '') FROM email_suppressions WHERE address = $1 AND cleared_at IS NULL`,
		mailsuppress.Normalize(address),
	).Scan(&cause, &eventID)
	if err != nil {
		return false, "", ""
	}
	return true, cause, eventID
}

func TestPostBounceWebhookHandler_ComplaintSuppressesTheAddress(t *testing.T) {
	db := testdb.New(t)
	_, email := seedSentOutboxRow(t, db)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	resp := postBounceWebhook(t, srv, signedMailgunPayload(t, bounceWebhookTestKey, "complained", "", email, "evt-complaint"))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	found, cause, eventID := suppressionState(t, db, email)
	if !found {
		t.Fatal("a complaint wrote no suppression row")
	}
	if cause != mailsuppress.CauseComplaint {
		t.Fatalf("cause = %q, want %q", cause, mailsuppress.CauseComplaint)
	}
	if eventID != "evt-complaint" {
		t.Fatalf("mailgun_event_id = %q, want evt-complaint", eventID)
	}
}

func TestPostBounceWebhookHandler_HardBounceSuppressesTheAddress(t *testing.T) {
	db := testdb.New(t)
	_, email := seedSentOutboxRow(t, db)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedPermanentFailurePayload(t, email, "evt-bounce", "bounce")
	resp := postBounceWebhook(t, srv, payload)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	found, cause, _ := suppressionState(t, db, email)
	if !found {
		t.Fatal("a hard bounce wrote no suppression row")
	}
	if cause != mailsuppress.CauseBounce {
		t.Fatalf("cause = %q, want %q", cause, mailsuppress.CauseBounce)
	}
}

// A "suppress-*" reason is Mailgun declining a send it never attempted.
// Writing a fresh suppression there would downgrade a permanent,
// never-clearable complaint into a clearable bounce on the next retry.
func TestPostBounceWebhookHandler_AlreadySuppressedReasonKeepsTheOriginalCause(t *testing.T) {
	db := testdb.New(t)
	_, email := seedSentOutboxRow(t, db)
	if err := mailsuppress.Record(t.Context(), db.App, email, mailsuppress.CauseComplaint, "evt-complaint"); err != nil {
		t.Fatalf("seed suppression: %v", err)
	}
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedPermanentFailurePayload(t, email, "evt-suppressed", "suppress-complaint")
	resp := postBounceWebhook(t, srv, payload)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	_, cause, eventID := suppressionState(t, db, email)
	if cause != mailsuppress.CauseComplaint {
		t.Fatalf("cause = %q, want it left at %q", cause, mailsuppress.CauseComplaint)
	}
	if eventID != "evt-complaint" {
		t.Fatalf("mailgun_event_id = %q, want the original evt-complaint", eventID)
	}
}

// Everything Mailgun reports that is neither a complaint nor a permanent
// failure leaves the address sendable -- an opened or temporarily
// deferred message must not silently stop a Practice's mail.
func TestPostBounceWebhookHandler_TemporaryFailureSuppressesNothing(t *testing.T) {
	db := testdb.New(t)
	_, email := seedSentOutboxRow(t, db)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	resp := postBounceWebhook(t, srv, signedMailgunPayload(t, bounceWebhookTestKey, "failed", "temporary", email, "evt-temp"))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	if found, _, _ := suppressionState(t, db, email); found {
		t.Fatal("a temporary failure suppressed the address")
	}
}

// The "suppress-*" event may be the first this endpoint ever sees for an
// address -- Mailgun held it before the webhook was provisioned, or a
// delivery was missed. Without a local row every outbox would retry an
// address Mailgun refuses forever.
func TestPostBounceWebhookHandler_AlreadySuppressedReasonRecordsWhenNothingIsOnFile(t *testing.T) {
	db := testdb.New(t)
	_, email := seedSentOutboxRow(t, db)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedPermanentFailurePayload(t, email, "evt-suppressed", "suppress-bounce")
	resp := postBounceWebhook(t, srv, payload)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	found, cause, _ := suppressionState(t, db, email)
	if !found {
		t.Fatal("a suppress-bounce with nothing on file wrote no suppression row")
	}
	if cause != mailsuppress.CauseBounce {
		t.Fatalf("cause = %q, want %q (taken from the reason)", cause, mailsuppress.CauseBounce)
	}
}

// Nothing Doula Cloud sends is unsubscribable mail (#731: every kind is
// transactional), so an unsubscribe on the shared domain is not this
// product's fact to record.
func TestPostBounceWebhookHandler_SuppressUnsubscribeRecordsNothing(t *testing.T) {
	db := testdb.New(t)
	_, email := seedSentOutboxRow(t, db)
	srv := newBounceWebhookServer(db, bounceWebhookTestKey)
	defer srv.Close()

	payload := signedPermanentFailurePayload(t, email, "evt-unsub", "suppress-unsubscribe")
	resp := postBounceWebhook(t, srv, payload)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	if found, _, _ := suppressionState(t, db, email); found {
		t.Fatal("suppress-unsubscribe wrote a suppression row")
	}
}
