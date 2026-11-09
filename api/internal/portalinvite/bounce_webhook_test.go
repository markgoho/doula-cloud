package portalinvite_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/webhookarrival"
)

const (
	bounceWebhookTestKey = "mailgun-signing-key-test"

	// signatureField is Mailgun's name for both the signature envelope and
	// the HMAC inside it, in every payload this package's tests build.
	signatureField = "signature"

	statusBounced    = "bounced"
	statusComplained = "complained"
	statusDelivered  = "delivered"
	eventFailed      = "failed"
	severityPerm     = "permanent"
	someoneElsewhere = "someone@example.test"
)

// bounceEnv is what a Mailgun webhook test starts from: a database and a
// server carrying the webhook route. Nobody is signed in; the signature
// is the whole credential.
type bounceEnv struct {
	T   *testing.T
	DB  *testdb.DB
	Srv *httptest.Server
}

func newBounceEnv(t *testing.T, signingKey string) *bounceEnv {
	t.Helper()
	db := testdb.New(t)
	mux := http.NewServeMux()
	mux.Handle("POST /mailgun/webhook", webhookarrival.Record(db.App, webhookarrival.Mailgun, portalinvite.PostBounceWebhookHandler(db.App, signingKey)))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &bounceEnv{T: t, DB: db, Srv: srv}
}

// deliver posts payload as Mailgun would and requires the status want.
func (e *bounceEnv) deliver(payload []byte, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodPost, e.Srv.URL+"/mailgun/webhook", want, reqtest.Body("", payload))
}

// signedMailgunPayload builds a Mailgun webhook body around event/severity/
// recipient/eventID, signed with signingKey the same way Mailgun signs a
// real delivery -- hex HMAC-SHA256 of timestamp+token
// (documentation.mailgun.com/docs/mailgun/user-manual/webhooks/securing-webhooks).
// A wrong signingKey produces a body whose signature won't verify, for the
// invalid-signature test.
func signedMailgunPayload(t *testing.T, signingKey, event, severity, recipient, eventID string) []byte {
	t.Helper()
	return signedMailgunPayloadWithReason(t, signingKey, event, severity, recipient, eventID, "")
}

