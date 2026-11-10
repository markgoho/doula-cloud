package payments

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/dbtx"
	"doula-cloud/api/internal/invoicestatus"
	"doula-cloud/api/internal/stripemode"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/webhookarrival"
)

// The v1 snapshot event types the Connect endpoint handles: #82's
// invoice.paid / invoice.payment_failed below, #2224's
// invoice.marked_uncollectible, invoice.voided (an Invoice voided in the
// Practice's Stripe Dashboard), #2225's payment_intent.processing, and #1009's
// credit_note.created / refund.created (refund_webhook.go). Any other
// event type is logged and dropped. WebhookEvent.Type is a plain string (the Client port
// abstracts away *stripe.Event), so stripe-go's own typed constants are
// converted once here rather than hand-rolling the literals.
//
// #80's account.updated is no longer what this endpoint acts on. A v2
// account does still emit v1 snapshot account.updated on the connected
// account, but its payload is the v1 model -- the three booleans -- which
// cannot express the four-valued capability statuses the practices
// columns now hold. The authoritative v2 state arrives as a thin event at
// PostAccountWebhookHandler below. Any v1 account event that still
// reaches this endpoint falls through to the default branch and is
// logged and dropped.
const (
	eventTypeInvoicePaid                = string(stripe.EventTypeInvoicePaid)
	eventTypeInvoicePaymentFailed       = string(stripe.EventTypeInvoicePaymentFailed)
	eventTypeInvoiceMarkedUncollectible = string(stripe.EventTypeInvoiceMarkedUncollectible)
	eventTypeInvoiceVoided              = string(stripe.EventTypeInvoiceVoided)
	eventTypePaymentIntentProcessing    = string(stripe.EventTypePaymentIntentProcessing)
)

// paymentIntentStatusProcessing is a PaymentIntent's status while its
// money is on its way -- a bank debit, for days (#2225).
const paymentIntentStatusProcessing = string(stripe.PaymentIntentStatusProcessing)

// eventTypeMerchantCapabilityStatusUpdated is the one v2 thin event type
// PostAccountWebhookHandler acts on: a capability on the account's
// merchant configuration changed status. stripe-go v86.3.0 ships no
// constant for the bracketed v2 event names, so this is the literal
// Stripe sends.
const eventTypeMerchantCapabilityStatusUpdated = "v2.core.account[configuration.merchant].capability_status_updated"

// invoicePaidObject is the subset of an invoice.paid event's Invoice
// object #82's handler needs: the Stripe invoice id to resolve the
// matching invoices row, the amount actually paid (may differ from the
// Invoice's own amount_cents in principle, though v1 models no partial
// payment), a Stripe reference for the payment itself, and the Unix
// timestamp Stripe recorded the payment at.
// The Stripe reference for the payment itself is deliberately absent
// here. Under API version 2026-07-29.dahlia an Invoice carries neither
// payment_intent nor charge, and the invoice.paid event body carries no
// payments list either -- all three verified against the Sandbox during
// #247's walk, where the old `payment_intent` field silently unmarshaled
// to "" and every payments row was written with an empty reference. The
// handler fetches it through the port instead.
type invoicePaidObject struct {
	ID                string `json:"id"`
	AmountPaid        int64  `json:"amount_paid"`
	StatusTransitions struct {
		PaidAt int64 `json:"paid_at"`
	} `json:"status_transitions"`
}

// invoiceEventObject is the subset of an invoice.payment_failed or
// invoice.marked_uncollectible event's Invoice object their handlers
// need -- just enough to resolve the matching invoices row; no Payment
// is ever created on either event.
type invoiceEventObject struct {
	ID string `json:"id"`
}

