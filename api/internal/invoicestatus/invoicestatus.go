// Package invoicestatus names an Invoice's stored lifecycle stages, the
// invoice_status enum of 00024_invoices.sql, and the two derived facts
// more than one package reads off an invoices row: whether it is still
// unsettled, and whether a payment on it is on its way. It imports
// nothing of this repo's own, so payments (which declares the moves
// between these stages, lifecycle.go) and the packages payments itself
// imports and so cannot import back -- client's erasure, practicedeletion,
// overdue -- all read one name instead of each restating the literals.
package invoicestatus

// Status is an Invoice's stored lifecycle stage. It mirrors Stripe's own
// Invoice statuses exactly (#82), on both rails: a by-hand Invoice is
// born open and never draft.
type Status string

// The five values of invoice_status.
const (
	// Draft is a Stripe-backed Invoice created on Stripe and not yet
	// finalized: it never reached the Client.
	Draft Status = "draft"
	// Open is billed and not yet collected.
	Open Status = "open"
	// Paid is collected. A refunded Invoice stays Paid, as Stripe's does
	// (00115_payment_refund.sql).
	Paid Status = "paid"
	// Uncollectible is written off: the Practice gave up collecting it.
	Uncollectible Status = "uncollectible"
	// Void is canceled: it is no longer owed and never was paid.
	Void Status = "void"
)

// All is every Status, in lifecycle order.
var All = []Status{Draft, Open, Paid, Uncollectible, Void}

// UnsettledExpr is whether an invoices row aliased i is still unsettled:
// Draft or Open, the two stages Stripe cannot redact and a Practice could
// still collect on. An unsettled Invoice refuses a Client's erasure and
// the Practice's deletion, so client's erasure eligibility and
// practicedeletion's precheck both read this one predicate.
const UnsettledExpr = `i.status IN ('draft', 'open')`

// PaymentOnItsWayExpr is whether a payment on an invoices row aliased i
// is on its way (#2225): payment_intent.processing set
// payment_processing_since, and only an open Invoice can carry it
// (invoices_processing_only_while_open, 00167). Every reader of the fact
// -- an Invoice list, the Client portal, the Clients list's rollup --
// reads this one expression.
const PaymentOnItsWayExpr = `(i.status = 'open' AND i.payment_processing_since IS NOT NULL)`
