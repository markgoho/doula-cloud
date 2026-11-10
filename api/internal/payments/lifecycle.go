package payments

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"doula-cloud/api/internal/invoicestatus"
)

// Mover is who may make an Invoice lifecycle move: a person acting in
// DoulaCloud, or Stripe reporting, by a verified Connect webhook event,
// what already happened in the Practice's own Stripe account.
type Mover string

// The three movers.
const (
	// MovedByStaff is a Staff member's request here -- including what
	// DoulaCloud does inside that request on her behalf, as when a raise
	// whose Stripe finalization failed leaves its draft voided.
	MovedByStaff Mover = "staff"
	// MovedByClient is the Client's own request in her portal.
	MovedByClient Mover = "client"
	// MovedByStripe is a Connect webhook event. Stripe is the record for a
	// Stripe-backed Invoice, so these moves accept every local stage
	// Stripe's own move could have started from, including a local
	// Draft whose finalization DoulaCloud never heard back about.
	MovedByStripe Mover = "stripe"
)

// InvoiceTransition is one named Invoice lifecycle move: the stages it
// may start from, the stage it leaves the Invoice in, which Invoices it
// applies to (by the rail each was raised on, billingModeOf), and who may
// make it. A move whose To is also one of its From stages changes no
// status -- recording a Refund, or a payment on its way -- and is
// declared here all the same, because its handler still asks this table
// whether the Invoice may take it.
//
// Before this table each move was an inline `status != "open"` beside a
// literal UPDATE, and the webhook's invoice.paid guard skipped only an
// Invoice already paid, so a void or draft one was moved to paid. Every
// route and every Connect webhook handler that moves an Invoice now asks
// its Transition's Check first, and writes t.To as the status rather than
// a literal (lifecycle_test.go's source scan holds both). The schema
// holds what each stage carries (00167_invoice_lifecycle_checks.sql).
type InvoiceTransition struct {
	// Name is the move in a few words, for a log line and a test.
	Name string
	// From is every stage the move may start from.
	From []invoicestatus.Status
	// To is the stage the move leaves the Invoice in.
	To invoicestatus.Status
	// Rails is which Invoices the move applies to.
	Rails []BillingMode
	// By is who may make the move.
	By Mover
	// WrongRail is the refusal a person reads when the Invoice is on a
	// rail the move does not apply to, and WrongStatus when it is at a
	// stage the move does not start from. Each is the sentence the route
	// has always written, word for word. A move only Stripe makes leaves
	// both empty: Check then words its refusal for the log line.
	WrongRail, WrongStatus string
}

// The two rails, as a Transition's Rails names them.
var (
	stripeRail = []BillingMode{BillingModeStripe}
	byHandRail = []BillingMode{BillingModeByHand}
	eitherRail = []BillingMode{BillingModeStripe, BillingModeByHand}
)

// TransitionFinalize is PostInvoiceHandler's Stripe rail once Stripe has
// finalized the draft it just created: the draft is now billed.
var TransitionFinalize = InvoiceTransition{
	Name: "finalize", From: []invoicestatus.Status{invoicestatus.Draft}, To: invoicestatus.Open,
	Rails: stripeRail, By: MovedByStaff,
}

// TransitionAbandonDraft is the same handler's other exit from Draft: when
// Stripe would not finalize the draft, the row is kept (DoulaCloud never
// loses track of an Invoice that exists on Stripe) and voided, recorded
// as activity.ActionInvoiceNotSent. A draft left as it was would refuse
// the Client's erasure and the Practice's deletion forever, since nothing
// else ever moves it. Stripe never emails or charges a draft, so voiding
// it here asks nothing of the Client.
var TransitionAbandonDraft = InvoiceTransition{
	Name: "abandon a draft Stripe would not finalize", From: []invoicestatus.Status{invoicestatus.Draft}, To: invoicestatus.Void,
	Rails: stripeRail, By: MovedByStaff,
}

// TransitionRecordPayment is PostManualPaymentHandler (#271): money that
// arrived outside Stripe, on either rail.
var TransitionRecordPayment = InvoiceTransition{
	Name: "record a payment", From: []invoicestatus.Status{invoicestatus.Open}, To: invoicestatus.Paid,
	Rails: eitherRail, By: MovedByStaff, WrongStatus: MsgInvoiceNotOpen,
}

