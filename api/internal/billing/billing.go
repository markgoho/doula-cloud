// Package billing owns a Practice's credit_ledger: the append-only record
// of billing credits granted to and consumed by a Practice. #74 established
// the schema, its RLS policy, and the signup-bonus grant (staffauth.signup
// inserts that row transactionally with the Practice). #75 adds Balance,
// the derived-balance read used by GetBalanceHandler and available to other
// packages. #76 adds ConsumeCredit, the seam #52's Engagement-creation flow
// will invoke to spend one credit once it's wired in (a separate ticket --
// this one only builds and tests the operation itself). The Stripe
// purchase webhook is a separate ticket (see #73) that will add exported
// functions here.
package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNoCreditsRemaining is returned by ConsumeCredit when a Practice's
// derived balance is 0 or less -- the caller (#52's Engagement-creation
// handler) turns this into a "no credits left, ask an Owner to buy more"
// response.
var ErrNoCreditsRemaining = errors.New("billing: no credits remaining")

// ConsumeCredit spends one credit for practiceID, appending a -1
// credit_ledger row tagged with engagementID. It must run inside the
// caller's own transaction (tx) -- the same one that inserts the
// Engagement row -- so the consumption and the Engagement either both
// commit or both roll back.
//
// The caller (in practice, staffauth.Middleware, before the handler ever
// runs) must already have set app.current_practice_id to practiceID on
// tx, the same contract every other RLS-scoped write in this codebase
// relies on. There is no refund operation: a credit spent here stays
// spent even if the Engagement is later cancelled or voided.
func ConsumeCredit(ctx context.Context, tx *sql.Tx, practiceID, engagementID string) error {
	// Locks the Practice row for the rest of this transaction so two
	// concurrent ConsumeCredit calls for the same Practice can't both
	// read a positive balance and both insert a consumption row --
	// serialized to a plain read-balance-then-insert instead of a
	// check-then-act race. practices carries no RLS policy, so this
	// lock is unaffected by app.current_practice_id.
	if _, err := tx.ExecContext(ctx, `SELECT id FROM practices WHERE id = $1 FOR UPDATE`, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("billing: lock practice: %w", err)
	}

	balance, err := Balance(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if balance <= 0 {
		return ErrNoCreditsRemaining
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO credit_ledger (practice_id, origin, quantity, consumed_engagement_id, consumed_at)
		 VALUES ($1, 'consumption', -1, $2, now())`,
		practiceID, engagementID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("billing: insert consumption row: %w", err)
	}
	return nil
}
