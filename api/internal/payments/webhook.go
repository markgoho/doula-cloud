package payments

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// maxWebhookBodyBytes bounds how much of a webhook request body
// PostConnectWebhookHandler will read -- mirrors billing's own bound on
// its (separate) webhook endpoint.
const maxWebhookBodyBytes = 1 << 20 // 1 MiB

// The two v1 snapshot event types the Connect endpoint handles: #82's
// invoice.paid / invoice.payment_failed. Any other event type is logged
// and dropped. WebhookEvent.Type is a plain string (the Client port
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
	eventTypeInvoicePaid          = string(stripe.EventTypeInvoicePaid)
	eventTypeInvoicePaymentFailed = string(stripe.EventTypeInvoicePaymentFailed)
)

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

// invoicePaymentFailedObject is the subset of an invoice.payment_failed
// event's Invoice object #82's handler needs -- just enough to resolve
// the matching invoices row; no Payment is ever created on this event.
type invoicePaymentFailedObject struct {
	ID string `json:"id"`
}

// PostConnectWebhookHandler receives Stripe's Connect *Invoice* events --
// a platform-level webhook endpoint, entirely separate from billing's
// Customer/Checkout webhook (#77), which is a different Stripe object
// type with its own signing secret. It verifies the Stripe-Signature
// header itself via client.VerifyWebhookSignature (no staffauth.Middleware
// in front of it -- there is no Practice session to authenticate), then
// dispatches on the event type: #82's invoice.paid/invoice.payment_failed
// resolve the matching invoices row (via its stored Stripe invoice id,
// scoped by the event's account field) and update its status --
// invoice.paid also creates exactly one payments row. An event for an
// unrecognized account id, an Invoice not found in Doula Cloud, or any
// event type other than these two, is logged and dropped rather than
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
func PostConnectWebhookHandler(db *sql.DB, client Client, webhookSecret string, enq tasknudge.Enqueuer) http.Handler {
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

		switch event.Type {
		case eventTypeInvoicePaid:
			handleInvoicePaid(w, r, db, client, event, enq)
		case eventTypeInvoicePaymentFailed:
			handleInvoicePaymentFailed(w, r, db, event)
		default:
			log.Printf("payments: connect webhook: dropping unhandled event type %q (id %s)", event.Type, event.ID)
			w.WriteHeader(http.StatusOK)
		}
	})
}

// claimEvent begins a transaction and records eventID in
// stripe_webhook_events, the shared idempotency prologue every event-type
// branch below runs before doing any type-specific work. alreadyProcessed
// reports whether this event id was already recorded by an earlier
// delivery -- the caller must still commit tx (a no-op commit) and
// acknowledge in that case, rather than re-applying anything.
func claimEvent(ctx context.Context, db *sql.DB, eventID string) (tx *sql.Tx, alreadyProcessed bool, err error) {
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		return nil, false, fmt.Errorf("payments: begin webhook event tx: %w", err)
	}
	result, err := tx.ExecContext(ctx,
		`INSERT INTO stripe_webhook_events (event_id) VALUES ($1) ON CONFLICT (event_id) DO NOTHING`, eventID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		_ = tx.Rollback()
		return nil, false, fmt.Errorf("payments: claim webhook event: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
		_ = tx.Rollback()
		return nil, false, fmt.Errorf("payments: check claimed webhook event rows: %w", err)
	}
	return tx, rows == 0, nil
}

