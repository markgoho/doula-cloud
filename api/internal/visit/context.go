// Package visit holds the Staff-side BFF handlers for Visit: list, create,
// and reassign. All three rely on staffauth.Middleware having already
// resolved the caller's Staff/Practice ids and opened a request-scoped
// *sql.Tx with app.current_practice_id set, the same way the engagement
// package's handlers do.
package visit

import (
	"context"
	"database/sql"
	"fmt"
)

// requireEngagementAtPractice confirms engagementID exists and belongs to
// practiceID, returning sql.ErrNoRows if not -- callers translate that into
// a 404, the same way engagement.DetailHandler does for the engagement
// itself. Shared by CreateHandler, ListHandler, and ReassignHandler so a
// Visit can never be created, listed, or reassigned under an Engagement at
// a different Practice, even one an attacker guesses the id of.
func requireEngagementAtPractice(ctx context.Context, tx *sql.Tx, engagementID, practiceID string) error {
	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM engagements WHERE id = $1 AND practice_id = $2)`,
		engagementID, practiceID,
	).Scan(&exists)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("visit: check engagement at practice: %w", err)
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}
