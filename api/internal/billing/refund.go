package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RefundWindowYears is how long after a purchase a Practice can ask for
// the money back, matching docs/copy/support-page.md word for word.
//
// Three years is two clocks at once. Tax Law 1139 gives us three years to
// reclaim sales tax already remitted, so every refundable dollar of tax
// stays recoverable for the life of the promise; and APL 1315(1-b)
// escheats an unspent balance to New York at three years' dormancy, so a
// shorter window would mean refusing a Practice her own money and then
// handing the same money to the State (#390, amended on #439).
const RefundWindowYears = 3

var (
	// ErrNothingRefundable is returned when a Practice holds no Credit
	// that a refund can draw against: everything is spent, granted free
	// of charge, or older than the window.
	ErrNothingRefundable = errors.New("billing: no refundable credits")
	// ErrRefundExceedsLot is returned when more Credits are asked for
	// than the lot being drawn from still holds. A refund reverses one
	// payment, so it never spans two purchases -- see Refund.
	ErrRefundExceedsLot = errors.New("billing: refund exceeds the credits available on the purchase it draws against")
)

// RefundQuote is what a Practice could be given back right now: how many
// purchased Credits are unspent and inside the window, and what that is
// worth including the sales tax charged on them.
type RefundQuote struct {
	Credits     int   `json:"credits"`
	AmountCents int64 `json:"amountCents"`
	TaxCents    int64 `json:"taxCents"`
}

// RefundReceipt records one issued refund: what was returned, and the
// Stripe objects it moved on.
type RefundReceipt struct {
	Credits         int    `json:"credits"`
	AmountCents     int64  `json:"amountCents"`
	TaxCents        int64  `json:"taxCents"`
	StripeRefundID  string `json:"stripeRefundId"`
	PaymentIntentID string `json:"paymentIntentId"`
}

// refundByRequestKey returns the refund already issued under requestKey,
// if there is one. The PaymentIntent comes from the lot the refund drew
// against, since that is where the money went back to.
func refundByRequestKey(ctx context.Context, tx *sql.Tx, requestKey string) (RefundReceipt, bool, error) {
	var receipt RefundReceipt
	var quantity, taxCents int64
	err := tx.QueryRowContext(ctx,
		`SELECT -r.quantity, -r.tax_cents, r.stripe_refund_id, l.stripe_payment_intent_id,
		        -r.quantity * r.unit_price_cents - r.tax_cents
		 FROM credit_ledger r
		 JOIN credit_ledger l ON l.id = r.drawn_lot_id
		 WHERE r.refund_request_key = $1`, requestKey,
	).Scan(&quantity, &taxCents, &receipt.StripeRefundID, &receipt.PaymentIntentID, &receipt.AmountCents)
	if errors.Is(err, sql.ErrNoRows) {
		return RefundReceipt{}, false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RefundReceipt{}, false, fmt.Errorf("billing: look up refund by request key: %w", err)
	}
	receipt.Credits = int(quantity)
	receipt.TaxCents = taxCents
	return receipt, true, nil
}

// refundableLots is the FIFO-ordered subset of a Practice's open lots that
// a refund may draw from: purchases only, inside the window.
//
// Credits given free of charge are not refundable -- /support promises a
// refund on Credits a Practice "has purchased", and a grant is not one.
// They are still spent first, because openLots is FIFO and a grant is
// always older, so a Practice's own money is the last thing consumed.
func refundableLots(lots []lot, now time.Time) []lot {
	refundable := []lot{}
	for _, l := range lots {
		if l.unitPriceCents <= 0 || !l.stripePaymentIntentID.Valid {
			continue
		}
		if now.After(l.createdAt.AddDate(RefundWindowYears, 0, 0)) {
			continue
		}
		refundable = append(refundable, l)
	}
	return refundable
}

// quote totals what the refundable lots are worth, tax included. Each
// lot's tax share is computed the same cumulative way an actual refund
// computes it, so the quote and the refunds that follow it agree to the
// cent even when a lot has already been partly refunded.
func quote(lots []lot) RefundQuote {
	q := RefundQuote{}
	for _, l := range lots {
		q.Credits += l.remaining
		q.AmountCents += int64(l.remaining) * l.unitPriceCents
		q.TaxCents += taxOnCredits(l.taxCents, l.quantity, l.refundedCount, l.remaining, l.taxRefundedCents)
	}
	return q
}

