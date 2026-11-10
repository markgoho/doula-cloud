package payments

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/paymentstanding"
	"doula-cloud/api/internal/staffauth"
)

// The payments.kind values a PaymentView reports, paymentstanding's names
// for the schema's enum (00095, 00103, 00115).
const (
	paymentKindManual   = paymentstanding.KindManual
	paymentKindStripe   = paymentstanding.KindStripe
	paymentKindReversal = paymentstanding.KindReversal
	paymentKindRefund   = paymentstanding.KindRefund
)

// MsgInvoiceNotPaidForRefund refuses a Refund against an Invoice that is
// not 'paid' (#1009): nothing was settled, so nothing can go back.
// Separate from MsgInvoiceNotPaid, whose wording names reversing.
const MsgInvoiceNotPaidForRefund = "This Invoice is not paid, so there is no money to return."

// MsgPaymentNotRefundable refuses a Refund target that is not a Payment
// belonging to :invoiceId -- a Payment this Invoice never had, or a row
// that is itself a reversal or a Refund. A Refund names the money that
// arrived, never another record of money moving.
const MsgPaymentNotRefundable = "This Payment cannot be refunded."

// MsgReversedPaymentCannotBeRefunded refuses a Refund against a Payment
// that has been reversed (#945): a reversal says that money never
// arrived, so there is none to return.
const MsgReversedPaymentCannotBeRefunded = "This Payment was reversed, so there is no money to return against it."

// MsgRefundExceedsPayment refuses a Refund larger than what its target
// Payment still covers after every earlier Refund against it.
const MsgRefundExceedsPayment = "That is more than is left to return on this Payment."

// MsgStripeRefundFailed is what a caller sees when Stripe refuses the
// credit note -- the fail-closed rule MsgStripePayOutOfBandFailed already
// follows: nothing is saved locally, and the real Stripe error is only
// logged, never echoed back.
const MsgStripeRefundFailed = "Stripe would not issue this refund, so nothing was recorded. Try again, or check this Practice's Stripe Dashboard."

// RefundPaymentRequest is the body of a POST to PostRefundPaymentHandler.
// Unlike recording a Payment, the amount is supplied: a Practice that
// keeps a canceled Engagement's first two visits returns the rest, and a
// partial Refund is the same shape as a full one.
type RefundPaymentRequest struct {
	AmountCents int64 `json:"amountCents"`
	// Method is how the Practice sent the money back. Required when the
	// Payment being returned was recorded by hand, whichever rail its
	// Invoice is on -- returning it is a by-hand act too. Refused when
	// the Payment was a card Payment Stripe collected: Stripe sends that
	// money back itself, and there is no method of the Practice's to name.
	Method PaymentMethod `json:"method,omitempty"`
	// Note is optional, and required when Method is PaymentMethodOther --
	// RecordPaymentRequest's own rule.
	Note string `json:"note,omitempty"`
}

// resolveRefundTarget is lockPaymentFor for ActRefundPayment, plus the
// act's amount bound: a Refund of more than is left to return on the
// Payment is refused with MsgRefundExceedsPayment. Under the Payment's
// row lock, so two concurrent Refunds against one Payment serialize and
// the second reads the first's row; 00175's trigger holds the same bound
// in the database.
func resolveRefundTarget(ctx context.Context, tx *sql.Tx, invoiceID, paymentID string, amountCents int64) (lockedPayment, error) {
	target, err := lockPaymentFor(ctx, tx, ActRefundPayment, invoiceID, paymentID)
	if err != nil {
		return lockedPayment{}, err
	}
	if amountCents > target.refundable() {
		return lockedPayment{}, paymentActRefusedError{message: MsgRefundExceedsPayment}
	}
	return target, nil
}

// validateRefundMethod applies RefundPaymentRequest.Method's rule once
// the target's kind is known. It returns false after writing the 400.
// The database cannot hold this rule itself -- whether a Refund names a
// method depends on another row's kind (00115's comment).
func validateRefundMethod(w http.ResponseWriter, req RefundPaymentRequest, targetKind string) bool {
	if targetKind == paymentKindStripe {
		if req.Method != "" {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
				"method must be empty when refunding a card payment",
				map[string]string{fieldMethod: "Stripe returns a card payment itself, so there is no method to choose"})
			return false
		}
		return true
	}
	if !validPaymentMethods[req.Method] {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
			`method must be "check", "bank_transfer", "cash", or "other"`,
			map[string]string{fieldMethod: "Select how the money was returned"})
		return false
	}
	if req.Method == PaymentMethodOther && req.Note == "" {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
			`note is required when method is "other"`,
			map[string]string{"note": `Enter a note for "Other"`})
		return false
	}
	return true
}