// PostConnectWebhookHandler receives Stripe's Connect *Invoice* events --
// a platform-level webhook endpoint, entirely separate from billing's
// Customer/Checkout webhook (#77), which is a different Stripe object
// type with its own signing secret. It verifies the Stripe-Signature
// header itself via client.VerifyWebhookSignature (no staffauth.Middleware
// in front of it -- there is no Practice session to authenticate), then
// dispatches on the event type. Each Invoice event resolves the matching
// invoices row (via its stored Stripe invoice id, scoped by the event's
// account field): invoice.paid flips it to paid and creates exactly one
// payments row; invoice.payment_failed records the failed attempt and
// leaves it open, as Stripe does (#2224); invoice.marked_uncollectible
// moves it to uncollectible; invoice.voided moves it to void;
// payment_intent.processing marks a payment on its way and leaves it open
// (#2225). Every move is one lifecycle.go declares, asked through
// takeStripeMove, and a move the table refuses is logged and
// acknowledged. An event for an
// unrecognized account id, an Invoice not found in DoulaCloud, or any
// event type not handled here, is logged and dropped rather than
// treated as an error -- Stripe retries indefinitely on anything but a
// 2xx. Replays of the same Stripe event id are recorded in
// stripe_webhook_events (#77's idempotency table, reused here per #80's
// and #82's ticket bodies rather than a second table) and skipped, so a
// retried delivery never re-applies a status transition or creates a
// second payments row. db must be the same low-privilege app_runtime
// connection staffauth.Middleware uses.
//
// This endpoint carries *snapshot* events only. A Stripe event
// destination has one event_payload for all of its events -- subscribing
// one destination to both a thin and a snapshot event type is rejected
// outright (verified against the Sandbox, #247) -- so the v2 account
// events go to PostAccountWebhookHandler on its own route, with its own
// secret.
func PostConnectWebhookHandler(db *sql.DB, client Client, webhookSecret string, mode stripemode.Mode, enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		event, ok := webhookarrival.OpenStripe(w, r, "payments: connect webhook", mode,
			func(body []byte, sigHeader string) (WebhookEvent, error) {
				return client.VerifyWebhookSignature(body, sigHeader, webhookSecret)
			},
			func(e WebhookEvent) (string, bool) { return e.ID, e.Livemode })
		if !ok {
			return
		}

		handle, ok := connectEventHandlers[event.Type]
		if !ok {
			webhookarrival.Mark(r.Context(), webhookarrival.Dropped)
			log.Printf("payments: connect webhook: dropping unhandled event type %q (id %s)", event.Type, event.ID)
			w.WriteHeader(http.StatusOK)
			return
		}
		handle(w, r, db, client, event, enq)
	})
}

type connectEventHandler func(w http.ResponseWriter, r *http.Request, db *sql.DB, client Client, event WebhookEvent, enq tasknudge.Enqueuer)

// connectEventHandlers is every event type PostConnectWebhookHandler acts
// on. stripe_destinations_test.go holds it equal to the connect route's
// enabled_events in terraform/stripe/destinations.json (#2390).
var connectEventHandlers = map[string]connectEventHandler{
	eventTypeInvoicePaid: handleInvoicePaid,
	eventTypeInvoicePaymentFailed: func(w http.ResponseWriter, r *http.Request, db *sql.DB, client Client, event WebhookEvent, _ tasknudge.Enqueuer) {
		handleInvoicePaymentFailed(w, r, db, client, event)
	},
	eventTypeInvoiceMarkedUncollectible: func(w http.ResponseWriter, r *http.Request, db *sql.DB, _ Client, event WebhookEvent, _ tasknudge.Enqueuer) {
		handleInvoiceMarkedUncollectible(w, r, db, event)
	},
	eventTypeInvoiceVoided: func(w http.ResponseWriter, r *http.Request, db *sql.DB, _ Client, event WebhookEvent, _ tasknudge.Enqueuer) {
		handleInvoiceVoided(w, r, db, event)
	},
	eventTypePaymentIntentProcessing: func(w http.ResponseWriter, r *http.Request, db *sql.DB, client Client, event WebhookEvent, _ tasknudge.Enqueuer) {
		handlePaymentIntentProcessing(w, r, db, client, event)
	},
	eventTypeCreditNoteCreated: func(w http.ResponseWriter, r *http.Request, db *sql.DB, _ Client, event WebhookEvent, _ tasknudge.Enqueuer) {
		handleCreditNoteCreated(w, r, db, event)
	},
	eventTypeRefundCreated: func(w http.ResponseWriter, r *http.Request, db *sql.DB, client Client, event WebhookEvent, _ tasknudge.Enqueuer) {
		handleRefundCreated(w, r, db, client, event)
	},
}

// ConnectEventTypes is every event type the connect route acts on, sorted.
func ConnectEventTypes() []string {
	return slices.Sorted(maps.Keys(connectEventHandlers))
}

// AccountEventTypes is every event type the account route acts on.
func AccountEventTypes() []string {
	return []string{eventTypeMerchantCapabilityStatusUpdated}
}

// beginEvent begins a transaction and records eventID in
// stripe_webhook_events, the shared idempotency prologue every event-type
// branch below runs before doing any type-specific work. ok=false means
// the response is already written: a 500, or the 200 that acknowledges a
// redelivery whose event id an earlier delivery recorded (nothing is
// re-applied). Callers defer rollbackUnlessCommitted(tx, committed) and
// pass committed to commitAndAck.
func beginEvent(w http.ResponseWriter, r *http.Request, db *sql.DB, eventID string) (tx *sql.Tx, committed *bool, ok bool) {
	tx, ok = dbtx.Begin(w, r, db)
	if !ok {
		// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
		return nil, nil, false
	}
	rows, ok := dbtx.ExecRows(r.Context(), w, tx,
		`INSERT INTO stripe_webhook_events (event_id) VALUES ($1) ON CONFLICT (event_id) DO NOTHING`, eventID,
	)
	if !ok {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		_ = tx.Rollback()
		return nil, nil, false
	}
	if rows == 0 {
		commitAndAck(w, tx, new(bool))
		_ = tx.Rollback()
		return nil, nil, false
	}
	return tx, new(bool), true
}