// Refundable is what a Practice could get back today, computed from the
// ledger alone -- no call to Stripe, no reconstruction from a Customer's
// payment history. A Practice that bought at two different prices and
// spent some of what she bought gets the arithmetic that actually
// applies: the oldest Credits are the spent ones, so the unspent balance
// is priced at what the later purchases cost.
func Refundable(ctx context.Context, tx *sql.Tx, practiceID string, now time.Time) (RefundQuote, error) {
	lots, err := openLots(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RefundQuote{}, err
	}
	return quote(refundableLots(lots, now)), nil
}

// Refund gives quantity Credits back to a Practice, at the price she paid
// for them and with the sales tax charged on them, against the payment
// they arrived on.
//
// It draws from **one** lot: the oldest purchase that is unspent and
// inside the window. A refund is a reversal of a payment, and two
// purchases are two payments -- issuing one refund across both would mean
// two Stripe calls that cannot both be undone if the second fails. A
// Practice wanting more than one purchase back asks twice, and each
// answer stands on its own.
//
// Issuing it against the original PaymentIntent is what makes Stripe Tax
// reverse the tax it reported; a refund moved any other way leaves the
// ST-100 overstating what New York is owed.
//
// The Stripe call runs before the ledger row is written, inside the
// caller's transaction: money moves first, then the record of it. The
// reverse order would leave a ledger claiming a refund that Stripe
// refused.
func Refund(ctx context.Context, tx *sql.Tx, client StripeClient, practiceID, requestKey string, quantity int, now time.Time) (RefundReceipt, error) {
	if quantity < 1 {
		return RefundReceipt{}, fmt.Errorf("billing: refund quantity must be at least 1")
	}

	// Answered from the row the first attempt wrote, if there was one.
	// This is what makes a retry safe rather than merely rare: the
	// operator reruns the same request, and gets the refund already
	// made instead of a second one.
	if receipt, found, err := refundByRequestKey(ctx, tx, requestKey); err != nil || found {
		return receipt, err
	}

	// Same lock ConsumeCredit takes, for the same reason: two refunds
	// arriving together must not both read the same lot as unspent.
	if _, err := tx.ExecContext(ctx, `SELECT id FROM practices WHERE id = $1 FOR UPDATE`, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RefundReceipt{}, fmt.Errorf("billing: lock practice: %w", err)
	}

	lots, err := openLots(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RefundReceipt{}, err
	}
	refundable := refundableLots(lots, now)
	if len(refundable) == 0 {
		return RefundReceipt{}, ErrNothingRefundable
	}
	l := refundable[0]
	if quantity > l.remaining {
		return RefundReceipt{}, ErrRefundExceedsLot
	}

	taxCents := taxOnCredits(l.taxCents, l.quantity, l.refundedCount, quantity, l.taxRefundedCents)
	amountCents := int64(quantity)*l.unitPriceCents + taxCents

	// requestKey rides through to Stripe as its idempotency key too, so
	// a retry that raced past the lookup above -- the first attempt's
	// transaction not yet committed -- replays the same Refund rather
	// than moving the money twice.
	refundID, err := client.RefundPayment(ctx, l.stripePaymentIntentID.String, requestKey, amountCents)
	if err != nil {
		return RefundReceipt{}, fmt.Errorf("billing: refund %d cents to practice %s: %w", amountCents, practiceID, err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO credit_ledger
		     (practice_id, origin, quantity, unit_price_cents, tax_cents, stripe_refund_id, drawn_lot_id, refund_request_key)
		 VALUES ($1, 'refund', $2, $3, $4, $5, $6, $7)`,
		practiceID, -quantity, l.unitPriceCents, -taxCents, refundID, l.id, requestKey,
	); err != nil {
		// Two attempts at one request, racing: 00054 makes both the
		// request key and the Stripe Refund unique, so the second is
		// refused here rather than recorded twice.
		return RefundReceipt{}, fmt.Errorf("billing: insert refund row: %w", err)
	}

	return RefundReceipt{
		Credits:         quantity,
		AmountCents:     amountCents,
		TaxCents:        taxCents,
		StripeRefundID:  refundID,
		PaymentIntentID: l.stripePaymentIntentID.String,
	}, nil
}