// signedMailgunPayloadWithReason is signedMailgunPayload plus the
// event-data.reason field ADR-0029 reads to tell a first-time SMTP
// rejection from a send Mailgun refused because the address was already
// on its own suppression list.
func signedMailgunPayloadWithReason(t *testing.T, signingKey, event, severity, recipient, eventID, reason string) []byte {
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
			"reason":    reason,
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

// seedSentOutboxRow seeds a Client (via seedPendingPortalInvite) and marks
// its portal_invite_outbox row 'sent', the state a bounce/complaint
// webhook always arrives after. Returns the row id and the Client's email
// -- the recipient address the webhook payload must carry to match it.
func seedSentOutboxRow(t *testing.T, db *testdb.DB) (outboxID, email string) {
	t.Helper()
	clientID, _ := seedPendingPortalInvite(t, db)
	portalUserID := portalUserIDForClient(t, db, clientID)
	outboxID = seedPortalInviteOutboxRow(t, db, portalUserID, 0, time.Now())
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

// TestPostBounceWebhookHandler_Outcomes is what each kind of Mailgun event
// does to the sent row and to the address's suppression. A complaint or a
// permanent failure marks the row and suppresses the address; everything
// else Mailgun reports leaves both alone -- an opened or temporarily
// deferred message must not silently stop a Practice's mail.
func TestPostBounceWebhookHandler_Outcomes(t *testing.T) {
	cases := []struct {
		name                         string
		event, severity, reason      string
		alreadyComplained            bool
		wantStatus                   string
		wantSuppressed               bool
		wantCause, wantSuppressEvent string
	}{
		{
			name: "a hard bounce", event: eventFailed, severity: severityPerm, reason: "bounce",
			wantStatus: statusBounced, wantSuppressed: true, wantCause: mailsuppress.CauseBounce,
		},
		{
			name: "a complaint", event: statusComplained,
			wantStatus: statusComplained, wantSuppressed: true, wantCause: mailsuppress.CauseComplaint, wantSuppressEvent: "evt-1",
		},
		{name: "a temporary failure", event: eventFailed, severity: "temporary", wantStatus: testOutboxStatusSent},
		{name: "a delivery", event: statusDelivered, wantStatus: testOutboxStatusSent},
		{
			// A "suppress-*" reason is Mailgun declining a send it never
			// attempted. Writing a fresh suppression there would downgrade
			// a permanent, never-clearable complaint into a clearable
			// bounce on the next retry.
			name: "a refusal because the address is already suppressed", event: eventFailed, severity: severityPerm, reason: "suppress-complaint",
			alreadyComplained: true, wantSuppressed: true, wantCause: mailsuppress.CauseComplaint, wantSuppressEvent: "evt-complaint",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newBounceEnv(t, bounceWebhookTestKey)
			outboxID, email := seedSentOutboxRow(t, e.DB)
			if c.alreadyComplained {
				if err := mailsuppress.Record(t.Context(), e.DB.App, email, mailsuppress.CauseComplaint, "evt-complaint"); err != nil {
					t.Fatalf("seed suppression: %v", err)
				}
			}

			e.deliver(signedMailgunPayloadWithReason(t, bounceWebhookTestKey, c.event, c.severity, email, "evt-1", c.reason), http.StatusOK)

			if c.wantStatus != "" {
				if status := outboxRowState(t, e.DB, outboxID); status != c.wantStatus {
					t.Errorf("outbox status = %q, want %q", status, c.wantStatus)
				}
			}
			found, cause, eventID := suppressionState(t, e.DB, email)
			if found != c.wantSuppressed {
				t.Fatalf("suppression found = %v, want %v", found, c.wantSuppressed)
			}
			if cause != c.wantCause {
				t.Errorf("cause = %q, want %q", cause, c.wantCause)
			}
			if c.wantSuppressEvent != "" && eventID != c.wantSuppressEvent {
				t.Errorf("mailgun_event_id = %q, want %q", eventID, c.wantSuppressEvent)
			}
		})
	}
}

func TestPostBounceWebhookHandler_UnknownRecipientIsNoOp(t *testing.T) {
	e := newBounceEnv(t, bounceWebhookTestKey)

	e.deliver(signedMailgunPayload(t, bounceWebhookTestKey, eventFailed, severityPerm, "nobody@example.test", "event-unknown-1"), http.StatusOK)
}

func TestPostBounceWebhookHandler_ReplayedEventIsNotDoubleProcessed(t *testing.T) {
	e := newBounceEnv(t, bounceWebhookTestKey)
	outboxID, email := seedSentOutboxRow(t, e.DB)
	payload := signedMailgunPayload(t, bounceWebhookTestKey, eventFailed, severityPerm, email, "event-replay-1")

	e.deliver(payload, http.StatusOK)
	// Manually revert to 'sent' -- if the replay were reprocessed, this
	// would flip back to 'bounced' and the check below would pass for the
	// wrong reason.
	if _, err := e.DB.Admin.ExecContext(t.Context(), `UPDATE portal_invite_outbox SET status = 'sent' WHERE id = $1`, outboxID); err != nil {
		t.Fatalf("revert outbox row: %v", err)
	}
	e.deliver(payload, http.StatusOK)

	if status := outboxRowState(t, e.DB, outboxID); status != testOutboxStatusSent {
		t.Fatalf("status = %q, want %s (replay must not reprocess)", status, testOutboxStatusSent)
	}
}

func TestPostBounceWebhookHandler_Refusals(t *testing.T) {
	cases := []struct {
		name string
		// configuredKey is the key the server verifies with.
		configuredKey string
		payload       func(t *testing.T) []byte
	}{
		{
			name:          "a signature made with the wrong key",
			configuredKey: bounceWebhookTestKey,
			payload: func(t *testing.T) []byte {
				return signedMailgunPayload(t, "wrong-key", eventFailed, severityPerm, someoneElsewhere, "event-bad-sig")
			},
		},
		{
			name:          "no key configured",
			configuredKey: "",
			payload: func(t *testing.T) []byte {
				return signedMailgunPayload(t, "", eventFailed, severityPerm, someoneElsewhere, "event-empty-key")
			},
		},
		{
			name:          "no signature fields",
			configuredKey: bounceWebhookTestKey,
			payload: func(*testing.T) []byte {
				return []byte(`{"event-data": {"id": "event-no-sig", "event": "failed", "severity": "permanent", "recipient": "someone@example.test"}}`)
			},
		},
		{name: "a body that is not JSON", configuredKey: bounceWebhookTestKey, payload: func(*testing.T) []byte { return []byte("not json") }},
		{
			name:          "a body over the limit",
			configuredKey: bounceWebhookTestKey,
			payload:       func(*testing.T) []byte { return []byte(strings.Repeat("a", (1<<20)+1)) },
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newBounceEnv(t, c.configuredKey)

			e.deliver(c.payload(t), http.StatusBadRequest)
		})
	}
}
