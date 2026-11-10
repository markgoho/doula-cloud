package billing

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Forfeit takes every Credit still in practiceID's lots off the ledger at
// now, and returns how many it took. It is a Practice's deletion's act
// (ADR-0031): the balance is forfeited, never refunded.
//
// It writes one forfeit row per lot ActForfeit admits, each naming its
// lot and taking exactly what that lot holds, so every lot is drained
// and the balance is zero -- and a refund or a consumption that comes
// after finds nothing to draw on. A Practice with nothing left writes
// nothing: an append-only ledger should not carry a zero-quantity row
// that answers no question.
//
// The caller holds the Practice row's lock and has set
// app.current_practice_id, as for every other draw.
func Forfeit(ctx context.Context, tx *sql.Tx, practiceID string, now time.Time) (int, error) {
	all, err := lotsAt(ctx, tx, practiceID, now)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return 0, err
	}
	forfeited := 0
	for _, l := range ActForfeit.from(all) {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO credit_ledger (practice_id, origin, quantity, drawn_lot_id, created_at)
			 VALUES ($1, $2, $3, $4, $5)`,
			practiceID, ActForfeit.Origin, -l.remaining, l.id, now,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return 0, fmt.Errorf("billing: forfeit lot %s: %w", l.id, err)
		}
		forfeited += l.remaining
	}
	return forfeited, nil
}
