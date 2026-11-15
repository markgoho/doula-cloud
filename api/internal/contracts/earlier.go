package contracts

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// EarlierContract is a voided Contract that an Engagement's current one
// replaced: the Contract section keeps it as history under its own
// anchor, so the Contracts list's row for it opens somewhere that shows
// it. VoidedBy is empty where the row's Activity was gone when the column
// was backfilled. RequestedBy and Reason come from the void request an
// Owner or an Admin granted, and are empty where the void was done
// without one.
type EarlierContract struct {
	ID          string    `json:"id"`
	VoidedAt    time.Time `json:"voidedAt"`
	VoidedBy    string    `json:"voidedBy,omitempty"`
	RequestedBy string    `json:"requestedBy,omitempty"`
	Reason      string    `json:"reason,omitempty"`
}

// contractHistory reads the id of engagementID's current Contract -- the
// newest, the one fetchContract reads -- and every voided Contract before
// it, newest first.
func contractHistory(ctx context.Context, tx *sql.Tx, engagementID string) (currentID string, earlier []EarlierContract, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM contracts WHERE engagement_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`,
		engagementID,
	).Scan(&currentID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- every caller has just read or written this Contract
		return "", nil, fmt.Errorf("contracts: read current contract id: %w", err)
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT c.id, c.voided_at, COALESCE(vs.name, ''), COALESCE(g.requested_by_name, ''), COALESCE(g.reason, '')
		   FROM contracts c
		   LEFT JOIN staff vs ON vs.id = c.voided_by
		   LEFT JOIN LATERAL (
		       SELECT rs.name AS requested_by_name, v.reason
		         FROM contract_void_requests v
		         JOIN staff rs ON rs.id = v.requested_by
		        WHERE v.contract_id = c.id AND v.status = 'voided'
		        ORDER BY v.created_at DESC, v.id DESC
		        LIMIT 1
		   ) g ON true
		  WHERE c.engagement_id = $1 AND c.status = 'voided' AND c.id <> $2
		  ORDER BY c.created_at DESC, c.id DESC`,
		engagementID, currentID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", nil, fmt.Errorf("contracts: list earlier contracts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var item EarlierContract
		if err := rows.Scan(&item.ID, &item.VoidedAt, &item.VoidedBy, &item.RequestedBy, &item.Reason); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return "", nil, fmt.Errorf("contracts: scan earlier contract: %w", err)
		}
		earlier = append(earlier, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return "", nil, fmt.Errorf("contracts: iterate earlier contracts: %w", err)
	}
	return currentID, earlier, nil
}
