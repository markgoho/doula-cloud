package payments

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"doula-cloud/api/internal/invoicestatus"
	"doula-cloud/api/internal/paymentstanding"
)

// PaymentAct is one act on a payments row, the row-level half of the
// lifecycle whose Invoice half is InvoiceTransitions: which Payments it
// may name (by kind), the standings it may start from, the standings it
// can leave the Payment in, and the row it writes. payments is
// append-only, so every act is an INSERT -- a new Payment, or a row
// pointing at one -- and a Payment's standing is derived from those rows
// by paymentstanding.Lateral, never stored.
//
// Each act is made beside an Invoice move, and asks both: the route
// checks its InvoiceTransition against the Invoice, then lockPaymentFor
// checks the PaymentAct against the Payment. 00175's trigger refuses, in
// the database, every row these declarations refuse.
//
// Who made an act, and when, is its activity row (payment_recorded,
// payment_reversed, payment_refunded, invoice_paid), as for every
// Invoice move (00167's comment). The payments row gains no actor column:
// a Payment Stripe collected has no Staff member behind it, and a second
// copy of the actor would be one more thing to keep in step.
type PaymentAct struct {
	// Name is the act in a few words, for a log line and a test.
	Name string
	// Kinds is which Payments the act may name. Empty for an act that
	// names none, because it records a new Payment.
	Kinds []string
	// From is every standing the act may start from. Empty for an act
	// that records a new Payment.
	From []paymentstanding.Standing
	// To is every standing the act can leave the Payment in -- or, for
	// recording one, the standing the new Payment is born in.
	To []paymentstanding.Standing
	// Writes is the payments kind of each row the act inserts.
	Writes []string
	// Refusals is, for each standing the act does not start from, the
	// sentence a person reads, word for word the one the route has always
	// written.
	Refusals map[paymentstanding.Standing]string
}

// ActRecordPayment is a Payment arriving: PostManualPaymentHandler
// (TransitionRecordPayment) and invoice.paid (TransitionStripePaid). The
// new Payment is born Live. An Invoice carries one standing Payment at a
// time, and its own transition (From open, or what Stripe may collect on)
// is what refuses a second.
var ActRecordPayment = PaymentAct{
	Name: "record a payment", To: []paymentstanding.Standing{paymentstanding.Live},
	Writes: []string{paymentstanding.KindManual, paymentstanding.KindStripe},
}

// ActReversePayment is PostReversePaymentHandler (#945), beside
// TransitionReversePayment: a by-hand Payment that never really arrived.
// Only a Live one: a reversal says the money never came, which cannot be
// said of money since sent back to the Client
// (MsgRefundedPaymentCannotBeReversed). A card Payment is never reversed
// (Stripe has no undo for it, #945's issue comment), so it is not a kind
// this act names.
var ActReversePayment = PaymentAct{
	Name: "reverse a payment", Kinds: []string{paymentstanding.KindManual},
	From:   []paymentstanding.Standing{paymentstanding.Live},
	To:     []paymentstanding.Standing{paymentstanding.Reversed},
	Writes: []string{paymentstanding.KindReversal},
	Refusals: map[paymentstanding.Standing]string{
		paymentstanding.Reversed:       MsgPaymentAlreadyReversed,
		paymentstanding.PartlyRefunded: MsgRefundedPaymentCannotBeReversed,
		paymentstanding.FullyRefunded:  MsgRefundedPaymentCannotBeReversed,
	},
}

// ActRefundPayment is PostRefundPaymentHandler (#1009), beside
// TransitionRefund, and the Connect webhook's credit_note.created and
// refund.created, beside TransitionStripeRefunded: money that arrived
// going back. Any Payment with money left on it, by hand or by card. The
// amount is bounded too, by what is left (refundable).
var ActRefundPayment = PaymentAct{
	Name: "refund a payment", Kinds: []string{paymentstanding.KindManual, paymentstanding.KindStripe},
	From:   []paymentstanding.Standing{paymentstanding.Live, paymentstanding.PartlyRefunded},
	To:     []paymentstanding.Standing{paymentstanding.PartlyRefunded, paymentstanding.FullyRefunded},
	Writes: []string{paymentstanding.KindRefund},
	Refusals: map[paymentstanding.Standing]string{
		paymentstanding.Reversed:      MsgReversedPaymentCannotBeRefunded,
		paymentstanding.FullyRefunded: MsgRefundExceedsPayment,
	},
}