// PostRefundPaymentHandler returns money a Client genuinely paid (#1009):
// an additive row in the append-only payments table -- never an UPDATE or
// DELETE on the Payment it returns -- with a negative amount and a
// pointer to that Payment. The Invoice stays 'paid': the money arrived
// and the bill was settled, and Stripe's own Invoice does exactly the
// same (00115's comment, from the Sandbox walk on #1009's issue). This is
// the opposite fact from PostReversePaymentHandler's, which says the
// money never arrived and returns the Invoice to 'open'.
//
// On a Stripe-backed Invoice, Stripe is called before anything is
// written, and a refusal fails the whole Refund closed -- the same order
// PostManualPaymentHandler follows for paid_out_of_band. The call is a
// credit note either way; which amount it carries follows the Payment
// being returned (see Client.IssueRefundCreditNote). On a by-hand
// Invoice there is no Stripe call at all.
//
// Holding the Invoice row lock across the Stripe call is deliberate, not
// incidental: the credit note fires credit_note.created (and, for a card,
// refund.created) back at the Connect webhook, and that handler locks the
// same Invoice row before looking for this Refund's reference. It
// therefore waits for this transaction to commit and finds the row --
// the same race handleInvoicePaid's already-paid guard closes for
// paid_out_of_band's own echo.
//
// Owner and Admin is declared at the mount, not checked here (#990's
// pattern, matching PostManualPaymentHandler and
// PostReversePaymentHandler). Must be mounted behind staffauth.Middleware.
func PostRefundPaymentHandler(client Client) http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		tx, practiceID, invoiceID, paymentID := c.Tx, c.PracticeID, c.ID("invoiceId"), c.ID("paymentId")

		var req RefundPaymentRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if req.AmountCents <= 0 {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
				"amountCents must be greater than zero",
				map[string]string{"amountCents": "Enter an amount to return greater than $0.00"})
			return
		}

		status, _, stripeInvoiceID, engagementID, ok := requireInvoiceAtPractice(w, r, tx, practiceID, invoiceID)
		if !ok {
			return
		}
		if ok, refusal := TransitionRefund.Check(status, railOf(stripeInvoiceID)); !ok {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, refusal, nil)
			return
		}

		target, err := resolveRefundTarget(r.Context(), tx, invoiceID, paymentID, req.AmountCents)
		var refused paymentActRefusedError
		switch {
		case errors.Is(err, sql.ErrNoRows):
			apierr.WriteError(w, MsgPaymentNotRefundable, http.StatusNotFound)
			return
		case errors.As(err, &refused):
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, refused.message, nil)
			return
		case err != nil:
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !validateRefundMethod(w, req, target.kind) {
			return
		}

		var stripeReference sql.NullString
		if stripeInvoiceID.Valid {
			accountID, err := fetchConnectAccountID(r.Context(), tx, practiceID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			outOfBand := target.kind == paymentKindManual
			reference, err := client.IssueRefundCreditNote(r.Context(), accountID, stripeInvoiceID.String, req.AmountCents, outOfBand)
			if err != nil {
				log.Printf("payments: refund payment: stripe credit note failed for invoice %s: %v", stripeInvoiceID.String, err)
				apierr.Write(w, http.StatusBadGateway, apierr.CodeFailedPrecondition, MsgStripeRefundFailed, nil)
				return
			}
			stripeReference = sql.NullString{String: reference, Valid: true}
		}

		var method, note sql.NullString
		if req.Method != "" {
			method = sql.NullString{String: string(req.Method), Valid: true}
		}
		if req.Note != "" {
			note = sql.NullString{String: req.Note, Valid: true}
		}

		refundedAt := clock.Now(r.Context()).UTC()
		var refundID string
		var createdAt time.Time
		if err := tx.QueryRowContext(r.Context(),
			`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, target_payment_id, method, note, stripe_payment_reference)
			 VALUES ($1, $2, $3, 'refund', $4, $5, $6, $7) RETURNING id, created_at`,
			invoiceID, -req.AmountCents, refundedAt, paymentID, method, note, stripeReference,
		).Scan(&refundID, &createdAt); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := recordRefundActivity(r.Context(), tx, practiceID, engagementID, paymentID, -req.AmountCents, method, activity.StaffActor(c.StaffID), ""); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		view := PaymentView{
			ID:              refundID,
			InvoiceID:       invoiceID,
			Kind:            paymentKindRefund,
			AmountCents:     -req.AmountCents,
			Method:          method.String,
			TargetPaymentID: &paymentID,
			PaidAt:          refundedAt,
			CreatedAt:       createdAt,
		}
		if note.Valid {
			view.Note = &note.String
		}
		apierr.WriteJSON(w, http.StatusCreated, view)
	}, staffauth.PathID("invoice", "invoiceId"), staffauth.PathID("payment", "paymentId"))
}

// recordRefundActivity writes a payment_refunded entry: who, when (the
// row's own created_at), against which Payment, and how much -- the
// audit trail #1009 asks for. method rides along when there is one, so
// the log answers "how did the money go back" too. The note never does:
// PostManualPaymentHandler's diff excludes a note the same way, so a
// personal-data field lives only in the one column the erasure sweep
// reaches (client.redactPaymentNotes).
func recordRefundActivity(ctx context.Context, tx *sql.Tx, practiceID, engagementID, targetPaymentID string, amountCents int64, method sql.NullString, actor activity.Actor, origin string) error {
	fields := map[string]any{diffKeyAmountCents: amountCents, "targetPaymentId": targetPaymentID}
	if method.Valid {
		fields[fieldMethod] = method.String
	}
	if origin != "" {
		fields["origin"] = origin
	}
	diff, err := json.Marshal(fields)
	if err != nil {
		// coverage:ignore reason: a map of strings and an int64 always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("payments: marshal refund activity: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionPaymentRefunded),
		Diff:        diff,
		Actor:       actor,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: record refund activity: %w", err)
	}
	return nil
}
