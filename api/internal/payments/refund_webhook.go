package payments

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

// The two v1 snapshot event types that report money going back to a
// Client (#1009), both observed in the Sandbox rather than read off the
// docs (#1009's issue comment):
//
//   - credit_note.created fires for every credit note, on either rail --
//     one this system issued through PostRefundPaymentHandler, or one a
//     Practice issued from the Invoice page of her own Stripe Dashboard.
//   - refund.created fires for every card Refund. A credit note carrying
//     refund_amount fires it alongside credit_note.created; a refund made
//     from the Dashboard's Payments page, against the charge rather than
//     the Invoice, fires only this -- no credit note exists, and Stripe's
//     own Invoice record never learns of it.
//
// Listening to both is what makes a Refund the Practice issued anywhere
// reach Doula Cloud's record. A card Refund therefore arrives twice, and
// refundReference is what lets the second arrival find the first's row.
const (
	eventTypeCreditNoteCreated = string(stripe.EventTypeCreditNoteCreated)
	eventTypeRefundCreated     = string(stripe.EventTypeRefundCreated)
)

// creditNoteObject is the subset of a credit_note.created event's object
// the handler needs. refunds[].refund is an id string in an event
// payload, never an expanded object.
type creditNoteObject struct {
	ID              string `json:"id"`
	Invoice         string `json:"invoice"`
	OutOfBandAmount int64  `json:"out_of_band_amount"`
	Refunds         []struct {
		AmountRefunded int64  `json:"amount_refunded"`
		Refund         string `json:"refund"`
	} `json:"refunds"`
}

// returned is how much of this credit note is money going back to the
// Client: what Stripe refunded plus what the Practice returned out of
// band. A credit note can also carry credit_amount -- a credit to the
// Customer's Stripe balance, which returns no money -- and that part is
// deliberately not counted; a credit note that is only that returns 0.
func (c creditNoteObject) returned() int64 {
	total := c.OutOfBandAmount
	for _, r := range c.Refunds {
		total += r.AmountRefunded
	}
	return total
}

func (c creditNoteObject) reference() string {
	ids := make([]string, 0, len(c.Refunds))
	for _, r := range c.Refunds {
		ids = append(ids, r.Refund)
	}
	return refundReference(c.ID, ids)
}

// refundObject is the subset of a refund.created event's Refund the
// handler needs. payment_intent is how it reaches a payments row: a card
// Payment's row stores that id as its stripe_payment_reference
// (RetrieveInvoicePaymentReference), and a Refund carries no Invoice id.
type refundObject struct {
	ID            string `json:"id"`
	Amount        int64  `json:"amount"`
	PaymentIntent string `json:"payment_intent"`
	Status        string `json:"status"`
}

// errRefundAlreadyRecorded signals that a Refund row already carries this
// event's reference -- the echo of a Refund PostRefundPaymentHandler just
// issued, or the second of a card Refund's two events.
var errRefundAlreadyRecorded = errors.New("payments: refund already recorded")

// recordStripeRefund is the shared body of both event handlers, run once
// each has resolved the Practice and found the Payment being returned.
// It locks the Invoice row first, then checks for the echo, and only
// then writes -- the order handleInvoicePaid's already-paid guard uses,
// for the same reason: PostRefundPaymentHandler holds this Invoice's row
// lock across its own Stripe call, so this handler waits here until that
// transaction commits, and then finds its Refund row.
//
// The Refund carries no method and no note: Stripe does not report how a
// Practice returned out-of-band money, and nobody in Doula Cloud wrote a
// note. Its actor is Doula Cloud, which is true in ADR-0022's own sense
// -- the product recorded it, with nobody here asking -- and the diff's
// origin says the Practice issued it from her Stripe Dashboard, which is
// what the Staff-facing sentence reads.
func recordStripeRefund(ctx context.Context, tx *sql.Tx, practiceID, invoiceID, targetPaymentID string, amountCents int64, reference string) error {
	_, _, _, engagementID, err := resolveInvoiceForPractice(ctx, tx, practiceID, invoiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- each caller already proved this row exists
		return err
	}
	var exists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM payments WHERE kind = 'refund' AND stripe_payment_reference = $1)`, reference,
	).Scan(&exists); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: check refund already recorded: %w", err)
	}
	if exists {
		return errRefundAlreadyRecorded
	}
	if _, err := resolveRefundTarget(ctx, tx, invoiceID, targetPaymentID, amountCents); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, stripe_payment_reference)
		 VALUES ($1, $2, $3, 'refund', $4, $5)`,
		invoiceID, -amountCents, clock.Now(ctx).UTC(), targetPaymentID, reference,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: insert stripe refund: %w", err)
	}
	return recordRefundActivity(ctx, tx, practiceID, engagementID, targetPaymentID, -amountCents, sql.NullString{}, activity.SystemActor(), activity.RefundOriginStripeDashboard)
}

