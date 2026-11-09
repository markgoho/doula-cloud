package billing

import (
	"database/sql"
	"encoding/json/v2"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/dbtx"
	"doula-cloud/api/internal/pgerr"
	"doula-cloud/api/internal/stripemode"
	"doula-cloud/api/internal/webhookarrival"
)

func refusePayload(w http.ResponseWriter, r *http.Request, event stripe.Event, field, message string) {
	log.Printf("billing: purchase webhook: refusing event %s (%s): %s", event.ID, event.Type, field)
	webhookarrival.Mark(r.Context(), webhookarrival.PayloadRefused)
	apierr.WriteError(w, message, http.StatusBadRequest)
}

// purchaseEventType is the one event type PostPurchaseWebhookHandler acts
// on. stripe_destinations_test.go holds it equal to the platform route's
// enabled_events in terraform/stripe/destinations.json (#2390).
const purchaseEventType = stripe.EventTypeCheckoutSessionCompleted

// HandledEventTypes is every event type PostPurchaseWebhookHandler acts on.
func HandledEventTypes() []string {
	return []string{string(purchaseEventType)}
}

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
func PostPurchaseWebhookHandler(db *sql.DB, webhookSecret string, mode stripemode.Mode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		event, ok := webhookarrival.OpenStripe(w, r, "billing: purchase webhook", mode,
			func(body []byte, sigHeader string) (stripe.Event, error) {
				return stripe.ConstructEvent(body, sigHeader, webhookSecret, stripe.WithIgnoreAPIVersionMismatch())
			},
			func(e stripe.Event) (string, bool) { return e.ID, e.Livemode })
		if !ok {
			return
		}

		// Every other event type -- including payment_intent.succeeded,
		// which fires alongside checkout.session.completed for the same
		// Checkout but under a *different* event id -- is acknowledged and
		// ignored. Crediting off both would double-credit a single
		// purchase, since dedupe is keyed by event id, not by the
		// underlying Checkout Session.
		if event.Type != purchaseEventType {
			webhookarrival.Mark(r.Context(), webhookarrival.Dropped)
			w.WriteHeader(http.StatusOK)
			return
		}

		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Async payment methods can leave a completed Checkout Session
		// unpaid; only a confirmed payment ever credits the ledger.
		if session.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
			w.WriteHeader(http.StatusOK)
			return
		}

		practiceID := session.Metadata["practice_id"]
		if _, err := uuid.Parse(practiceID); err != nil {
			refusePayload(w, r, event, "metadata.practice_id", "invalid practice id")
			return
		}
		quantity, err := strconv.Atoi(session.Metadata["quantity"])
		if err != nil || quantity < 1 {
			refusePayload(w, r, event, "metadata.quantity", "invalid quantity metadata")
			return
		}

		// What the lot cost, taken from the Session rather than from our
		// own idea of the price, because the Session is what the Practice
		// actually paid. amount_subtotal is the pre-tax figure -- every
		// line item is tax_behavior "exclusive" (apportion.go), so
		// amount_total would fold New York's tax into the unit price and
		// refund it twice.
		//
		// A subtotal that does not divide evenly by the quantity would
		// mean storing a lossy price, and a lossy price is a refund
		// computed wrong three years from now. Refuse it and let Stripe
		// retry: nothing has been written yet.
		if session.AmountSubtotal <= 0 || session.AmountSubtotal%int64(quantity) != 0 {
			refusePayload(w, r, event, "amount_subtotal", "checkout session subtotal does not divide by quantity")
			return
		}
		unitPriceCents := session.AmountSubtotal / int64(quantity)
		var taxCents int64
		if session.TotalDetails != nil {
			taxCents = session.TotalDetails.AmountTax
		}
		// The object a refund has to be issued against, so Stripe Tax
		// reverses the tax it reported to New York. Unexpanded in a
		// webhook payload, so this carries the id and nothing else --
		// which is all the ledger keeps.
		if session.PaymentIntent == nil || session.PaymentIntent.ID == "" {
			refusePayload(w, r, event, "payment_intent", "checkout session has no payment intent")
			return
		}

		tx, ok := dbtx.Begin(w, r, db)
		if !ok {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}
		defer dbtx.Rollback(tx)

		// The webhook runs with no staffauth.Middleware in front of it, so
		// nothing else sets this session variable -- the Stripe signature
		// verified above is what licenses setting it here, standing in for
		// the membership check Middleware would otherwise perform.
		if _, err := tx.ExecContext(r.Context(),
			`SELECT set_config('app.current_practice_id', $1, true)`, practiceID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		rows, ok := dbtx.ExecRows(r.Context(), w, tx,
			`INSERT INTO stripe_webhook_events (event_id) VALUES ($1) ON CONFLICT (event_id) DO NOTHING`, event.ID,
		)
		if !ok {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}
		if rows == 0 {
			// Already processed this Stripe event id -- commit the no-op
			// tx and acknowledge, without crediting a second time.
			_ = tx.Commit()
			w.WriteHeader(http.StatusOK)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO credit_ledger
			     (practice_id, origin, quantity, unit_price_cents, tax_cents, stripe_payment_intent_id)
			 VALUES ($1, 'purchase', $2, $3, $4, $5)`,
			practiceID, quantity, unitPriceCents, taxCents, session.PaymentIntent.ID,
		); err != nil {
			if pgerr.IsForeignKeyViolation(err) {
				refusePayload(w, r, event, "metadata.practice_id: no such Practice", "no such practice")
				return
			}
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		dbtx.CommitWith(w, tx, http.StatusOK)
	})
}
