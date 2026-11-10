package billing

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// lot is one grant or purchase row of credit_ledger together with how much
// of it is still unspent -- the unit the ledger is actually drawn from.
// #420 gave every lot a price so that "at the price paid for them" is
// answerable from our own data years later; a granted lot carries a real
// $0.00 rather than an unknown price.
type lot struct {
	id                    string
	origin                string
	quantity              int
	remaining             int
	refundedCount         int
	unitPriceCents        int64
	taxCents              int64
	taxRefundedCents      int64
	stripePaymentIntentID sql.NullString
	createdAt             time.Time
	state                 LotState
	refundable            bool
}

// lotsAt lists every one of a Practice's lots as it stands at at, oldest
// first, read from credit_lots (00176) -- the one derivation of a lot's
// remainder, its state and whether a refund may draw on it. A caller
// picks the lots its act may draw on with LotAct.from.
//
// FIFO is the one draw order, for consumption, refunds and forfeiture
// alike (#390). Oldest-first is what makes a grant reach the Practice it
// was given to before anything she paid for is touched, and it drains the
// lots nearest their three-year refund window first, which is the
// direction that favors the Practice.
//
// remaining subtracts every draw already made against the lot --
// consumption, refund and forfeit rows all point at it through
// drawn_lot_id and all carry a negative quantity, so one sum covers them,
// and the remainders of a Practice's lots sum to its balance.
//
// refundedCount and taxRefundedCents count only the refunds, and they are
// what stop a partly refunded lot from returning its tax twice. They are
// separate from remaining because a spent Credit and a refunded Credit
// both leave the balance and mean opposite things about the tax: tax
// follows the money back only when the money goes back.
func lotsAt(ctx context.Context, tx *sql.Tx, practiceID string, at time.Time) ([]lot, error) {
	rows, err := tx.QueryContext(ctx, `SELECT * FROM credit_lots($1, $2)`, practiceID, at)
	return scanLots(rows, err)
}

// refundLots is lotsAt as ops_runtime reads it, through ops_refund_lots
// (00148, restated over credit_lots by 00176), which also locks the
// Practice row the way ConsumeCredit does, so two refunds cannot both
// read one lot as unspent.
func refundLots(ctx context.Context, tx *sql.Tx, practiceID string, at time.Time) ([]lot, error) {
	rows, err := tx.QueryContext(ctx, `SELECT * FROM ops_refund_lots($1, $2)`, practiceID, at)
	return scanLots(rows, err)
}

func scanLots(rows *sql.Rows, err error) ([]lot, error) {
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: list credit lots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	lots := []lot{}
	for rows.Next() {
		var l lot
		if err := rows.Scan(&l.id, &l.origin, &l.quantity, &l.unitPriceCents, &l.taxCents,
			&l.stripePaymentIntentID, &l.createdAt, &l.remaining, &l.refundedCount, &l.taxRefundedCents,
			&l.state, &l.refundable); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("billing: scan credit lot: %w", err)
		}
		lots = append(lots, l)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: iterate credit lots: %w", err)
	}
	return lots, nil
}

// taxOnCredits is the tax to return when refundedSoFar + n Credits of a
// lot have been refunded, given that alreadyReturned cents of the lot's
// tax have gone back already. refundedSoFar counts refunds only: a Credit
// that was spent bought what it was charged tax on, and that tax is New
// York's to keep.
//
// The share is computed cumulatively rather than per-refund, so the
// fractions of a cent that each partial refund drops are picked up by the
// next one, and refunding a whole lot -- in one go or in six -- always
// returns exactly the tax that was charged on it. Truncation means a
// partial refund never returns more tax than its share, which is the safe
// direction: the difference stays with the Practice's remaining Credits
// rather than being claimed back from New York early.
func taxOnCredits(taxCents int64, quantity, refundedSoFar, n int, alreadyReturned int64) int64 {
	cumulative := taxCents * int64(refundedSoFar+n) / int64(quantity)
	return cumulative - alreadyReturned
}