// rollbackUnlessCommitted is the deferred half of beginEvent.
func rollbackUnlessCommitted(tx *sql.Tx, committed *bool) {
	if !*committed {
		// coverage:ignore reason: only reached after a DB failure, not exercised by unit tests
		_ = tx.Rollback()
	}
}

// commitAndAck commits tx and, on success, marks *committed true (so the
// caller's deferred rollback becomes a no-op) and writes a 200. On a
// commit failure it writes 500 instead, leaving *committed false so the
// deferred rollback runs.
func commitAndAck(w http.ResponseWriter, tx *sql.Tx, committed *bool) {
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB commit failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	*committed = true
	w.WriteHeader(http.StatusOK)
}

// resolvePracticeForEvent resolves accountID to a Practice and scopes the
// rest of tx to it (app.current_practice_id), so every later read and
// write on invoices and payments is filtered by RLS too. Split out of
// resolveInvoiceForEvent for #1009's refund.created, which reaches a
// Payment by its PaymentIntent and has no Stripe invoice id to look up.
// sql.ErrNoRows for an account no Practice holds.
func resolvePracticeForEvent(ctx context.Context, tx *sql.Tx, accountID string) (practiceID string, err error) {
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM practices WHERE stripe_connect_account_id = $1`, accountID,
	).Scan(&practiceID); err != nil {
		// coverage:ignore reason: the sql.ErrNoRows branch (unrecognized account) is exercised by unit tests; a non-ErrNoRows DB failure here is not
		return "", fmt.Errorf("payments: resolve practice for webhook event: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("payments: scope webhook event tx to practice: %w", err)
	}
	return practiceID, nil
}

// resolveInvoiceForEvent resolves accountID to a Practice (the same
// account-resolution rule handleCapabilityStatusUpdated uses) and, on a
// match,
// sets app.current_practice_id on tx for the rest of the transaction --
// the session variable staffauth.Middleware would otherwise have set
// per-request, which this webhook never gets (see
// PostConnectWebhookHandler's doc comment) -- before looking up the
// invoices row matching stripeInvoiceID. Setting the session var first,
// rather than joining invoices to practices directly, means the INSERT/
// UPDATE callers issue afterward on invoices and payments are correctly
// scoped by invoices_practice_visibility and payments_practice_visibility
// too, not just this read. Returns sql.ErrNoRows if either the account or
// the Stripe invoice id is unrecognized, so callers can log-and-drop
// rather than error. Also returns practiceID -- handleInvoicePaid needs
// it to queue a payment_received_outbox row (#344).
func resolveInvoiceForEvent(ctx context.Context, tx *sql.Tx, stripeInvoiceID, accountID string) (invoiceID, practiceID string, err error) {
	practiceID, err = resolvePracticeForEvent(ctx, tx, accountID)
	if err != nil {
		return "", "", err
	}

	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM invoices WHERE stripe_invoice_id = $1`, stripeInvoiceID,
	).Scan(&invoiceID); err != nil {
		// coverage:ignore reason: the sql.ErrNoRows branch (unknown invoice) is exercised by unit tests; a non-ErrNoRows DB failure here is not
		return "", "", fmt.Errorf("payments: resolve invoice for webhook event: %w", err)
	}
	return invoiceID, practiceID, nil
}

// resolveEventInvoice is resolveInvoiceForEvent for an event handler. An
// unrecognized account or unknown Stripe invoice id is logged and dropped
// -- committed and acknowledged, per #82's ticket body, not treated as an
// error -- and ok is false; a database failure is a 500.
func resolveEventInvoice(w http.ResponseWriter, r *http.Request, tx *sql.Tx, committed *bool, event WebhookEvent, stripeInvoiceID string) (invoiceID, practiceID string, ok bool) {
	invoiceID, practiceID, err := resolveInvoiceForEvent(r.Context(), tx, stripeInvoiceID, event.Account)
	if errors.Is(err, sql.ErrNoRows) {
		webhookarrival.Mark(r.Context(), webhookarrival.Dropped)
		log.Printf("payments: connect webhook: dropping %s for unresolved invoice %q (account %q, event id %s)", event.Type, stripeInvoiceID, event.Account, event.ID)
		commitAndAck(w, tx, committed)
		return "", "", false
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return "", "", false
	}
	return invoiceID, practiceID, true
}

// invoiceEvent is what an event about one Invoice holds once it is claimed
// and resolved: the open transaction, whether it committed, and the
// Invoice and Practice the Stripe invoice id named.
type invoiceEvent struct {
	tx                    *sql.Tx
	committed             *bool
	invoiceID, practiceID string
}

