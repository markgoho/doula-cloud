package payments

import (
	"context"
	"database/sql"
	"fmt"

	"doula-cloud/api/internal/overdue"
)

// OwedSummary (#2755) is what an Engagement's open Invoices add up to, over
// every one of them rather than the page the list carries, so the record's
// at-a-glance "Owed" never waits on paging. Money is cents. OverdueCents
// is the part of OwedCents the Practice-wide list also calls overdue
// (overdue.Expr). NextDueOn is the earliest due day among the open
// Invoices that are not overdue, and OldestOverdueOn the earliest due day
// among those that are; each is a Practice-day YYYY-MM-DD and absent where
// no Invoice fits.
type OwedSummary struct {
	OwedCents       int64   `json:"owedCents"`
	OverdueCents    int64   `json:"overdueCents"`
	NextDueOn       *string `json:"nextDueOn,omitempty"`
	OldestOverdueOn *string `json:"oldestOverdueOn,omitempty"`
}

// owedSummaryQuery is one pass with FILTER clauses, as
// practiceInvoiceTotalsQuery is. $1 is the Engagement and $2 is
// overdue.From's instant.
var owedSummaryQuery = `SELECT
		COALESCE(SUM(i.amount_cents), 0),
		COALESCE(SUM(i.amount_cents) FILTER (WHERE ` + overdue.Expr("$2") + `), 0),
		MIN(i.due_at) FILTER (WHERE NOT ` + overdue.Expr("$2") + `),
		MIN(i.due_at) FILTER (WHERE ` + overdue.Expr("$2") + `)
	FROM invoices i
	JOIN contracts c ON c.id = i.contract_id
	WHERE c.engagement_id = $1 AND i.status = 'open'`

// readOwedSummary reads engagementID's OwedSummary, judging lateness against
// days, the same zone and instant the list beside it uses.
func readOwedSummary(ctx context.Context, tx *sql.Tx, engagementID string, days bookDays) (*OwedSummary, error) {
	var owed OwedSummary
	var nextDue, oldestOverdue sql.NullTime
	if err := tx.QueryRowContext(ctx, owedSummaryQuery, engagementID, overdue.From(days.now, days.zone)).
		Scan(&owed.OwedCents, &owed.OverdueCents, &nextDue, &oldestOverdue); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("payments: owed summary: %w", err)
	}
	if nextDue.Valid {
		day := practiceDay(nextDue.Time, days.zone)
		owed.NextDueOn = &day
	}
	if oldestOverdue.Valid {
		day := practiceDay(oldestOverdue.Time, days.zone)
		owed.OldestOverdueOn = &day
	}
	return &owed, nil
}