// finishRefundEvent turns recordStripeRefund's outcome into the webhook's
// reply. Every outcome that is not a database failure is acknowledged
// with a 200 -- Stripe retries anything else indefinitely -- and the ones
// that recorded nothing say why in the log.
func finishRefundEvent(w http.ResponseWriter, tx *sql.Tx, committed *bool, event WebhookEvent, err error) {
	switch {
	case err == nil:
	case errors.Is(err, errRefundAlreadyRecorded):
		log.Printf("payments: connect webhook: %s already recorded, skipping (event id %s)", event.Type, event.ID)
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, errPaymentReversedForRefund), errors.Is(err, errRefundExceedsPayment):
		// Stripe returned money against a Payment Doula Cloud's record
		// cannot take it against. Stripe caps a refund at what it
		// collected, which is what the Payment row recorded, so this means
		// the two records already disagreed before this event arrived.
		// Logged loudly rather than written: the local invariant that a
		// Payment is never over-returned holds absolutely, and the log is
		// what an operator reconciles from.
		log.Printf("payments: connect webhook: %s cannot be recorded against Doula Cloud's Payment (event id %s): %v", event.Type, event.ID, err)
	default:
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	commitAndAck(w, tx, committed)
}

// handleCreditNoteCreated records a credit note that returned money as a
// Refund, unless it is the echo of one this system issued.
func handleCreditNoteCreated(w http.ResponseWriter, r *http.Request, db *sql.DB, event WebhookEvent) {
	var note creditNoteObject
	if err := json.Unmarshal(event.Data, &note); err != nil {
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if note.returned() == 0 {
		log.Printf("payments: connect webhook: credit note %s returned no money, not a Refund (event id %s)", note.ID, event.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	tx, alreadyProcessed, err := claimEvent(r.Context(), db, event.ID)
	if err != nil {
		// coverage:ignore reason: claimEvent's own failures are DB failures, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	committed := false
	defer func() {
		if !committed {
			// coverage:ignore reason: only reached after a DB failure, not exercised by unit tests
			_ = tx.Rollback()
		}
	}()
	if alreadyProcessed {
		commitAndAck(w, tx, &committed)
		return
	}

	invoiceID, practiceID, err := resolveInvoiceForEvent(r.Context(), tx, note.Invoice, event.Account)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("payments: connect webhook: dropping credit_note.created for unresolved invoice %q (account %q, event id %s)", note.Invoice, event.Account, event.ID)
		commitAndAck(w, tx, &committed)
		return
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}

	// A credit note names the Invoice, not the Payment; the Payment it
	// returns is the one covering that Invoice -- activePaymentIDSubquery's
	// own rule, and the one the Staff screen offers a Refund against.
	var targetPaymentID string
	err = tx.QueryRowContext(r.Context(),
		`SELECT p.id FROM payments p
		  WHERE p.invoice_id = $1 AND p.kind IN ('manual', 'stripe')
		    AND NOT EXISTS (SELECT 1 FROM payments r WHERE r.target_payment_id = p.id AND r.kind = 'reversal')
		  ORDER BY p.created_at DESC LIMIT 1`, invoiceID,
	).Scan(&targetPaymentID)
	if err == nil {
		err = recordStripeRefund(r.Context(), tx, practiceID, invoiceID, targetPaymentID, note.returned(), note.reference())
	}
	finishRefundEvent(w, tx, &committed, event, err)
}

// handleRefundCreated records a card Refund as a Refund row, unless its
// credit note (or PostRefundPaymentHandler) already did. Reached through
// the PaymentIntent, since a Refund carries no Invoice id.
func handleRefundCreated(w http.ResponseWriter, r *http.Request, db *sql.DB, event WebhookEvent) {
	var refund refundObject
	if err := json.Unmarshal(event.Data, &refund); err != nil {
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	// A failed or canceled Refund returned nothing. One that is created
	// pending and later fails is a new fact this model cannot hold yet --
	// #1410.
	if refund.Status == "failed" || refund.Status == "canceled" {
		log.Printf("payments: connect webhook: refund %s is %s, nothing returned (event id %s)", refund.ID, refund.Status, event.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	tx, alreadyProcessed, err := claimEvent(r.Context(), db, event.ID)
	if err != nil {
		// coverage:ignore reason: claimEvent's own failures are DB failures, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	committed := false
	defer func() {
		if !committed {
			// coverage:ignore reason: only reached after a DB failure, not exercised by unit tests
			_ = tx.Rollback()
		}
	}()
	if alreadyProcessed {
		commitAndAck(w, tx, &committed)
		return
	}

	practiceID, err := resolvePracticeForEvent(r.Context(), tx, event.Account)
	var invoiceID, targetPaymentID string
	if err == nil {
		err = tx.QueryRowContext(r.Context(),
			`SELECT id, invoice_id FROM payments WHERE kind = 'stripe' AND stripe_payment_reference = $1`, refund.PaymentIntent,
		).Scan(&targetPaymentID, &invoiceID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		// Not every Refund on a Practice's account is against an Invoice
		// Doula Cloud raised; one that is not has nothing to attach to.
		log.Printf("payments: connect webhook: dropping refund.created %s for unresolved payment %q (account %q, event id %s)", refund.ID, refund.PaymentIntent, event.Account, event.ID)
		commitAndAck(w, tx, &committed)
		return
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	finishRefundEvent(w, tx, &committed, event, recordStripeRefund(r.Context(), tx, practiceID, invoiceID, targetPaymentID, refund.Amount, refund.ID))
}
