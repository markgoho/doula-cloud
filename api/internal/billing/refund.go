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

// RefundQuote is what one refund would give back right now: Credits
// from the oldest purchase still unspent and inside the window, what
// that is worth including the sales tax charged on them, and the most
// Credits that purchase can give back.
type RefundQuote struct {
	Credits         int    `json:"credits"`
	AmountCents     int64  `json:"amountCents"`
	TaxCents        int64  `json:"taxCents"`
	MaxCredits      int    `json:"maxCredits"`
	PaymentIntentID string `json:"-"`
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

// RefundOrder is one refund asked for: the Practice, how many Credits,
// and the request key that names it, so a retry is the same refund.
type RefundOrder struct {
	PracticeID string
	Credits    int
	RequestKey string
}

// RecordAct writes the operator_acts row a refund is taken under, once
// its quote is known and before any money moves, and returns its id for
// the refund's ledger row.
type RecordAct func(ctx context.Context, quote RefundQuote) (operatorActID string, err error)

// refundByRequestKey returns the refund already issued under requestKey
// at the Practice, if there is one.
func refundByRequestKey(ctx context.Context, tx *sql.Tx, practiceID, requestKey string) (RefundReceipt, bool, error) {
	var receipt RefundReceipt
	err := tx.QueryRowContext(ctx,
		`SELECT credits, amount_cents, tax_cents, stripe_refund_id, payment_intent_id
		   FROM ops_refund_by_request_key($1, $2)`, practiceID, requestKey,
	).Scan(&receipt.Credits, &receipt.AmountCents, &receipt.TaxCents, &receipt.StripeRefundID, &receipt.PaymentIntentID)
	if errors.Is(err, sql.ErrNoRows) {
		return RefundReceipt{}, false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RefundReceipt{}, false, fmt.Errorf("billing: look up refund by request key: %w", err)
	}
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

// planRefund is the lot a refund of credits draws from and what it gives
// back. credits 0 asks for everything that lot still holds.
//
// It draws from **one** lot: the oldest purchase that is unspent and
// inside the window. A refund is a reversal of a payment, and two
// purchases are two payments -- issuing one refund across both would mean
// two Stripe calls that cannot both be undone if the second fails. A
// Practice wanting more than one purchase back asks twice, and each
// answer stands on its own.
//
// Each tax share is computed cumulatively against the lot, so a run of
// partial refunds returns exactly the tax the lot was charged.
func planRefund(ctx context.Context, tx *sql.Tx, practiceID string, credits int, now time.Time) (lot, RefundQuote, error) {
	lots, err := refundLots(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return lot{}, RefundQuote{}, err
	}
	refundable := refundableLots(lots, now)
	if len(refundable) == 0 {
		return lot{}, RefundQuote{}, ErrNothingRefundable
	}
	l := refundable[0]
	if credits == 0 {
		credits = l.remaining
	}
	if credits > l.remaining {
		return lot{}, RefundQuote{}, ErrRefundExceedsLot
	}
	taxCents := taxOnCredits(l.taxCents, l.quantity, l.refundedCount, credits, l.taxRefundedCents)
	return l, RefundQuote{
		Credits:         credits,
		AmountCents:     int64(credits)*l.unitPriceCents + taxCents,
		TaxCents:        taxCents,
		MaxCredits:      l.remaining,
		PaymentIntentID: l.stripePaymentIntentID.String,
	}, nil
}

// QuoteRefund is what a refund of credits would give back today,
// computed from the ledger alone, with no call to Stripe. credits 0 asks
// for everything the oldest refundable purchase still holds.
func QuoteRefund(ctx context.Context, tx *sql.Tx, practiceID string, credits int, now time.Time) (RefundQuote, error) {
	_, q, err := planRefund(ctx, tx, practiceID, credits, now)
	return q, err
}

// Refund gives order.Credits back to a Practice, at the price she paid
// for them and with the sales tax charged on them, against the payment
// they arrived on, on an ops_runtime transaction (ADR-0062).
//
// Issuing it against the original PaymentIntent is what makes Stripe Tax
// reverse the tax it reported; a refund moved any other way leaves the
// ST-100 overstating what New York is owed.
//
// recordAct writes the act before the Stripe call, and the ledger row
// keys it after: money moves, then the record of it, all in the caller's
// transaction, so a refusal from Stripe leaves neither.
func Refund(ctx context.Context, tx *sql.Tx, client StripeClient, order RefundOrder, now time.Time, recordAct RecordAct) (RefundReceipt, error) {
	if order.Credits < 1 {
		return RefundReceipt{}, fmt.Errorf("billing: refund quantity must be at least 1")
	}

	// A retry under the same key gets the refund already made instead of
	// a second one.
	if receipt, found, err := refundByRequestKey(ctx, tx, order.PracticeID, order.RequestKey); err != nil || found {
		return receipt, err
	}

	l, q, err := planRefund(ctx, tx, order.PracticeID, order.Credits, now)
	if err != nil {
		return RefundReceipt{}, err
	}

	actID, err := recordAct(ctx, q)
	if err != nil {
		return RefundReceipt{}, fmt.Errorf("billing: record refund act: %w", err)
	}

	// The request key rides through to Stripe as its idempotency key too,
	// so a retry that raced past the lookup above -- the first attempt's
	// transaction not yet committed -- replays the same Refund rather
	// than moving the money twice.
	refundID, err := client.RefundPayment(ctx, q.PaymentIntentID, order.RequestKey, q.AmountCents)
	if err != nil {
		return RefundReceipt{}, fmt.Errorf("billing: refund %d cents to practice %s: %w", q.AmountCents, order.PracticeID, err)
	}

	if _, err := tx.ExecContext(ctx,
		`SELECT ops_record_refund($1, $2, $3, $4, $5, $6, $7, $8)`,
		order.PracticeID, l.id, q.Credits, q.TaxCents, refundID, order.RequestKey, actID, now,
	); err != nil {
		// Two attempts at one request, racing: 00054 makes both the
		// request key and the Stripe Refund unique, so the second is
		// refused here rather than recorded twice.
		return RefundReceipt{}, fmt.Errorf("billing: insert refund row: %w", err)
	}

	return RefundReceipt{
		Credits:         q.Credits,
		AmountCents:     q.AmountCents,
		TaxCents:        q.TaxCents,
		StripeRefundID:  refundID,
		PaymentIntentID: q.PaymentIntentID,
	}, nil
}
