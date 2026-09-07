// Package visit holds the Staff-side BFF handlers for Visit: list, create,
// reassign, schedule, and notes. All five rely on staffauth.Middleware
// having already resolved the caller's Staff/Practice ids and opened a
// request-scoped *sql.Tx with app.current_practice_id set, the same way
// the engagement package's handlers do.
package visit

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
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

// parseScheduledAt turns a request's optional RFC3339 scheduledAt string
// into a *time.Time, shared by CreateHandler and ScheduleHandler so the
// two writes agree on the one format a caller may send. raw == nil (the
// field absent, explicitly null, or the whole body absent) means
// unscheduled and returns (nil, true) -- not an error. A raw value that
// fails to parse writes its own 400 and returns ok=false, the caller's
// signal to return without doing anything else.
func parseScheduledAt(w http.ResponseWriter, raw *string) (scheduledAt *time.Time, ok bool) {
	if raw == nil {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		apierr.WriteError(w, "scheduledAt must be an RFC3339 timestamp", http.StatusBadRequest)
		return nil, false
	}
	return &parsed, true
}