// TransitionVoid is PostVoidInvoiceHandler (#271). A Stripe-backed Invoice
// is voided in the Stripe Dashboard instead, and reaches DoulaCloud as
// TransitionStripeVoided.
var TransitionVoid = InvoiceTransition{
	Name: "void", From: []invoicestatus.Status{invoicestatus.Open}, To: invoicestatus.Void,
	Rails: byHandRail, By: MovedByStaff,
	WrongRail: MsgStripeInvoiceCannotBeVoidedOrWrittenOff, WrongStatus: MsgInvoiceNotOpen,
}

// TransitionWriteOff is PostWriteOffInvoiceHandler (#271). A Stripe-backed
// Invoice is written off in the Stripe Dashboard and reaches DoulaCloud
// as TransitionStripeUncollectible.
var TransitionWriteOff = InvoiceTransition{
	Name: "write off", From: []invoicestatus.Status{invoicestatus.Open}, To: invoicestatus.Uncollectible,
	Rails: byHandRail, By: MovedByStaff,
	WrongRail: MsgStripeInvoiceCannotBeVoidedOrWrittenOff, WrongStatus: MsgInvoiceNotOpen,
}

// TransitionReversePayment is PostReversePaymentHandler (#945): a by-hand
// Payment that never really arrived, so the Invoice is owed again.
var TransitionReversePayment = InvoiceTransition{
	Name: "reverse a payment", From: []invoicestatus.Status{invoicestatus.Paid}, To: invoicestatus.Open,
	Rails: byHandRail, By: MovedByStaff,
	WrongRail: MsgStripeInvoiceCannotBeReversed, WrongStatus: MsgInvoiceNotPaid,
}

// TransitionRefund is PostRefundPaymentHandler's precondition (#1009): a
// Refund returns money against a paid Invoice and leaves it paid, as
// Stripe's own does.
var TransitionRefund = InvoiceTransition{
	Name: "refund a payment", From: []invoicestatus.Status{invoicestatus.Paid}, To: invoicestatus.Paid,
	Rails: eitherRail, By: MovedByStaff, WrongStatus: MsgInvoiceNotPaidForRefund,
}

// TransitionClientPay is ClientGetPaymentHandler's precondition (#1020):
// she may start paying an open Stripe-backed Invoice in her portal. What
// her payment then does to the Invoice reaches DoulaCloud as
// TransitionPaymentOnItsWay and TransitionStripePaid.
var TransitionClientPay = InvoiceTransition{
	Name: "pay in the portal", From: []invoicestatus.Status{invoicestatus.Open}, To: invoicestatus.Open,
	Rails: stripeRail, By: MovedByClient,
	WrongRail: msgClientInvoiceByHand, WrongStatus: msgClientInvoiceNotOpen,
}

// TransitionStripePaid is invoice.paid (#82). Stripe takes a payment on an
// open Invoice and, as Stripe allows, on one already marked uncollectible
// -- money that moved is recorded, never refused -- and on a local Draft
// whose finalization DoulaCloud never heard back about. A void Invoice is
// not a stage Stripe can collect on, so that event is refused, logged and
// acknowledged, and counted as refused on the Platform's webhook screen
// for a person to reconcile.
var TransitionStripePaid = InvoiceTransition{
	Name: "invoice.paid", From: []invoicestatus.Status{invoicestatus.Draft, invoicestatus.Open, invoicestatus.Uncollectible}, To: invoicestatus.Paid,
	Rails: stripeRail, By: MovedByStripe,
}

// TransitionStripeUncollectible is invoice.marked_uncollectible (#2224):
// the Practice wrote the Invoice off in her own Stripe account.
var TransitionStripeUncollectible = InvoiceTransition{
	Name: "invoice.marked_uncollectible", From: []invoicestatus.Status{invoicestatus.Draft, invoicestatus.Open}, To: invoicestatus.Uncollectible,
	Rails: stripeRail, By: MovedByStripe,
}

// TransitionStripeVoided is invoice.voided: the Practice voided the Invoice
// in her own Stripe account, which is where MsgStripeInvoiceCannotBeVoidedOrWrittenOff
// sends her to do it. Stripe voids an open Invoice and an uncollectible
// one, so both are declared. Before this, the event was never subscribed
// to, and an Invoice voided on Stripe stayed open here -- outstanding,
// overdue, and in the way of the Client's erasure and the Practice's
// deletion.
var TransitionStripeVoided = InvoiceTransition{
	Name: "invoice.voided", From: []invoicestatus.Status{invoicestatus.Draft, invoicestatus.Open, invoicestatus.Uncollectible}, To: invoicestatus.Void,
	Rails: stripeRail, By: MovedByStripe,
}

