// Package paymentstanding names where a Payment stands, and where a Fee
// Payment stands, and holds the one SQL derivation of each that every
// reader and every guard reads.
//
// payments is append-only (00025, #945, #1009) and has no status column:
// a row recording money arriving -- kind 'manual' or 'stripe' -- is
// changed only by other rows pointing at it through target_payment_id. A
// 'reversal' says the money never arrived; 'refund' rows say how much of
// it went back. So a Payment's standing is derived, never stored, and it
// is derived here, once: Lateral. Before this package the derivation was
// retyped at five readers and four guards (an Invoice list, the
// Practice-wide list, the Client portal, the credit-note webhook, the
// reversal and refund guards), one of them in a variant of its own.
//
// attachment_fee_payments (00136) is the same shape with no refunds: a
// 'recorded' Fee Payment stands until a 'reversal' row targets it.
// FeeStandsExpr is that rule, read by feepayment and by offer's inbox.
//
// It imports nothing of this repo's own, so payments, feepayment and
// offer -- which payments' importers cannot import back -- all read one
// name. The schema half is 00175_payment_standing.sql: a trigger that
// refuses a row no declared act could write (a reversal of a refunded
// Payment, a Refund beyond what is left, a second standing Payment on one
// Invoice). The acts themselves, and what each requires, are declared
// where their routes live: payments' PaymentActs.
package paymentstanding

import (
	"context"
	"database/sql"
	"fmt"
)

// Standing is where a Payment (a 'manual' or 'stripe' payments row)
// stands, as Lateral derives it.
type Standing string

// The four standings, matching the labels Lateral's CASE writes.
const (
	// Live has nothing pointing at it: the money arrived and none of it
	// has gone back.
	Live Standing = "live"
	// PartlyRefunded has Refunds against it summing to less than it: some
	// of the money went back.
	PartlyRefunded Standing = "partly_refunded"
	// FullyRefunded has Refunds against it summing to all of it. It still
	// stands -- the money arrived and the bill was settled, so its Invoice
	// stays paid, as Stripe's does (00115) -- but there is nothing left to
	// return.
	FullyRefunded Standing = "fully_refunded"
	// Reversed has a reversal against it: the money never arrived (#945),
	// so it no longer covers its Invoice.
	Reversed Standing = "reversed"
)

// Standings is every standing, in the order a Payment can pass through
// them. Lateral's labels are held to it by paymentstanding_test.go.
var Standings = []Standing{Live, PartlyRefunded, FullyRefunded, Reversed}

// Stands reports whether the Payment still covers its Invoice: every
// standing but Reversed. An Invoice is paid exactly while one Payment on
// it stands.
func (s Standing) Stands() bool { return s != Reversed }

// The payment_kind values (00025, 00095, 00103, 00115).
const (
	KindManual   = "manual"
	KindStripe   = "stripe"
	KindReversal = "reversal"
	KindRefund   = "refund"
)

// ArrivedExpr is whether a payments row aliased p records money arriving
// -- a Payment, which has a standing -- rather than a reversal or Refund
// row pointing at one.
const ArrivedExpr = `p.kind IN ('manual', 'stripe')`

// RefundedSumExpr is how much the payments rows aliased pt it is
// aggregated over returned, as a positive number and 0 when there are
// none, for rows already narrowed to IsRefundExpr. A Refund's amount is
// negative (payments_amount_sign_matches_kind, 00115), so "how much of an
// Invoice is still covered" stays one SUM over its rows.
const RefundedSumExpr = `COALESCE(-SUM(pt.amount_cents), 0)`

// IsRefundExpr is whether a payments row aliased pt is a Refund.
const IsRefundExpr = `pt.kind = 'refund'`

// Lateral derives the standing of a payments row aliased p already in the
// FROM list, as alias ps: ps.standing (one of Standings, as text) and
// ps.refunded_cents (how much of it has gone back, positive, 0 for none).
// It is the one place the rows pointing at a Payment are read. The two
// lookups ride the partial indexes payments_one_reversal_per_payment
// (00115) and payments_refunds_by_target (00175).
//
// A reversal and a Refund never both point at one Payment -- 00175's
// trigger refuses either after the other -- so a Reversed Payment always
// has refunded_cents 0.
const Lateral = `CROSS JOIN LATERAL (
	SELECT CASE WHEN f.reversed THEN 'reversed'
	            WHEN f.refunded_cents = 0 THEN 'live'
	            WHEN f.refunded_cents < p.amount_cents THEN 'partly_refunded'
	            ELSE 'fully_refunded' END AS standing,
	       f.refunded_cents
	  FROM (SELECT EXISTS (SELECT 1 FROM payments pt WHERE pt.target_payment_id = p.id AND pt.kind = 'reversal') AS reversed,
	               (SELECT ` + RefundedSumExpr + ` FROM payments pt WHERE pt.target_payment_id = p.id AND ` + IsRefundExpr + `) AS refunded_cents) f
	) ps`

// InvoicePaymentJoin finds, for an invoices row aliased i, the Payment
// that stands on it, as alias ap: ap.id, ap.kind, ap.method, ap.note,
// ap.reference (stripe_payment_reference), ap.amount_cents, ap.standing
// and ap.refunded_cents, every column NULL when none stands. An Invoice
// carries at most one standing Payment (00175's trigger); ORDER BY ...
// LIMIT 1 only makes the LATERAL single-row by construction.
//
// Every Refund on an Invoice targets its standing Payment -- a Refund
// targets a Payment on its own Invoice that stands, and a refunded
// Payment cannot be reversed -- so ap.refunded_cents is also how much of
// the Invoice has gone back to the Client.
const InvoicePaymentJoin = `LEFT JOIN LATERAL (
	SELECT p.id, p.kind::text AS kind, p.method::text AS method, p.note, p.stripe_payment_reference AS reference,
	       p.amount_cents, ps.standing, ps.refunded_cents
	  FROM payments p ` + Lateral + `
	 WHERE p.invoice_id = i.id AND ` + ArrivedExpr + ` AND ps.standing <> 'reversed'
	 ORDER BY p.created_at DESC LIMIT 1) ap ON true`

// Queryer is what Read reads through: a *sql.Tx in every caller.
type Queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Read derives one Payment's standing and how much of it has gone back.
// The caller has already found, and locked, the row as a Payment
// (ArrivedExpr); sql.ErrNoRows otherwise.
func Read(ctx context.Context, q Queryer, paymentID string) (standing Standing, refundedCents int64, err error) {
	if err := q.QueryRowContext(ctx,
		`SELECT ps.standing, ps.refunded_cents FROM payments p `+Lateral+` WHERE p.id = $1 AND `+ArrivedExpr,
		paymentID,
	).Scan(&standing, &refundedCents); err != nil {
		return "", 0, fmt.Errorf("paymentstanding: read %s: %w", paymentID, err)
	}
	return standing, refundedCents, nil
}

// FeeReversedExpr is whether a Fee Payment aliased f has been reversed: a
// 'reversal' row targets it (attachment_fee_payments_one_reversal, 00136,
// allows one).
const FeeReversedExpr = `EXISTS (SELECT 1 FROM attachment_fee_payments rv WHERE rv.target_id = f.id AND rv.kind = 'reversal')`

// FeeStandsExpr is whether an attachment_fee_payments row aliased f is a
// Fee Payment that stands: recorded, and not reversed. An Attachment's fee
// is paid exactly while one stands on it (00175's trigger refuses a
// second).
const FeeStandsExpr = `(f.kind = 'recorded' AND NOT ` + FeeReversedExpr + `)`
