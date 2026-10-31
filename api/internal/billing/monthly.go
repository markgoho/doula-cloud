package billing

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MonthLayout is how a month is written on the way in and on the way
// out of the monthly read: "2026-01".
const MonthLayout = "2006-01"

// PracticeMonth is one Practice's Credits bought and Credits spent in one
// calendar month, UTC -- one row of the founder's monthly read
// (ADR-0046). It names the Practice and nobody in her: no Staff member,
// no Client, no login, no feature.
type PracticeMonth struct {
	PracticeID           string  `json:"practiceId"`
	PracticeName         string  `json:"practiceName"`
	PracticeCreatedMonth string  `json:"practiceCreatedMonth"`
	Month                string  `json:"month"`
	CreditsBought        int     `json:"creditsBought"`
	CreditsSpent         int     `json:"creditsSpent"`
	FirstPurchaseMonth   *string `json:"firstPurchaseMonth"`
}

type monthlyPractice struct {
	id, name  string
	createdAt time.Time
	deletedAt sql.NullTime
}

// MonthlyCredits returns one row per Practice per calendar month she
// existed, from her creation month to the month of through (or of her
// deletion, if earlier), oldest Practice first. Months are UTC.
//
// Credits bought is the sum of purchase lots and Credits spent the count
// of consumption rows, so a signup bonus, a founding grant, a refund and
// a forfeit count as neither. Like DormantPractices it walks Practices
// one at a time, because credit_ledger's row-level security reads one
// Practice from the session variable.
func MonthlyCredits(ctx context.Context, tx *sql.Tx, through time.Time) ([]PracticeMonth, error) {
	through = monthStart(through)

	practices, err := monthlyPractices(ctx, tx)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}

	out := []PracticeMonth{}
	for _, p := range practices {
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, p.id); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, fmt.Errorf("billing: set current practice id: %w", err)
		}

		bought, spent, firstPurchase, err := ledgerByMonth(ctx, tx, p.id)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, err
		}

		last := through
		if p.deletedAt.Valid && monthStart(p.deletedAt.Time).Before(last) {
			last = monthStart(p.deletedAt.Time)
		}
		created := monthStart(p.createdAt)
		for m := created; !m.After(last); m = m.AddDate(0, 1, 0) {
			key := m.Format(MonthLayout)
			out = append(out, PracticeMonth{
				PracticeID:           p.id,
				PracticeName:         p.name,
				PracticeCreatedMonth: created.Format(MonthLayout),
				Month:                key,
				CreditsBought:        bought[key],
				CreditsSpent:         spent[key],
				FirstPurchaseMonth:   firstPurchase,
			})
		}
	}
	return out, nil
}

// monthlyPractices lists every Practice. practices carries no row-level
// security policy, so this is the one cross-tenant read MonthlyCredits
// needs.
func monthlyPractices(ctx context.Context, tx *sql.Tx) ([]monthlyPractice, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, name, created_at, deleted_at FROM practices ORDER BY created_at, id`)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: list practices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	practices := []monthlyPractice{}
	for rows.Next() {
		var p monthlyPractice
		if err := rows.Scan(&p.id, &p.name, &p.createdAt, &p.deletedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("billing: scan practice: %w", err)
		}
		practices = append(practices, p)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: iterate practices: %w", err)
	}
	return practices, nil
}

// ledgerByMonth reads the current Practice's purchases and consumptions
// keyed by UTC month, and the month of her first purchase, nil if she
// has bought none.
func ledgerByMonth(ctx context.Context, tx *sql.Tx, practiceID string) (bought, spent map[string]int, firstPurchase *string, err error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT to_char(date_trunc('month', created_at AT TIME ZONE 'UTC'), 'YYYY-MM'),
		        coalesce(sum(quantity) FILTER (WHERE origin = 'purchase'), 0),
		        count(*) FILTER (WHERE origin = 'consumption')
		 FROM credit_ledger
		 WHERE practice_id = $1 AND origin IN ('purchase', 'consumption')
		 GROUP BY 1
		 ORDER BY 1`, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, nil, nil, fmt.Errorf("billing: read ledger by month: %w", err)
	}
	defer func() { _ = rows.Close() }()

	bought, spent = map[string]int{}, map[string]int{}
	for rows.Next() {
		var month string
		var b, s int
		if err := rows.Scan(&month, &b, &s); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, nil, nil, fmt.Errorf("billing: scan ledger month: %w", err)
		}
		bought[month], spent[month] = b, s
		if b > 0 && firstPurchase == nil {
			first := month
			firstPurchase = &first
		}
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, nil, nil, fmt.Errorf("billing: iterate ledger months: %w", err)
	}
	return bought, spent, firstPurchase, nil
}

func monthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
