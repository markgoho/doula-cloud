package staffauth

import (
	"context"
	"database/sql"
	"fmt"
)

// OwnerEmails returns the email of every Staff member holding the owner
// role at practiceID. A worker that runs outside a Staff session relies on
// 00033's app.notification_worker_trusted policies on staff and
// practice_memberships, which are table-generic, so each worker reuses
// them rather than minting its own.
func OwnerEmails(ctx context.Context, tx *sql.Tx, practiceID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT s.email FROM staff s
		 JOIN practice_memberships pm ON pm.staff_id = s.id
		 WHERE pm.practice_id = $1 AND 'owner' = ANY(pm.roles)`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("staffauth: resolve owner emails: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("staffauth: scan owner email: %w", err)
		}
		emails = append(emails, email)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("staffauth: iterate owner emails: %w", err)
	}
	return emails, nil
}
