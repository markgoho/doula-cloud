package payments

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// PaymentMethod is the closed set of ways a manually recorded Payment
// (#271) arrived. PaymentMethodOther requires a note -- see
// RecordPaymentRequest's own doc comment.
type PaymentMethod string

// The four values PaymentMethod takes.
const (
	PaymentMethodCheck        PaymentMethod = "check"
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"
	PaymentMethodCash         PaymentMethod = "cash"
	PaymentMethodOther        PaymentMethod = "other"
)

var validPaymentMethods = map[PaymentMethod]bool{
	PaymentMethodCheck:        true,
	PaymentMethodBankTransfer: true,
	PaymentMethodCash:         true,
	PaymentMethodOther:        true,
}

// paidOnLayout is the date-only form PaidOn is read in -- GOV.UK's "known
// date" pattern (#404: a chosen instant, not memorized, the same
// reasoning behind every other TextInput type="date" in this codebase),
// so the wire form matches what type="date" posts.
const paidOnLayout = "2006-01-02"

// MsgInvoiceNotOpen is the refusal for recording a Payment, voiding, or
// writing off an Invoice that is not currently 'open' -- a draft never
// reached anybody, a void one was cancelled, an uncollectible one was
// already written off, and an already-paid one would double-count.
const MsgInvoiceNotOpen = "This Invoice is not open, so nothing can be recorded or changed against it."

// MsgStripeInvoiceCannotBeVoidedOrWrittenOff refuses void/write-off
// against a Stripe-backed Invoice (#271): nothing else in the model moves
// a by-hand Invoice out of 'open' besides these two actions and a
// Payment, so only a by-hand Invoice needs the escape hatch. A
// Stripe-backed Invoice is cancelled or written off in Stripe's own
// Dashboard, which flows back by webhook.
const MsgStripeInvoiceCannotBeVoidedOrWrittenOff = "A Stripe-backed Invoice cannot be voided or written off here -- use the Stripe Dashboard."

// MsgStripePayOutOfBandFailed is what a caller sees when Stripe refuses
// the paid_out_of_band call -- #271's fail-closed rule: nothing is saved
// locally, and the real Stripe error is only logged, never echoed back
// (it may carry Stripe-internal detail unfit for a Staff-facing message).
const MsgStripePayOutOfBandFailed = "Stripe would not mark this Invoice paid, so nothing was recorded. Try again, or check this Practice's Stripe Dashboard."

// RecordPaymentRequest is the body of a POST to PostManualPaymentHandler.
// The amount is never supplied -- it is always the Invoice's own
// amount_cents (#271: partial payments stay out of the model).
type RecordPaymentRequest struct {
	Method PaymentMethod `json:"method"`
	// Note is optional for every method, and required when Method is
	// PaymentMethodOther.
	Note string `json:"note,omitempty"`
	// PaidOn is the date the recorder says the money arrived, in
	// paidOnLayout form. It may precede the Invoice's own creation (a
	// deposit that can arrive early) but never the future.
	PaidOn string `json:"paidOn"`
}

