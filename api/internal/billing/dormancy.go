package billing

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DormancyNoticeYears is how long without contact makes a Practice's
// unspent balance worth writing to her about.
//
// Two years, not three: APL 1315(1-b) escheats the balance at three
// years' dormancy, and the due-diligence mailing has to reach the owner
// before that, not after. A year's warning is what turns an escheat into
// a Practice spending her own Credits.
const DormancyNoticeYears = 2

// DormantPractice is a Practice holding unspent Credits that nobody has
// been seen using -- the row the annual balance notice and the
// December/January due-diligence mailings are addressed from.
type DormantPractice struct {
	PracticeID    string     `json:"practiceId"`
	Balance       int        `json:"balance"`
	LastContactAt *time.Time `json:"lastContactAt"`
}

// practiceIDs lists every Practice id. practices carries no row-level
// security policy, so this is the one cross-tenant read the sweep below
// needs; everything it goes on to read is scoped one Practice at a time.
func practiceIDs(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM practices ORDER BY id`)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: list practices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("billing: scan practice id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("billing: iterate practices: %w", err)
	}
	return ids, nil
}

// DormantPractices lists the Practices holding a positive Credit balance
// whose Staff have not been active since notSeenSince -- a Practice with
// no recorded contact at all counts as dormant, since a Practice that has
// never signed in is exactly the one the notice is for.
//
// It walks Practices one at a time rather than reading the whole ledger
// in one statement, because row-level security is what scopes
// credit_ledger and staff, and the session variable those policies read
// holds one Practice at a time. practices itself carries no policy, so
// listing them is the one cross-tenant read this needs.
func DormantPractices(ctx context.Context, tx *sql.Tx, notSeenSince time.Time) ([]DormantPractice, error) {
	ids, err := practiceIDs(ctx, tx)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}

	dormant := []DormantPractice{}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, id); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, fmt.Errorf("billing: set current practice id: %w", err)
		}

		balance, err := Balance(ctx, tx, id)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, err
		}
		if balance <= 0 {
			continue
		}

		var lastContact sql.NullTime
		if err := tx.QueryRowContext(ctx,
			`SELECT max(s.last_active_at)
			 FROM staff s
			 JOIN practice_memberships pm ON pm.staff_id = s.id
			 WHERE pm.practice_id = $1`, id,
		).Scan(&lastContact); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, fmt.Errorf("billing: read last practice contact: %w", err)
		}
		if lastContact.Valid && lastContact.Time.After(notSeenSince) {
			continue
		}

		entry := DormantPractice{PracticeID: id, Balance: balance}
		if lastContact.Valid {
			seen := lastContact.Time
			entry.LastContactAt = &seen
		}
		dormant = append(dormant, entry)
	}
	return dormant, nil
}