// commitAndAck commits tx and, on success, marks *committed true (so the
// caller's deferred rollback becomes a no-op) and writes a 200. On a
// commit failure it writes 500 instead, leaving *committed false so the
// deferred rollback runs.
func commitAndAck(w http.ResponseWriter, tx *sql.Tx, committed *bool) {
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB commit failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}
	*committed = true
	w.WriteHeader(http.StatusOK)
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
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM practices WHERE stripe_connect_account_id = $1`, accountID,
	).Scan(&practiceID); err != nil {
		// coverage:ignore reason: the sql.ErrNoRows branch (unrecognized account) is exercised by unit tests; a non-ErrNoRows DB failure here is not
		return "", "", fmt.Errorf("payments: resolve practice for webhook event: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", "", fmt.Errorf("payments: scope webhook event tx to practice: %w", err)
	}

	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM invoices WHERE stripe_invoice_id = $1`, stripeInvoiceID,
	).Scan(&invoiceID); err != nil {
		// coverage:ignore reason: the sql.ErrNoRows branch (unknown invoice) is exercised by unit tests; a non-ErrNoRows DB failure here is not
		return "", "", fmt.Errorf("payments: resolve invoice for webhook event: %w", err)
	}
	return invoiceID, practiceID, nil
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
		return fmt.Errorf("payments: resolve engagement for invoice paid: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionInvoicePaid),
		Actor:       activity.ClientActor(clientID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: record invoice paid: %w", err)
	}
	return nil
}

