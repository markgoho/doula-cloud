package practicerate

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"doula-cloud/api/internal/activity"
)

// contractAmountRepriceDiff is a reprice pass's per-Contract activity
// diff, the same shape contracts.contractAmountDiff uses for an
// Owner/Admin override -- kept as its own unexported type here rather
// than importing contracts (practicerate has no other dependency on that
// package, and #966/#967 already established the convention of one
// package reaching another's table directly via SQL rather than a
// function call -- see contracts/price.go's own join against
// practice_rates).
type contractAmountRepriceDiff struct {
	AmountCentsBefore int64 `json:"amountCentsBefore"`
	AmountCentsAfter  int64 `json:"amountCentsAfter"`
}

// repriceUnsignedContracts is PutRateHandler's side effect (#968): every
// Contract at practiceID for kind that has not yet been signed --
// status 'draft' or 'sent', never 'signed' or 'voided' (a voided
// Contract was signed before it was voided, so its content is just as
// frozen) -- and whose amount was never overridden by an Owner or Admin
// (amount_overridden) re-derives to newAmountCents. Runs in the same tx
// PutRateHandler already holds, so the rate change and every reprice it
// causes land together or not at all.
//
// The UPDATE...FROM...RETURNING shape reads the before-amount and the
// Engagement id each affected row needs for its own activity entry in
// the same statement that writes the new amount, rather than a SELECT
// first and a second UPDATE -- there is no window in which a concurrent
// override could slip in between reading "before" and writing "after".
// != $3 in the CTE's WHERE also makes this idempotent the same way
// PutRateHandler's own caller-facing write is: a rate "changed" to the
// value it already held never reaches here at all (PutRateHandler only
// calls this once before/after have already been proven to differ), and
// a Contract already sitting at newAmountCents (e.g. a second kind's
// rate change coincidentally matching an unrelated Contract) is left
// alone and gets no activity entry.
func repriceUnsignedContracts(ctx context.Context, tx *sql.Tx, practiceID, kind string, newAmountCents int64) error {
	rows, err := tx.QueryContext(ctx,
		`WITH repriced AS (
			SELECT c.id, c.engagement_id, c.amount_cents AS before_amount
			FROM contracts c
			JOIN engagements e ON e.id = c.engagement_id
			WHERE e.practice_id = $1
			  AND e.kind = $2::engagement_kind
			  AND c.status IN ('draft', 'sent')
			  AND c.amount_overridden = false
			  AND c.amount_cents != $3
		)
		UPDATE contracts c SET amount_cents = $3, amount_changed_at = now()
		FROM repriced
		WHERE c.id = repriced.id
		RETURNING repriced.engagement_id, repriced.before_amount`,
		practiceID, kind, newAmountCents,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("practicerate: reprice unsigned contracts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type repriced struct {
		engagementID string
		before       int64
	}
	var affected []repriced
	for rows.Next() {
		var r repriced
		if err := rows.Scan(&r.engagementID, &r.before); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return fmt.Errorf("practicerate: scan repriced contract: %w", err)
		}
		affected = append(affected, r)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB iteration failure, not exercised by unit tests
		return fmt.Errorf("practicerate: iterate repriced contracts: %w", err)
	}

	// Record must run after rows is fully drained and closed -- both
	// query the same tx, and a *sql.Rows left open blocks a second
	// query on the same connection.
	for _, r := range affected {
		diffJSON, err := json.Marshal(contractAmountRepriceDiff{AmountCentsBefore: r.before, AmountCentsAfter: newAmountCents})
		if err != nil {
			// coverage:ignore reason: marshal of a fixed, always-serializable struct never fails
			return fmt.Errorf("practicerate: marshal reprice diff: %w", err)
		}
		if err := activity.Record(ctx, tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   r.engagementID,
			Action:      string(activity.ActionContractAmountRepriced),
			Diff:        diffJSON,
			Actor:       activity.SystemActor(),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("practicerate: record reprice: %w", err)
		}
	}
	return nil
}
