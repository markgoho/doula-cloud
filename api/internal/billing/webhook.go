package billing

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/staffauth"
)

// maxWebhookBodyBytes bounds how much of a webhook request body
// PostPurchaseWebhookHandler will read -- Stripe event payloads are
// small; this just stops an unbounded read on an unauthenticated
// endpoint.
const maxWebhookBodyBytes = 1 << 20 // 1 MiB

// PostPurchaseWebhookHandler receives Stripe's Customer/Checkout events --
// a new, platform-level webhook endpoint, entirely separate from #38's
// (not yet implemented) Connect webhook, which is a different Stripe
// object type. It verifies the Stripe-Signature header itself (no
// staffauth.Middleware in front of it -- there is no Practice session to
// authenticate), and on a confirmed `checkout.session.completed` event
// with payment_status "paid", appends a `+N purchase` credit_ledger row
// for the Practice named in the event's metadata. Replays of the same
// Stripe event id are recorded in stripe_webhook_events and skipped, so a
// retried delivery never double-credits the ledger. db must be the same
// low-privilege app_runtime connection staffauth.Middleware uses.
func PostPurchaseWebhookHandler(db *sql.DB, webhookSecret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes))
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		event, err := stripe.ConstructEvent(body, r.Header.Get("Stripe-Signature"), webhookSecret,
			stripe.WithIgnoreAPIVersionMismatch())
		if err != nil {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}

		// Every other event type -- including payment_intent.succeeded,
		// which fires alongside checkout.session.completed for the same
		// Checkout but under a *different* event id -- is acknowledged and
		// ignored. Crediting off both would double-credit a single
		// purchase, since dedupe is keyed by event id, not by the
		// underlying Checkout Session.
		if event.Type != stripe.EventTypeCheckoutSessionCompleted {
			w.WriteHeader(http.StatusOK)
			return
		}

		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Async payment methods can leave a completed Checkout Session
		// unpaid; only a confirmed payment ever credits the ledger.
		if session.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
			w.WriteHeader(http.StatusOK)
			return
		}

		practiceID := session.Metadata["practice_id"]
		if !staffauth.ParseUUID(w, "practice", practiceID) {
			return
		}
		quantity, err := strconv.Atoi(session.Metadata["quantity"])
		if err != nil || quantity < 1 {
			http.Error(w, "invalid quantity metadata", http.StatusBadRequest)
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
			if !committed {
				_ = tx.Rollback()
			}
		}()

		// The webhook runs with no staffauth.Middleware in front of it, so
		// nothing else sets this session variable -- the Stripe signature
		// verified above is what licenses setting it here, standing in for
		// the membership check Middleware would otherwise perform.
		if _, err := tx.ExecContext(r.Context(),
			`SELECT set_config('app.current_practice_id', $1, true)`, practiceID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

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
			// tx and acknowledge, without crediting a second time.
			if err := tx.Commit(); err == nil {
				committed = true
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'purchase', $2)`,
			practiceID, quantity,
		); err != nil {
			// Includes a foreign-key violation if practiceID -- taken
			// verbatim from event metadata -- doesn't name a real Practice.
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
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