// PaymentView is a manually recorded Payment, as returned by
// PostManualPaymentHandler.
type PaymentView struct {
	ID          string    `json:"id"`
	InvoiceID   string    `json:"invoiceId"`
	AmountCents int64     `json:"amountCents"`
	Method      string    `json:"method"`
	Note        *string   `json:"note,omitempty"`
	PaidAt      time.Time `json:"paidAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

// resolveInvoiceForPractice locks :invoiceId's row FOR UPDATE within
// practiceID, returning its current status, amount, Stripe invoice id
// (invalid when by-hand), and the Engagement it hangs off of -- the
// shared prologue for recording a Payment, voiding, and writing off, all
// of which must see and hold the row's true current state against a
// concurrent webhook or a second request racing the same Invoice.
// sql.ErrNoRows when the Invoice does not exist, or is not this
// Practice's (RLS already scopes the read; the WHERE clause is app-layer
// redundancy on top of it, the same belt-and-braces every other handler
// in this package applies).
func resolveInvoiceForPractice(ctx context.Context, tx *sql.Tx, practiceID, invoiceID string) (status string, amountCents int64, stripeInvoiceID sql.NullString, engagementID string, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT i.status, i.amount_cents, i.stripe_invoice_id, e.id
		   FROM invoices i
		   JOIN contracts c ON c.id = i.contract_id
		   JOIN engagements e ON e.id = c.engagement_id
		  WHERE i.id = $1 AND i.practice_id = $2
		  FOR UPDATE OF i`,
		invoiceID, practiceID,
	).Scan(&status, &amountCents, &stripeInvoiceID, &engagementID)
	// coverage:ignore reason: the sql.ErrNoRows branch is exercised by unit tests; a non-ErrNoRows DB failure here is not
	if err != nil {
		return "", 0, sql.NullString{}, "", fmt.Errorf("payments: resolve invoice for practice: %w", err)
	}
	return status, amountCents, stripeInvoiceID, engagementID, nil
}