func (e invoiceEvent) rollbackUnlessCommitted() {
	rollbackUnlessCommitted(e.tx, e.committed)
}

// beginInvoiceEvent is the prologue of every handler for an event about
// one Invoice: decode the payload, claim the event, and resolve the
// Invoice it names (stripeInvoiceID reads that id off the decoded
// payload). ok=false means the response is already written -- a 500, or
// the 200 acknowledging a replay or an event for an Invoice this system
// does not know. Callers defer ev.rollbackUnlessCommitted() straight
// after.
func beginInvoiceEvent[T any](w http.ResponseWriter, r *http.Request, db *sql.DB, event WebhookEvent, stripeInvoiceID func(T) string) (obj T, ev invoiceEvent, ok bool) {
	if err := json.Unmarshal(event.Data, &obj); err != nil {
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return obj, invoiceEvent{}, false
	}
	tx, committed, ok := beginEvent(w, r, db, event.ID)
	if !ok {
		return obj, invoiceEvent{}, false
	}
	invoiceID, practiceID, ok := resolveEventInvoice(w, r, tx, committed, event, stripeInvoiceID(obj))
	if !ok {
		rollbackUnlessCommitted(tx, committed)
		return obj, invoiceEvent{}, false
	}
	return obj, invoiceEvent{tx: tx, committed: committed, invoiceID: invoiceID, practiceID: practiceID}, true
}

// recordInvoicePaid writes #476's activity row for the invoice.paid
// event -- actor_kind 'client' (ADR-0022: "Amara paid the invoice" is
// her act), the paying Client's identity threaded through via
// invoices -> contracts -> engagements rather than invented: only the
// Engagement's own Client holds the Stripe Hosted Invoice link this
// payment came from. app.current_practice_id is already set on tx by
// resolveInvoiceForEvent, called just above this in the caller, so no
// further scoping is needed here.
func recordInvoicePaid(ctx context.Context, tx *sql.Tx, practiceID, invoiceID string) error {
	return recordInvoiceEvent(ctx, tx, practiceID, invoiceID, activity.ActionInvoicePaid, nil, true)
}

// recordInvoiceEvent writes one Engagement activity row for something
// Stripe reported about invoiceID, reaching the Engagement and its
// Client through invoices -> contracts -> engagements. byClient picks the
// actor: the Client herself for her own payment or attempt, DoulaCloud
// for what the Practice did in her own Stripe account (ADR-0022).
func recordInvoiceEvent(ctx context.Context, tx *sql.Tx, practiceID, invoiceID string, action activity.EngagementAction, diff jsontext.Value, byClient bool) error {
	var engagementID, clientID string
	if err := tx.QueryRowContext(ctx,
		`SELECT e.id, e.client_id
		 FROM invoices i
		 JOIN contracts c ON c.id = i.contract_id
		 JOIN engagements e ON e.id = c.engagement_id
		 WHERE i.id = $1`,
		invoiceID,
	).Scan(&engagementID, &clientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: resolve engagement for %s: %w", action, err)
	}
	actor := activity.SystemActor()
	if byClient {
		actor = activity.ClientActor(clientID)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(action),
		Diff:        diff,
		Actor:       actor,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: record %s: %w", action, err)
	}
	return nil
}

