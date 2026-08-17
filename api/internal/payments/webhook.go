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

	"doula-cloud/api/internal/staffauth"
)

// maxWebhookBodyBytes bounds how much of a webhook request body
// PostConnectWebhookHandler will read -- mirrors billing's own bound on
// its (separate) webhook endpoint.
const maxWebhookBodyBytes = 1 << 20 // 1 MiB

// The three Stripe Connect event types this endpoint handles: #80's
// account.updated, and #82's invoice.paid / invoice.payment_failed. Any
// other event type is logged and dropped. WebhookEvent.Type is a plain
// string (the Client port abstracts away *stripe.Event), so stripe-go's
// own typed constants are converted once here rather than hand-rolling
// the literals.
const (
	eventTypeAccountUpdated       = string(stripe.EventTypeAccountUpdated)
	eventTypeInvoicePaid          = string(stripe.EventTypeInvoicePaid)
	eventTypeInvoicePaymentFailed = string(stripe.EventTypeInvoicePaymentFailed)
)

// accountUpdatedObject is the subset of an account.updated event's Account
// object payments cares about -- the same three capabilities persisted on
// practices.stripe_connect_*.
type accountUpdatedObject struct {
	ChargesEnabled   bool `json:"charges_enabled"`
	PayoutsEnabled   bool `json:"payouts_enabled"`
	DetailsSubmitted bool `json:"details_submitted"`
}

// invoicePaidObject is the subset of an invoice.paid event's Invoice
// object #82's handler needs: the Stripe invoice id to resolve the
// matching invoices row, the amount actually paid (may differ from the
// Invoice's own amount_cents in principle, though v1 models no partial
// payment), a Stripe reference for the payment itself, and the Unix
// timestamp Stripe recorded the payment at.
type invoicePaidObject struct {
	ID                string `json:"id"`
	AmountPaid        int64  `json:"amount_paid"`
	PaymentIntentID   string `json:"payment_intent"`
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

// PostConnectWebhookHandler receives Stripe's Connect account events -- a
// new, platform-level webhook endpoint, entirely separate from billing's
// Customer/Checkout webhook (#77), which is a different Stripe object
// type with its own signing secret. It verifies the Stripe-Signature
// header itself via client.VerifyWebhookSignature (no staffauth.Middleware
// in front of it -- there is no Practice session to authenticate), then
// dispatches on the event type: #80's account.updated updates the three
// capability booleans on the matching practices row, and #82's
// invoice.paid/invoice.payment_failed resolve the matching invoices row
// (via its stored Stripe invoice id, scoped by the event's account field)
// and update its status -- invoice.paid also creates exactly one payments
// row. An event for an unrecognized account id, an Invoice not found in
// Doula Cloud, or any event type other than these three, is logged and
// dropped rather than treated as an error -- Stripe retries indefinitely
// on anything but a 2xx. Replays of the same Stripe event id are recorded
// in stripe_webhook_events (#77's idempotency table, reused here per
// #80's and #82's ticket bodies rather than a second table) and skipped,
// so a retried delivery never re-applies a status transition or creates a
// second payments row. db must be the same low-privilege app_runtime
// connection staffauth.Middleware uses.
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

		switch event.Type {
		case eventTypeAccountUpdated:
			handleAccountUpdated(w, r, db, event)
		case eventTypeInvoicePaid:
			handleInvoicePaid(w, r, db, event)
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
// account-resolution rule handleAccountUpdated uses) and, on a match,
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
// rather than error.
func resolveInvoiceForEvent(ctx context.Context, tx *sql.Tx, stripeInvoiceID, accountID string) (invoiceID string, err error) {
	var practiceID string
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

	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM invoices WHERE stripe_invoice_id = $1`, stripeInvoiceID,
	).Scan(&invoiceID); err != nil {
		// coverage:ignore reason: the sql.ErrNoRows branch (unknown invoice) is exercised by unit tests; a non-ErrNoRows DB failure here is not
		return "", fmt.Errorf("payments: resolve invoice for webhook event: %w", err)
	}
	return invoiceID, nil
}

// handleAccountUpdated applies #80's account.updated handling: on a
// recognized account, updates the three capability booleans on the
// matching practices row.
func handleAccountUpdated(w http.ResponseWriter, r *http.Request, db *sql.DB, event WebhookEvent) {
	var account accountUpdatedObject
	if err := json.Unmarshal(event.Data, &account); err != nil {
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
		// Commit the no-op tx and acknowledge, without re-applying the
		// status update.
		commitAndAck(w, tx, &committed)
		return
	}

	result, err := tx.ExecContext(r.Context(),
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
	rows, err := result.RowsAffected()
	if err != nil {
		// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		// No practice has this connected account id -- logged and
		// dropped, per #80's ticket body, not treated as an error. The
		// stripe_webhook_events row inserted above still commits, so a
		// retried delivery of this same unrecognized event is also a
		// no-op rather than logging twice.
		log.Printf("payments: connect webhook: dropping account.updated for unrecognized account %q (event id %s)", event.Account, event.ID)
	}

	commitAndAck(w, tx, &committed)
}

// handleInvoicePaid applies #82's invoice.paid handling: resolves the
// matching invoices row, creates exactly one payments row recording the
// amount actually paid and when, and flips the Invoice's status to paid.
func handleInvoicePaid(w http.ResponseWriter, r *http.Request, db *sql.DB, event WebhookEvent) {
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

	invoiceID, err := resolveInvoiceForEvent(r.Context(), tx, inv.ID, event.Account)
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

	if _, err := tx.ExecContext(r.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at) VALUES ($1, $2, $3, $4)`,
		invoiceID, inv.PaymentIntentID, inv.AmountPaid, paidAt,
	); err != nil {
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

	commitAndAck(w, tx, &committed)
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

	invoiceID, err := resolveInvoiceForEvent(r.Context(), tx, inv.ID, event.Account)
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
