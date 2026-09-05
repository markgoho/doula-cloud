package portalinvite

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/mailsuppress"
)

// maxMailgunWebhookBodyBytes bounds how much of a webhook request body
// PostBounceWebhookHandler will read, mirroring billing's Stripe webhook
// bound on this same unauthenticated-endpoint concern.
const maxMailgunWebhookBodyBytes = 1 << 20 // 1 MiB

// mailgunWebhookPayload is the subset of Mailgun's webhook body
// (documentation.mailgun.com/docs/mailgun/user-manual/webhooks/webhook-payloads)
// this handler needs: the signature envelope, and the event fields that
// distinguish a hard bounce and a complaint from every other delivery
// event.
type mailgunWebhookPayload struct {
	Signature struct {
		Timestamp string `json:"timestamp"`
		Token     string `json:"token"`
		Signature string `json:"signature"`
	} `json:"signature"`
	EventData struct {
		ID        string `json:"id"`
		Event     string `json:"event"`
		Recipient string `json:"recipient"`
		Severity  string `json:"severity"`
		// Reason distinguishes a first-time SMTP rejection from a send
		// Mailgun never attempted because the address was already on its
		// own suppression list. The "suppress-bounce" /
		// "suppress-complaint" / "suppress-unsubscribe" values mean the
		// latter (ADR-0029, research on #731): recording a fresh
		// suppression for one of those would only restate what is
		// already true, and would overwrite the original cause -- a
		// complaint's permanent suppression could be downgraded to a
		// clearable bounce by a later retry against the same address.
		Reason string `json:"reason"`
	} `json:"event-data"`
}

// alreadySuppressed reports whether reason says Mailgun refused this
// send server-side because the address was already suppressed, rather
// than attempting it and being rejected.
func alreadySuppressed(reason string) bool {
	return strings.HasPrefix(reason, "suppress-")
}

// PostBounceWebhookHandler receives Mailgun's delivery-event webhook -- a
// platform-level endpoint built in the shape of
// billing.PostPurchaseWebhookHandler: signature-verified, no
// staffauth.Middleware in front of it (there is no Practice or Client
// session on this path), replays deduped against mailgun_webhook_events
// (00037). Record-only per #337/#340: a hard bounce
// ("failed"/"permanent") sets the matching portal_invite_outbox row to
// 'bounced', a spam complaint ("complained") sets it to 'complained'.
// Every other event type is acknowledged and ignored.
//
// No longer record-only as of #733/ADR-0029: either event also writes an
// email_suppressions row, which every one of the eleven mail kinds
// consults through mailsuppress.Sender before sending. The portal-invite
// row's own status stays exactly as #340 left it, for #346's Staff
// screen; the suppression is the part that reaches the other ten
// outboxes. It still triggers no remedy -- a Staff member re-invites by
// hand, and only after clearing a bounce-caused suppression.
//
// It lives in portalinvite because the portal-invite row update is the
// package-specific half and this is where the signature check and its
// tests already are; the endpoint itself is platform-level, mounted at
// one path Mailgun's console is configured against.
func PostBounceWebhookHandler(db *sql.DB, signingKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxMailgunWebhookBodyBytes))
		if err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var payload mailgunWebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if signingKey == "" || !validMailgunSignature(signingKey, payload.Signature.Timestamp, payload.Signature.Token, payload.Signature.Signature) {
			apierr.WriteError(w, "invalid signature", http.StatusBadRequest)
			return
		}

		var newStatus, suppressionCause string
		switch {
		case payload.EventData.Event == "failed" && payload.EventData.Severity == "permanent":
			newStatus = "bounced"
			// A "suppress-*" reason is Mailgun declining a send it never
			// made; the address is already suppressed, on Mailgun's list
			// and on ours, so the portal-invite row still records that
			// this send did not arrive but no new suppression is written.
			if !alreadySuppressed(payload.EventData.Reason) {
				suppressionCause = mailsuppress.CauseBounce
			}
		case payload.EventData.Event == "complained":
			newStatus = "complained"
			suppressionCause = mailsuppress.CauseComplaint
		default:
			// Every other event type -- delivered, opened, clicked,
			// unsubscribed, or a temporary failure Mailgun itself will
			// retry -- is acknowledged and ignored. This endpoint only
			// records the two terminal states #337 decided on.
			w.WriteHeader(http.StatusOK)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed := false
		defer func() {
			if !committed {
				// coverage:ignore reason: only reached by a DB failure
				// after BeginTx succeeds -- every branch past this point
				// that returns without committing is itself a DB-failure
				// coverage:ignore, so there is no non-DB way to drive
				// this closed (mirrors outbox_handler.go's identical
				// defer).
				_ = tx.Rollback()
			}
		}()

		result, err := tx.ExecContext(r.Context(),
			`INSERT INTO mailgun_webhook_events (event_id) VALUES ($1) ON CONFLICT (event_id) DO NOTHING`, payload.EventData.ID,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if rows == 0 {
			// Already processed this Mailgun event id -- commit the
			// no-op tx and acknowledge, without recording it twice.
			if err := tx.Commit(); err == nil {
				committed = true
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		// The suppression row is the half of this handler that reaches
		// every outbox, not only portal_invite_outbox: ADR-0011 puts all
		// eleven mail kinds on one Mailgun domain, so one complaint has
		// to stop every one of them. Written before the portal-invite
		// UPDATE below and in the same transaction, so an address is
		// never marked 'complained' on a row without the suppression
		// that makes the next send refuse.
		if suppressionCause != "" {
			if err := mailsuppress.Record(r.Context(), tx, payload.EventData.Recipient, suppressionCause, payload.EventData.ID); err != nil {
				// coverage:ignore reason: DB write failure, not exercised by unit tests
				apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		// Licenses the client_portal_users/clients SELECT policies
		// 00032_portal_invite_outbox.sql granted to
		// app.notification_worker_trusted -- the signature check above
		// stands in for the membership check staffauth.Middleware would
		// otherwise perform.
		if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Matched by recipient address, not a stored Mailgun message id
		// -- mail.Sender.Send discards Mailgun's response body, so
		// there is nothing more precise to join on. Of every 'sent' row
		// for this address, the most recently sent one is picked; if
		// the same address received more than one invite, a bounce for
		// an older send would land on the newer row instead. Acceptable
		// for record-only, pre-launch (#340) -- #346's Staff screen
		// should read this state knowing that limit. An address with no
		// matching sent row (nothing sent yet, or already
		// bounced/complained/dead_lettered) is a silent no-op.
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE portal_invite_outbox SET status = $1
			 WHERE id = (
			     SELECT o.id FROM portal_invite_outbox o
			     JOIN client_portal_users pu ON pu.id = o.client_portal_user_id
			     JOIN clients c ON c.id = pu.client_id
			     WHERE lower(c.email) = lower($2) AND o.status = 'sent'
			     ORDER BY o.sent_at DESC
			     LIMIT 1
			     FOR UPDATE OF o
			 )`,
			newStatus, payload.EventData.Recipient,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true
		w.WriteHeader(http.StatusOK)
	})
}

// validMailgunSignature verifies Mailgun's webhook signature: hex-encoded
// HMAC-SHA256 of timestamp+token, keyed by the account's HTTP webhook
// signing key (documentation.mailgun.com/docs/mailgun/user-manual/webhooks/securing-webhooks).
func validMailgunSignature(signingKey, timestamp, token, signature string) bool {
	if timestamp == "" || token == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(timestamp + token))
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}