// TransitionStripeRefunded is credit_note.created and refund.created
// (#1009): the Practice returned money from her own Stripe account. As
// TransitionRefund, only a paid Invoice has money to return, and it stays
// paid. An Invoice that is not paid has no standing Payment
// (paymentstanding) for the Refund to be recorded against, so the two
// records already disagree; the event is refused, logged, acknowledged
// and counted as refused on the Platform's webhook screen for a person to
// reconcile -- takeStripeMove's rule -- rather than recorded against a
// Payment the Invoice no longer counts. Before this, the webhook path
// asked nothing of the Invoice while the Staff path refused it.
var TransitionStripeRefunded = InvoiceTransition{
	Name: "money returned on Stripe", From: []invoicestatus.Status{invoicestatus.Paid}, To: invoicestatus.Paid,
	Rails: stripeRail, By: MovedByStripe,
}

// TransitionPaymentOnItsWay is payment_intent.processing (#2225): a
// payment is on its way, which only an open Invoice can have. The status
// stays open, as Stripe keeps it.
var TransitionPaymentOnItsWay = InvoiceTransition{
	Name: "payment_intent.processing", From: []invoicestatus.Status{invoicestatus.Open}, To: invoicestatus.Open,
	Rails: stripeRail, By: MovedByStripe,
}

// InvoiceTransitions is every declared Invoice lifecycle move, walked by
// lifecycle_test.go.
var InvoiceTransitions = []InvoiceTransition{
	TransitionFinalize, TransitionAbandonDraft, TransitionRecordPayment, TransitionVoid, TransitionWriteOff,
	TransitionReversePayment, TransitionRefund, TransitionClientPay,
	TransitionStripePaid, TransitionStripeUncollectible, TransitionStripeVoided, TransitionPaymentOnItsWay,
	TransitionStripeRefunded,
}

// Check reports whether an Invoice at current, raised on rail, may take t
// (ok=true, no refusal), or, if not, the refusal: WrongRail when the rail
// is wrong, which is asked first, then WrongStatus when the stage is.
func (t InvoiceTransition) Check(current invoicestatus.Status, rail BillingMode) (ok bool, refusal string) {
	if !slices.Contains(t.Rails, rail) {
		return false, refusalOr(t.WrongRail, fmt.Sprintf("%s does not apply to a %s Invoice", t.Name, rail))
	}
	if !slices.Contains(t.From, current) {
		return false, refusalOr(t.WrongStatus, fmt.Sprintf("%s cannot move an Invoice that is %s", t.Name, current))
	}
	return true, ""
}

// Reached reports whether an Invoice at current is already where t leaves
// it, and t is a move that changes the status: a redelivered Stripe event
// whose move an earlier delivery already made, which is acknowledged
// with nothing applied rather than refused.
func (t InvoiceTransition) Reached(current invoicestatus.Status) bool {
	return current == t.To && !slices.Contains(t.From, t.To)
}

// refusalOr is the declared refusal when there is one, and fallback, for
// a log line, when the move is one only Stripe makes.
func refusalOr(declared, fallback string) string {
	if declared != "" {
		return declared
	}
	return fallback
}

// moveInvoice is the one statement in this package that writes an
// Invoice's status: it sets t.To on invoiceID, plus whatever else the
// move carries -- set is a fixed fragment written by the caller, never
// request text, starting with a comma and numbering its parameters from
// $3, and args are those parameters. The caller has asked t.Check
// already, with the refusal it writes. lifecycle_test.go holds this as
// the only "UPDATE invoices SET status" outside tests.
func moveInvoice(ctx context.Context, tx *sql.Tx, invoiceID string, t InvoiceTransition, set string, args ...any) error {
	//nolint:gosec // set is a fixed fragment written by this package's own callers, never request text; every value rides args as a bind parameter
	query := `UPDATE invoices SET status = $2` + set + ` WHERE id = $1`
	if _, err := tx.ExecContext(ctx, query, append([]any{invoiceID, string(t.To)}, args...)...); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("payments: %s: %w", t.Name, err)
	}
	return nil
}