// handleInvoicePaid applies #82's invoice.paid handling: resolves the
// matching invoices row, creates exactly one payments row recording the
// amount actually paid and when, and flips the Invoice's status to paid.
func handleInvoicePaid(w http.ResponseWriter, r *http.Request, db *sql.DB, client Client, event WebhookEvent, enq tasknudge.Enqueuer) {
	inv, ev, ok := beginInvoiceEvent(w, r, db, event, func(o invoicePaidObject) string { return o.ID })
	if !ok {
		return
	}
	defer ev.rollbackUnlessCommitted()
	tx, committed, invoiceID, practiceID := ev.tx, ev.committed, ev.invoiceID, ev.practiceID

	// #271's already-paid guard, now TransitionStripePaid's: FOR UPDATE
	// locks the row against a concurrent writer -- specifically
	// PostManualPaymentHandler's own paid_out_of_band call, whose own
	// invoice.paid echo can arrive here while that handler's transaction
	// is still in flight -- and an Invoice already paid is the ordinary
	// double-delivery case (a real card payment webhook redelivered
	// outside claimEvent's own dedup window is not the only way this
	// event type can repeat), acknowledged with nothing left to do. A void
	// Invoice is refused, not paid (TransitionStripePaid's comment).
	if !takeStripeMove(w, r, ev, event, TransitionStripePaid) {
		return
	}

	paidAt := time.Unix(inv.StatusTransitions.PaidAt, 0).UTC()

	// The reference is what makes a payments row traceable back to Stripe,
	// and it is not in the event. A failure to fetch it must not lose the
	// payment: log it and store an empty reference rather than 500 and
	// have Stripe redeliver an event whose money already moved. A later
	// refund.created still finds such a Payment, through the Invoice its
	// PaymentIntent pays (refundedStripePayment).
	reference, err := client.RetrieveInvoicePaymentReference(r.Context(), event.Account, inv.ID)
	if err != nil {
		log.Printf("payments: connect webhook: invoice.paid %s recorded without a Stripe payment reference: %v", inv.ID, err)
		reference = ""
	}

	var paymentID string
	if err := tx.QueryRowContext(r.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at, kind) VALUES ($1, $2, $3, $4, 'stripe') RETURNING id`,
		invoiceID, reference, inv.AmountPaid, paidAt,
	).Scan(&paymentID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if err := moveInvoice(r.Context(), tx, invoiceID, TransitionStripePaid, `, paid_at = $3, payment_processing_since = NULL`, paidAt); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if err := recordInvoicePaid(r.Context(), tx, practiceID, invoiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	// Queue #344's "a Payment arrived" Platform Notification on the same
	// tx and after the payments row exists to satisfy payment_id's FK --
	// see 00035_payment_received_outbox.sql for why this needs no
	// rollback-surviving write the way QueueOutOfCreditsNotification does.
	if err := QueuePaymentReceivedNotification(r.Context(), tx, paymentID, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	commitAndAck(w, tx, committed)
	// ADR-0013: fired only on this success path, gated on committed --
	// unlike the alreadyProcessed/dropped branches above (which also call
	// commitAndAck but queued no row), and unlike a failed commit, which
	// leaves the just-queued payment_received_outbox row rolled back too.
	if *committed {
		tasknudge.Fire(enq, tasknudge.PaymentReceived)(r.Context())
	}
}

// lockInvoiceStatus reads an Invoice's status under FOR UPDATE, so two
// deliveries touching the same Invoice serialize. The row is known to
// exist: resolveInvoiceForEvent found it a moment ago.
func lockInvoiceStatus(w http.ResponseWriter, r *http.Request, tx *sql.Tx, invoiceID string) (invoicestatus.Status, bool) {
	var status invoicestatus.Status
	if err := tx.QueryRowContext(r.Context(),
		`SELECT status FROM invoices WHERE id = $1 FOR UPDATE`, invoiceID,
	).Scan(&status); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- resolveInvoiceForEvent already proved this row exists
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return "", false
	}
	return status, true
}

// takeStripeMove is how every Connect webhook handler that moves an
// Invoice asks the lifecycle table first: it locks the row ev names and
// asks t.Check whether Stripe's event may move it from where it stands.
// proceed=false means the response is already written. An Invoice already
// where t leaves it is a redelivery under another event id: acknowledged,
// nothing applied. One t does not start from is refused: logged, counted
// as refused on the Platform's webhook screen so a person can reconcile
// it with Stripe, and acknowledged -- never a 500, which Stripe would
// redeliver for days with the same answer waiting.
func takeStripeMove(w http.ResponseWriter, r *http.Request, ev invoiceEvent, event WebhookEvent, t InvoiceTransition) (proceed bool) {
	current, ok := lockInvoiceStatus(w, r, ev.tx, ev.invoiceID)
	if !ok {
		// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure, not exercised by unit tests
		return false
	}
	if t.Reached(current) {
		log.Printf("payments: connect webhook: %s for invoice %s already %s, skipping (event id %s)", event.Type, ev.invoiceID, current, event.ID)
		commitAndAck(w, ev.tx, ev.committed)
		return false
	}
	if ok, refusal := t.Check(current, BillingModeStripe); !ok {
		webhookarrival.Mark(r.Context(), webhookarrival.Refused)
		log.Printf("payments: connect webhook: refusing %s for invoice %s: %s (event id %s)", event.Type, ev.invoiceID, refusal, event.ID)
		commitAndAck(w, ev.tx, ev.committed)
		return false
	}
	return true
}

// recordPaymentAttempt records an attempt to pay an Invoice on its
// activity trail, with the payment method when Stripe named one.
func recordPaymentAttempt(w http.ResponseWriter, r *http.Request, tx *sql.Tx, practiceID, invoiceID string, action activity.EngagementAction, methodType string) bool {
	diff := map[string]string{}
	if methodType != "" {
		diff[activity.DiffKeyPaymentMethodType] = methodType
	}
	diffJSON, err := json.Marshal(diff)
	if err != nil {
		// coverage:ignore reason: a map of strings always marshals
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if err := recordInvoiceEvent(r.Context(), tx, practiceID, invoiceID, action, diffJSON, true); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	return true
}

// handleInvoicePaymentFailed records one failed attempt to pay an
// Invoice (#2224) and leaves its status alone. Stripe sends this event on
// every failed attempt -- a declined card, a bank debit that bounces days
// later -- and keeps the Invoice open for the next one, so ours stays
// open too and she can pay it again. Creates no payments row. The method
// she tried is asked of Stripe, since the event names none; a failure to
// ask records the attempt without it rather than lose it, the rule
// handleInvoicePaid follows for its reference. It also arrives while a
// redirect method waits on her (requires_action), which is no failure
// and records nothing (#2225).
func handleInvoicePaymentFailed(w http.ResponseWriter, r *http.Request, db *sql.DB, client Client, event WebhookEvent) {
	inv, ev, ok := beginInvoiceEvent(w, r, db, event, func(o invoiceEventObject) string { return o.ID })
	if !ok {
		return
	}
	defer ev.rollbackUnlessCommitted()
	tx, committed, invoiceID, practiceID := ev.tx, ev.committed, ev.invoiceID, ev.practiceID

	// Stripe asks before anything is written: the same event arrives the
	// moment a redirect method sends her away to approve it, and that is
	// not a failure (#2225). A failure to ask records the attempt without
	// a method rather than lose it, the rule handleInvoicePaid follows.
	attempt, err := client.RetrieveInvoiceFailedAttempt(r.Context(), event.Account, inv.ID)
	if err != nil {
		log.Printf("payments: connect webhook: invoice.payment_failed %s recorded without a payment method: %v", inv.ID, err)
	}
	if attempt.AwaitingAction {
		log.Printf("payments: connect webhook: invoice.payment_failed for %q is waiting on the Client, not failed (event id %s)", inv.ID, event.ID)
		commitAndAck(w, tx, committed)
		return
	}

	// A payment that was on its way (#2225) and did not go through is no
	// longer on its way. The Invoice stays open for another try.
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE invoices SET payment_processing_since = NULL WHERE id = $1`, invoiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	if !recordPaymentAttempt(w, r, tx, practiceID, invoiceID, activity.ActionInvoicePaymentFailed, attempt.PaymentMethodType) {
		// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure, not exercised by unit tests
		return
	}

	commitAndAck(w, tx, committed)
}

