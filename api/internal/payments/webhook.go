package payments

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/staffauth"
)

// maxWebhookBodyBytes bounds how much of a webhook request body
// PostConnectWebhookHandler will read -- mirrors billing's own bound on
// its (separate) webhook endpoint.
const maxWebhookBodyBytes = 1 << 20 // 1 MiB

// eventTypeAccountUpdated is the only Stripe Connect event type this
// ticket (#80) handles. #82 extends this same endpoint with
// invoice.paid / invoice.payment_failed. WebhookEvent.Type is a plain
// string (the Client port abstracts away *stripe.Event), so stripe-go's
// own typed constant is converted once here rather than hand-rolling the
// literal.
const eventTypeAccountUpdated = string(stripe.EventTypeAccountUpdated)

// accountUpdatedObject is the subset of an account.updated event's Account
// object payments cares about -- the same three capabilities persisted on
// practices.stripe_connect_*.
type accountUpdatedObject struct {
	ChargesEnabled   bool `json:"charges_enabled"`
	PayoutsEnabled   bool `json:"payouts_enabled"`
	DetailsSubmitted bool `json:"details_submitted"`
}

// PostConnectWebhookHandler receives Stripe's Connect account events -- a
// new, platform-level webhook endpoint, entirely separate from billing's
// Customer/Checkout webhook (#77), which is a different Stripe object
// type with its own signing secret. It verifies the Stripe-Signature
// header itself via client.VerifyWebhookSignature (no staffauth.Middleware
// in front of it -- there is no Practice session to authenticate), and on
// a recognized `account.updated` event updates the three capability
// booleans on the practices row whose stripe_connect_account_id matches
// the event's account field. An event for an unrecognized account id, or
// any event type other than account.updated, is logged and dropped rather
// than treated as an error -- Stripe retries indefinitely on anything but
// a 2xx. Replays of the same Stripe event id are recorded in
// stripe_webhook_events (#77's idempotency table, reused here per #80's
// ticket body rather than a second table) and skipped, so a retried
// delivery never re-applies a status transition. db must be the same
// low-privilege app_runtime connection staffauth.Middleware uses.
func PostConnectWebhookHandler(db *sql.DB, client Client, webhookSecret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes))
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		event, err := client.VerifyWebhookSignature(body, r.Header.Get("Stripe-Signature"), webhookSecret)
		if err != nil {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}

		if event.Type != eventTypeAccountUpdated {
			log.Printf("payments: connect webhook: dropping unhandled event type %q (id %s)", event.Type, event.ID)
			w.WriteHeader(http.StatusOK)
			return
		}

		var account accountUpdatedObject
		if err := json.Unmarshal(event.Data, &account); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed := false
		defer func() {
			// coverage:ignore reason: only reached after a DB failure post-BeginTx; every such failure path below is itself coverage:ignore'd (unlike billing's webhook, nothing here can fail on ordinary bad input), so this rollback is never exercised by unit tests
			if !committed {
				_ = tx.Rollback()
			}
		}()

		result, err := tx.ExecContext(r.Context(),
			`INSERT INTO stripe_webhook_events (event_id) VALUES ($1) ON CONFLICT (event_id) DO NOTHING`, event.ID,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if rows == 0 {
			// Already processed this Stripe event id -- commit the no-op
			// tx and acknowledge, without re-applying the status update.
			if err := tx.Commit(); err == nil {
				committed = true
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		result, err = tx.ExecContext(r.Context(),
			`UPDATE practices SET
				stripe_connect_charges_enabled = $1,
				stripe_connect_payouts_enabled = $2,
				stripe_connect_details_submitted = $3
			WHERE stripe_connect_account_id = $4`,
			account.ChargesEnabled, account.PayoutsEnabled, account.DetailsSubmitted, event.Account,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		rows, err = result.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if rows == 0 {
			// No practice has this connected account id -- logged and
			// dropped, per #80's ticket body, not treated as an error.
			// The stripe_webhook_events row inserted above still commits,
			// so a retried delivery of this same unrecognized event is
			// also a no-op rather than logging twice.
			log.Printf("payments: connect webhook: dropping account.updated for unrecognized account %q (event id %s)", event.Account, event.ID)
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true
		w.WriteHeader(http.StatusOK)
	})
}