// handleInvoicePaid applies #82's invoice.paid handling: resolves the
// matching invoices row, creates exactly one payments row recording the
// amount actually paid and when, and flips the Invoice's status to paid.
func handleInvoicePaid(w http.ResponseWriter, r *http.Request, db *sql.DB, client Client, event WebhookEvent, enq tasknudge.Enqueuer) {
	var inv invoicePaidObject
	if err := json.Unmarshal(event.Data, &inv); err != nil {
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	tx, alreadyProcessed, err := claimEvent(r.Context(), db, event.ID)
	if err != nil {
		// coverage:ignore reason: claimEvent's own failures are DB failures, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}
	committed := false
	defer func() {
		// coverage:ignore reason: only reached after a DB failure; every failure path below is itself coverage:ignore'd, so this rollback is never exercised by unit tests
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if alreadyProcessed {
		// Commit the no-op tx and acknowledge -- a replayed invoice.paid
		// event must not create a second payments row.
		commitAndAck(w, tx, &committed)
		return
	}

	invoiceID, practiceID, err := resolveInvoiceForEvent(r.Context(), tx, inv.ID, event.Account)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Unrecognized account or unknown Stripe invoice id -- logged
			// and dropped, per #82's ticket body, not treated as an
			// error.
			log.Printf("payments: connect webhook: dropping invoice.paid for unresolved invoice %q (account %q, event id %s)", inv.ID, event.Account, event.ID)
			commitAndAck(w, tx, &committed)
			return
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	paidAt := time.Unix(inv.StatusTransitions.PaidAt, 0).UTC()

	// The reference is what makes a payments row traceable back to Stripe,
	// and it is not in the event. A failure to fetch it must not lose the
	// payment: log it and store an empty reference rather than 500 and
	// have Stripe redeliver an event whose money already moved.
	reference, err := client.RetrieveInvoicePaymentReference(r.Context(), event.Account, inv.ID)
	if err != nil {
		log.Printf("payments: connect webhook: invoice.paid %s recorded without a Stripe payment reference: %v", inv.ID, err)
		reference = ""
	}

	var paymentID string
	if err := tx.QueryRowContext(r.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		invoiceID, reference, inv.AmountPaid, paidAt,
	).Scan(&paymentID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE invoices SET status = 'paid', paid_at = $1 WHERE id = $2`, paidAt, invoiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if err := recordInvoicePaid(r.Context(), tx, practiceID, invoiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	// Queue #344's "a Payment arrived" Platform Notification on the same
	// tx and after the payments row exists to satisfy payment_id's FK --
	// see 00035_payment_received_outbox.sql for why this needs no
	// rollback-surviving write the way QueueOutOfCreditsNotification does.
	if err := QueuePaymentReceivedNotification(r.Context(), tx, paymentID, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	commitAndAck(w, tx, &committed)
	// ADR-0013: fired only on this success path, gated on committed --
	// unlike the alreadyProcessed/dropped branches above (which also call
	// commitAndAck but queued no row), and unlike a failed commit, which
	// leaves the just-queued payment_received_outbox row rolled back too.
	if committed {
		tasknudge.Fire(enq, tasknudge.PaymentReceived)(r.Context())
	}
}

// handleInvoicePaymentFailed applies #82's invoice.payment_failed
// handling: resolves the matching invoices row and flips its status to
// uncollectible, per 00024_invoices.sql's documented open -> paid/
// uncollectible transitions. Creates no payments row.
func handleInvoicePaymentFailed(w http.ResponseWriter, r *http.Request, db *sql.DB, event WebhookEvent) {
	var inv invoicePaymentFailedObject
	if err := json.Unmarshal(event.Data, &inv); err != nil {
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	tx, alreadyProcessed, err := claimEvent(r.Context(), db, event.ID)
	if err != nil {
		// coverage:ignore reason: claimEvent's own failures are DB failures, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}
	committed := false
	defer func() {
		// coverage:ignore reason: only reached after a DB failure; every failure path below is itself coverage:ignore'd, so this rollback is never exercised by unit tests
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if alreadyProcessed {
		commitAndAck(w, tx, &committed)
		return
	}

	invoiceID, _, err := resolveInvoiceForEvent(r.Context(), tx, inv.ID, event.Account)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("payments: connect webhook: dropping invoice.payment_failed for unresolved invoice %q (account %q, event id %s)", inv.ID, event.Account, event.ID)
			commitAndAck(w, tx, &committed)
			return
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	if _, err := tx.ExecContext(r.Context(),
		`UPDATE invoices SET status = 'uncollectible' WHERE id = $1`, invoiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	commitAndAck(w, tx, &committed)
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
func PostAccountWebhookHandler(db *sql.DB, client Client, webhookSecret string, enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes))
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		event, err := client.ParseAccountEvent(body, r.Header.Get("Stripe-Signature"), webhookSecret)
		if err != nil {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}

		if event.Type != eventTypeMerchantCapabilityStatusUpdated {
			// Stripe sends several other account events -- created,
			// [identity].updated, [defaults].updated -- none of which
			// change what the Payments screen reports.
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
// that caused the change, and queues #343's payout-incomplete
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
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	tx, alreadyProcessed, err := claimEvent(r.Context(), db, event.ID)
	if err != nil {
		// coverage:ignore reason: claimEvent's own failures are DB failures, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}
	committed := false
	defer func() {
		// coverage:ignore reason: only reached after a DB failure; every failure path below is itself coverage:ignore'd, so this rollback is never exercised by unit tests
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if alreadyProcessed {
		commitAndAck(w, tx, &committed)
		return
	}

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
			log.Printf("payments: account webhook: dropping capability status update for unrecognized account %q (event id %s)", event.AccountID, event.ID)
			commitAndAck(w, tx, &committed)
			return
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	newRequirementsDue := requirementsOrEmpty(status.RequirementsDue)
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE practices SET
			stripe_connect_card_payments_status = $1,
			stripe_connect_payouts_status = $2,
			stripe_connect_requirements_due = $3,
			stripe_connect_status_event_id = $4,
			stripe_connect_status_updated_at = now()
		WHERE id = $5`,
		string(status.CardPayments), string(status.Payouts), newRequirementsDue, event.ID, practiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	queuedPayoutNotification := oldRequirementsCount == 0 && len(newRequirementsDue) > 0
	if queuedPayoutNotification {
		if err := QueuePayoutIncompleteNotification(r.Context(), tx, practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
	}

	commitAndAck(w, tx, &committed)
	// ADR-0013: fired only when this call actually queued a payout_outbox
	// row and the commit that persisted it succeeded -- not on the
	// alreadyProcessed/unrecognized-account branches above, which also
	// call commitAndAck but queued nothing.
	if committed && queuedPayoutNotification {
		tasknudge.Fire(enq, tasknudge.Payout)(r.Context())
	}
}