// handleInvoiceMarkedUncollectible is the one way a Stripe-backed
// Invoice reaches 'uncollectible' (#2224): Stripe says it was marked so,
// whether the Practice wrote it off in her own Stripe Dashboard or a
// rule she set there did it. It moves an open Invoice and records
// invoice_written_off with DoulaCloud as the actor -- no Staff member of
// this system did it -- and an origin saying where it was done, the
// footing a Dashboard Refund stands on (#1009). An Invoice already paid
// or voided here is refused by TransitionStripeUncollectible: logged and
// acknowledged, unchanged.
func handleInvoiceMarkedUncollectible(w http.ResponseWriter, r *http.Request, db *sql.DB, event WebhookEvent) {
	handleDashboardMove(w, r, db, event, TransitionStripeUncollectible, activity.ActionInvoiceWrittenOff)
}

// handleInvoiceVoided is how a Stripe-backed Invoice reaches 'void': the
// Practice voided it in her own Stripe Dashboard, which is where
// MsgStripeInvoiceCannotBeVoidedOrWrittenOff sends her to do it. It
// records invoice_voided with DoulaCloud as the actor and the Dashboard
// as its origin, as a write-off from there does. Without it, an Invoice
// voided on Stripe stayed open here: owed, overdue, and refusing the
// Client's erasure and the Practice's deletion.
func handleInvoiceVoided(w http.ResponseWriter, r *http.Request, db *sql.DB, event WebhookEvent) {
	handleDashboardMove(w, r, db, event, TransitionStripeVoided, activity.ActionInvoiceVoided)
}

