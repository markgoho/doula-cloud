package outbox

import (
	"context"
	"database/sql"
	"fmt"
)

const recipientsSelect = `SELECT s.id, s.email FROM staff s
		 JOIN practice_memberships pm ON pm.staff_id = s.id
		 WHERE pm.practice_id = $1 AND `

const (
	ownerRecipientsQuery         = recipientsSelect + `'owner' = ANY(pm.roles) ORDER BY s.id`
	ownerAndAdminRecipientsQuery = recipientsSelect + `(pm.roles && ARRAY['owner', 'admin']::practice_role[]) ORDER BY s.id`
)

// OwnerRecipients returns the Staff id and email of every Staff member
// holding the owner role at practiceID, ordered by id. A worker that runs
// outside a Staff session relies on 00033's
// app.notification_worker_trusted policies on staff and
// practice_memberships, which are table-generic, so each worker reuses
// them rather than minting its own.
func OwnerRecipients(ctx context.Context, tx *sql.Tx, practiceID string) (staffIDs, emails []string, err error) {
	return recipients(ctx, tx, practiceID, ownerRecipientsQuery)
}

// OwnerAndAdminRecipients is OwnerRecipients for the owner or admin role.
func OwnerAndAdminRecipients(ctx context.Context, tx *sql.Tx, practiceID string) (staffIDs, emails []string, err error) {
	return recipients(ctx, tx, practiceID, ownerAndAdminRecipientsQuery)
}

func recipients(ctx context.Context, tx *sql.Tx, practiceID, query string) (staffIDs, emails []string, err error) {
	rows, err := tx.QueryContext(ctx, query, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, nil, fmt.Errorf("outbox: resolve recipients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, nil, fmt.Errorf("outbox: scan recipient: %w", err)
		}
		staffIDs = append(staffIDs, id)
		emails = append(emails, email)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: cursor failure, not exercised by unit tests
		return nil, nil, fmt.Errorf("outbox: read recipients: %w", err)
	}
	return staffIDs, emails, nil
}