// PostManualPaymentHandler records a Payment that did not come through
// Stripe (#271) -- Owner and Admin only, matching ADR-0008's
// Contract-money read row. Accepted only against an 'open' Invoice, for
// its full amount; refused on draft, void, uncollectible, or an Invoice
// already paid. Against a Stripe-backed Invoice, Stripe's own invoice is
// marked paid_out_of_band before anything is written locally, and a
// Stripe refusal fails the whole record closed -- nothing is saved. Must
// be mounted behind staffauth.Middleware.
//
// Owner and Admin is declared at the mount, not checked here (#990,
// following #970's own move for Contract writes): the handler no longer
// calls staffauth.RequireOwnerOrAdmin, because a Doula is refused by the
// gate before this runs. Widening or narrowing this route means editing
// its ir.ReplayableGated role list in mount.go.
func PostManualPaymentHandler(client Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		invoiceID := r.PathValue("invoiceId")
		if !staffauth.ParseUUID(w, "invoice", invoiceID) {
			return
		}

		var req RecordPaymentRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if !validPaymentMethods[req.Method] {
			apierr.WriteError(w, `method must be "check", "bank_transfer", "cash", or "other"`, http.StatusBadRequest)
			return
		}
		if req.Method == PaymentMethodOther && req.Note == "" {
			apierr.WriteError(w, `note is required when method is "other"`, http.StatusBadRequest)
			return
		}
		paidOn, err := time.Parse(paidOnLayout, req.PaidOn)
		if err != nil {
			apierr.WriteError(w, "paidOn must be a date in YYYY-MM-DD form", http.StatusBadRequest)
			return
		}
		if paidOn.After(time.Now().UTC().Truncate(24 * time.Hour)) {
			apierr.WriteError(w, "paidOn cannot be in the future", http.StatusBadRequest)
			return
		}

		status, amountCents, stripeInvoiceID, engagementID, err := resolveInvoiceForPractice(r.Context(), tx, practiceID, invoiceID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "invoice not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if status != invoiceStatusOpen {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgInvoiceNotOpen, nil)
			return
		}

		if stripeInvoiceID.Valid {
			accountID, err := fetchConnectAccountID(r.Context(), tx, practiceID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if err := client.PayOutOfBand(r.Context(), accountID, stripeInvoiceID.String); err != nil {
				log.Printf("payments: record manual payment: stripe pay out of band failed for invoice %s: %v", stripeInvoiceID.String, err)
				apierr.Write(w, http.StatusBadGateway, apierr.CodeFailedPrecondition, MsgStripePayOutOfBandFailed, nil)
				return
			}
		}

		var note sql.NullString
		if req.Note != "" {
			note = sql.NullString{String: req.Note, Valid: true}
		}

		var paymentID string
		var createdAt time.Time
		if err := tx.QueryRowContext(r.Context(),
			`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method, note)
			 VALUES ($1, $2, $3, 'manual', $4, $5) RETURNING id, created_at`,
			invoiceID, amountCents, paidOn, string(req.Method), note,
		).Scan(&paymentID, &createdAt); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE invoices SET status = 'paid', paid_at = $1 WHERE id = $2`, paidOn, invoiceID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		diff, err := json.Marshal(map[string]any{"method": string(req.Method), "amountCents": amountCents})
		if err != nil {
			// coverage:ignore reason: a map of a string and an int64 always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionPaymentRecorded),
			Diff:        diff,
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		view := PaymentView{
			ID:          paymentID,
			InvoiceID:   invoiceID,
			AmountCents: amountCents,
			Method:      string(req.Method),
			PaidAt:      paidOn,
			CreatedAt:   createdAt,
		}
		if note.Valid {
			view.Note = &note.String
		}
		apierr.WriteJSON(w, http.StatusCreated, view)
	})
}

// InvoiceTransitionView is what void/write-off return: the Invoice's new
// status.
type InvoiceTransitionView struct {
	Status string `json:"status"`
}

// transitionByHandInvoice is PostVoidInvoiceHandler and
// PostWriteOffInvoiceHandler's shared body -- both are the same shape of
// act (an Owner or Admin moves a by-hand, still-open Invoice to a
// terminal status that is not 'paid') and differ only in the UPDATE they
// run and the Activity action they record. updateStatusQuery is the
// literal statement to run (matching this package's own convention of
// writing an enum value as a literal rather than a bound parameter,
// e.g. handleInvoicePaid's `SET status = 'paid'`) -- both callers pass a
// fixed, internal constant, never anything request-supplied.
//
// Owner and Admin is declared at each caller's own mount line, not
// checked here (#990, following #970's own move for Contract writes):
// this shared body no longer calls staffauth.RequireOwnerOrAdmin,
// because a Doula is refused by the gate before either PostVoidInvoiceHandler
// or PostWriteOffInvoiceHandler ever runs. Widening or narrowing either
// route means editing its own ir.ExemptGated role list in mount.go.
func transitionByHandInvoice(newStatus, updateStatusQuery string, action activity.EngagementAction) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		invoiceID := r.PathValue("invoiceId")
		if !staffauth.ParseUUID(w, "invoice", invoiceID) {
			return
		}

		status, _, stripeInvoiceID, engagementID, err := resolveInvoiceForPractice(r.Context(), tx, practiceID, invoiceID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "invoice not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if stripeInvoiceID.Valid {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgStripeInvoiceCannotBeVoidedOrWrittenOff, nil)
			return
		}
		if status != invoiceStatusOpen {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgInvoiceNotOpen, nil)
			return
		}

		if _, err := tx.ExecContext(r.Context(), updateStatusQuery, invoiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(action),
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, InvoiceTransitionView{Status: newStatus})
	})
}

// PostVoidInvoiceHandler moves a by-hand, still-open Invoice to 'void'
// (#271) -- for a mistyped amount or an Invoice raised in error, never a
// Payment reversal. Refused on a Stripe-backed Invoice (voided from
// Stripe's own Dashboard instead) or one that is not 'open'. Must be
// mounted behind staffauth.Middleware.
func PostVoidInvoiceHandler() http.Handler {
	return transitionByHandInvoice("void", `UPDATE invoices SET status = 'void' WHERE id = $1`, activity.ActionInvoiceVoided)
}

// PostWriteOffInvoiceHandler moves a by-hand, still-open Invoice to
// 'uncollectible' (#271) -- the Practice giving up on ever collecting it,
// never a Payment reversal. Refused on a Stripe-backed Invoice (reachable
// only via Stripe's own invoice.marked_uncollectible webhook) or one that
// is not 'open'. Must be mounted behind staffauth.Middleware.
func PostWriteOffInvoiceHandler() http.Handler {
	return transitionByHandInvoice("uncollectible", `UPDATE invoices SET status = 'uncollectible' WHERE id = $1`, activity.ActionInvoiceWrittenOff)
}