// handleDashboardMove is the shared body of the two events that report
// what the Practice did to an Invoice in her own Stripe account: t moves
// it -- clearing any payment on its way, which only an open Invoice can
// carry -- and action records it, by DoulaCloud, from the Dashboard.
func handleDashboardMove(w http.ResponseWriter, r *http.Request, db *sql.DB, event WebhookEvent, t InvoiceTransition, action activity.EngagementAction) {
	_, ev, ok := beginInvoiceEvent(w, r, db, event, func(o invoiceEventObject) string { return o.ID })
	if !ok {
		return
	}
	defer ev.rollbackUnlessCommitted()
	tx, committed, invoiceID, practiceID := ev.tx, ev.committed, ev.invoiceID, ev.practiceID

	if !takeStripeMove(w, r, ev, event, t) {
		return
	}

	if err := moveInvoice(r.Context(), tx, invoiceID, t, `, payment_processing_since = NULL`); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	diff, err := json.Marshal(map[string]string{"origin": activity.OriginStripeDashboard})
	if err != nil {
		// coverage:ignore reason: a map of strings always marshals
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if err := recordInvoiceEvent(r.Context(), tx, practiceID, invoiceID, action, diff, false); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	commitAndAck(w, tx, committed)
}

// paymentIntentEventObject is the subset of a payment_intent.* event's
// PaymentIntent object #2225's handler reads: its id. It names no
// Invoice under API version 2026-07-29.dahlia, so the handler asks.
type paymentIntentEventObject struct {
	ID string `json:"id"`
}

// handlePaymentIntentProcessing marks a payment on its way (#2225). A
// bank debit is 'processing' for days, and Stripe keeps its Invoice open
// meanwhile, so the status stays 'open' and payment_processing_since
// holds the fact beside it until invoice.paid or invoice.payment_failed
// clears it. The attempt is recorded as hers, with the method she used.
//
// The PaymentIntent names no Invoice, so Stripe is asked which one it
// pays -- and asked first, before the event is claimed: without that
// answer nothing can be applied, so a failure is a 500 Stripe retries,
// not a replay swallowed later (handleCapabilityStatusUpdated's rule).
// The same answer carries the PaymentIntent's status now, so a delivery
// that arrives after the payment already succeeded or failed marks
// nothing.
func handlePaymentIntentProcessing(w http.ResponseWriter, r *http.Request, db *sql.DB, client Client, event WebhookEvent) {
	var pi paymentIntentEventObject
	if err := json.Unmarshal(event.Data, &pi); err != nil {
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	intent, err := client.RetrievePaymentIntentInvoice(r.Context(), event.Account, pi.ID)
	if err != nil {
		log.Printf("payments: connect webhook: payment_intent.processing %s: invoice lookup failed: %v", pi.ID, err)
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	tx, committed, ok := beginEvent(w, r, db, event.ID)
	if !ok {
		return
	}
	defer rollbackUnlessCommitted(tx, committed)
	if intent.StripeInvoiceID == "" || intent.Status != paymentIntentStatusProcessing {
		log.Printf("payments: connect webhook: payment_intent.processing %s pays no invoice or is now %q, leaving it (event id %s)", pi.ID, intent.Status, event.ID)
		commitAndAck(w, tx, committed)
		return
	}

	invoiceID, practiceID, ok := resolveEventInvoice(w, r, tx, committed, event, intent.StripeInvoiceID)
	if !ok {
		return
	}

	// Only an open Invoice has a payment on its way
	// (TransitionPaymentOnItsWay): one paid by hand or voided meanwhile is
	// refused and left as it is.
	if !takeStripeMove(w, r, invoiceEvent{tx: tx, committed: committed, invoiceID: invoiceID, practiceID: practiceID}, event, TransitionPaymentOnItsWay) {
		return
	}
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE invoices SET payment_processing_since = $1 WHERE id = $2`,
		clock.Now(r.Context()), invoiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	if !recordPaymentAttempt(w, r, tx, practiceID, invoiceID, activity.ActionInvoicePaymentProcessing, intent.PaymentMethodType) {
		// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure, not exercised by unit tests
		return
	}

	commitAndAck(w, tx, committed)
}

// PostAccountWebhookHandler receives Stripe's v2 *thin* account events on
// their own route. It exists because Accounts v2 stopped emitting v1
// snapshot events for connected accounts entirely: creating a v2 Account
// in the Sandbox produced six v2 thin events and nothing at all on
// /v1/events (#247). stripe.ConstructEvent -- what
// VerifyWebhookSignature calls -- rejects a thin event by design, so
// verification goes through client.ParseAccountEvent instead. The two
// routes cannot be merged: one Stripe event destination carries one
// event_payload, thin or snapshot, never both.
//
// A thin notification carries no object, only a reference to what
// changed. So on a capability_status_updated event this fetches the
// account's current state from Stripe (client.RetrieveAccount) and
// persists it -- the event says something moved, Stripe's own copy says
// what it moved to. That also means a delivery that arrives out of order
// cannot write a stale status: whatever is fetched is current at the
// moment of the write.
//
// Everything else matches PostConnectWebhookHandler: no
// staffauth.Middleware (there is no session), event ids claimed in
// stripe_webhook_events so a retry is a no-op, and an unrecognized
// account or unhandled type logged and dropped with a 200 rather than
// erroring, because Stripe retries indefinitely on anything else.
func PostAccountWebhookHandler(db *sql.DB, client Client, webhookSecret string, mode stripemode.Mode, enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		event, ok := webhookarrival.OpenStripe(w, r, "payments: account webhook", mode,
			func(body []byte, sigHeader string) (AccountEvent, error) {
				return client.ParseAccountEvent(body, sigHeader, webhookSecret)
			},
			func(e AccountEvent) (string, bool) { return e.ID, e.Livemode })
		if !ok {
			return
		}

		if event.Type != eventTypeMerchantCapabilityStatusUpdated {
			// Stripe sends several other account events -- created,
			// [identity].updated, [defaults].updated -- none of which
			// change what the Payments screen reports.
			webhookarrival.Mark(r.Context(), webhookarrival.Dropped)
			log.Printf("payments: account webhook: dropping unhandled event type %q (id %s)", event.Type, event.ID)
			w.WriteHeader(http.StatusOK)
			return
		}

		handleCapabilityStatusUpdated(w, r, db, client, event, enq)
	})
}

// handleCapabilityStatusUpdated applies one
// capability_status_updated thin event: fetches the account's current
// capability statuses and outstanding requirements from Stripe, writes
// them onto the matching practices row along with the event id and time
// that caused the change, appends the same facts to
// stripe_connect_status_history (#1812), and queues #343's payout-incomplete
// Notification on the requirements_due empty -> non-empty transition
// (see 00034_payout_outbox.sql for the trigger/cadence reasoning).
func handleCapabilityStatusUpdated(w http.ResponseWriter, r *http.Request, db *sql.DB, client Client, event AccountEvent, enq tasknudge.Enqueuer) {
	status, err := client.RetrieveAccount(r.Context(), event.AccountID)
	if err != nil {
		// Unlike an unrecognized account, this is a genuine failure to
		// reach Stripe. A 500 is right: Stripe retries, and the next
		// delivery gets another chance to read the state. Nothing is
		// claimed in stripe_webhook_events yet, so that retry is not
		// swallowed as a replay.
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	tx, committed, ok := beginEvent(w, r, db, event.ID)
	if !ok {
		return
	}
	defer rollbackUnlessCommitted(tx, committed)

	// Locks the practices row for the rest of this transaction and reads
	// its pre-update requirements_due, so the empty -> non-empty episode
	// transition below can be judged against what was actually there a
	// moment ago rather than racing a concurrent delivery for the same
	// account. cardinality(), not a Scan into []string, for the same
	// driver reason requirementsStillOutstanding (outbox.go) avoids it.
	var practiceID string
	var oldRequirementsCount int
	err = tx.QueryRowContext(r.Context(),
		`SELECT id, cardinality(stripe_connect_requirements_due) FROM practices WHERE stripe_connect_account_id = $1 FOR UPDATE`,
		event.AccountID,
	).Scan(&practiceID, &oldRequirementsCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No practice has this connected account id -- logged and
			// dropped, the same rule the Invoice branches follow. The
			// stripe_webhook_events row inserted above still commits, so a
			// retried delivery of this same unrecognized event is also a
			// no-op rather than logging twice.
			webhookarrival.Mark(r.Context(), webhookarrival.Dropped)
			log.Printf("payments: account webhook: dropping capability status update for unrecognized account %q (event id %s)", event.AccountID, event.ID)
			commitAndAck(w, tx, committed)
			return
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	newRequirementsDue := requirementsOrEmpty(status.RequirementsDue)
	processedAt := clock.Now(r.Context())
	// Stripe always sends `created`; a notification without one would
	// otherwise be stored as year 1 and read as the earliest event of all.
	// Our own time is the closest truthful stand-in.
	eventCreatedAt := event.Created
	if eventCreatedAt.IsZero() {
		eventCreatedAt = processedAt
	}
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE practices SET
			stripe_connect_card_payments_status = $1,
			stripe_connect_payouts_status = $2,
			stripe_connect_requirements_due = $3,
			stripe_connect_status_event_id = $4,
			stripe_connect_status_updated_at = $5
		WHERE id = $6`,
		string(status.CardPayments), string(status.Payouts), newRequirementsDue, event.ID, processedAt, practiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	// #1812: practices keeps only the latest state, so each applied event
	// also leaves one append-only row -- on this same tx, so the history
	// and the row it explains cannot disagree. Only an applied event gets
	// here: the replay and unrecognized-account branches above have
	// already returned. See 00128_stripe_connect_status_history.sql for
	// why both Stripe's time and ours are kept.
	if _, err := tx.ExecContext(r.Context(),
		`INSERT INTO stripe_connect_status_history
			(practice_id, stripe_event_id, card_payments_status, payouts_status, requirements_due, stripe_event_created_at, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		practiceID, event.ID, string(status.CardPayments), string(status.Payouts), newRequirementsDue, eventCreatedAt, processedAt,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	queuedPayoutNotification := oldRequirementsCount == 0 && len(newRequirementsDue) > 0
	if queuedPayoutNotification {
		if err := QueuePayoutIncompleteNotification(r.Context(), tx, practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
	}

	commitAndAck(w, tx, committed)
	// ADR-0013: fired only when this call actually queued a payout_outbox
	// row and the commit that persisted it succeeded -- not on the
	// alreadyProcessed/unrecognized-account branches above, which also
	// call commitAndAck but queued nothing.
	if *committed && queuedPayoutNotification {
		tasknudge.Fire(enq, tasknudge.Payout)(r.Context())
	}
}