// PaymentActs is every declared act on a payments row, walked by
// payment_acts_test.go.
var PaymentActs = []PaymentAct{ActRecordPayment, ActReversePayment, ActRefundPayment}

// Check reports whether a Payment of kind, at standing, may take a (ok,
// no refusal), or the refusal for the standing it is in. The kind is
// asked by lockPaymentFor, which treats a Payment of another kind as one
// that is not there.
func (a PaymentAct) Check(standing paymentstanding.Standing) (ok bool, refusal string) {
	if slices.Contains(a.From, standing) {
		return true, ""
	}
	return false, refusalOr(a.Refusals[standing], fmt.Sprintf("%s cannot start from a Payment that is %s", a.Name, standing))
}

// admits reports whether a Payment of kind, at standing, may take a.
func (a PaymentAct) admits(kind string, standing paymentstanding.Standing) bool {
	ok, _ := a.Check(standing)
	return ok && slices.Contains(a.Kinds, kind)
}

// paymentActsOpen is what a Staff member may still do with the Payment
// standing on an Invoice at status on rail -- of kind, at standing, with
// left cents not yet returned -- asked of the same two declarations each
// route asks: the Invoice's transition and the Payment's act. It is what
// InvoiceView.RefundableCents and Reversible carry, so a screen offers an
// act exactly when its route would take it. kind is empty when no
// Payment stands.
func paymentActsOpen(status invoicestatus.Status, rail BillingMode, kind string, standing paymentstanding.Standing, left int64) (refundableCents int64, reversible bool) {
	if ok, _ := TransitionRefund.Check(status, rail); ok && ActRefundPayment.admits(kind, standing) {
		refundableCents = left
	}
	if ok, _ := TransitionReversePayment.Check(status, rail); ok {
		reversible = ActReversePayment.admits(kind, standing)
	}
	return refundableCents, reversible
}

// paymentActRefusedError is lockPaymentFor's refusal: the Payment is there,
// and the act does not start from where it stands. message is the
// declared sentence.
type paymentActRefusedError struct{ message string }

func (e paymentActRefusedError) Error() string { return "payments: " + e.message }

// lockedPayment is the Payment an act names, as read under its row lock.
type lockedPayment struct {
	kind          string
	amountCents   int64
	standing      paymentstanding.Standing
	refundedCents int64
}

// refundable is how much is left to return on the Payment.
func (p lockedPayment) refundable() int64 { return p.amountCents - p.refundedCents }

// lockPaymentFor locks :paymentId's row FOR UPDATE, scoped to :invoiceId
// and to a Payment (not a row pointing at one), then derives its standing
// under that lock and asks a whether it may take it. Two acts on one
// Payment therefore serialize here, and the second reads the first's
// row. sql.ErrNoRows when there is no such Payment on this Invoice, or it
// is of a kind a does not name; paymentActRefusedError when it stands where a
// does not start.
func lockPaymentFor(ctx context.Context, tx *sql.Tx, a PaymentAct, invoiceID, paymentID string) (lockedPayment, error) {
	var target lockedPayment
	err := tx.QueryRowContext(ctx,
		`SELECT p.kind::text, p.amount_cents FROM payments p
		  WHERE p.id = $1 AND p.invoice_id = $2 AND `+paymentstanding.ArrivedExpr+`
		  FOR UPDATE`,
		paymentID, invoiceID,
	).Scan(&target.kind, &target.amountCents)
	// coverage:ignore reason: the sql.ErrNoRows branch is exercised by unit tests; a non-ErrNoRows DB failure here is not
	if err != nil {
		return lockedPayment{}, fmt.Errorf("payments: %s: lock payment: %w", a.Name, err)
	}
	if !slices.Contains(a.Kinds, target.kind) {
		return lockedPayment{}, fmt.Errorf("payments: %s: a %s payment: %w", a.Name, target.kind, sql.ErrNoRows)
	}
	target.standing, target.refundedCents, err = paymentstanding.Read(ctx, tx, paymentID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- the row was locked a statement ago
		return lockedPayment{}, fmt.Errorf("payments: %s: %w", a.Name, err)
	}
	if ok, refusal := a.Check(target.standing); !ok {
		return lockedPayment{}, paymentActRefusedError{message: refusal}
	}
	return target, nil
}
