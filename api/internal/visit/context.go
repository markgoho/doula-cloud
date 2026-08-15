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
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/staffauth"
)

// requireTx resolves the request-scoped tx and Practice id
// staffauth.Middleware placed on context, writing the appropriate error
// response itself if the tx is somehow missing. Shared by ListHandler,
// CreateHandler, and ReassignHandler -- the same shape as engagement's own
// requireTx.
func requireTx(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, has := staffauth.Tx(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	practiceID, _ = staffauth.PracticeID(r.Context())
	return tx, practiceID, true
}

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

// parseUUID validates that pathValue is a well-formed UUID, writing a 400
// response itself (naming the field via label) if not. Shared by
// CreateHandler, ListHandler, and ReassignHandler, each of which parses one
// or more path/body ids the same way.
func parseUUID(w http.ResponseWriter, label, value string) (ok bool) {
	if _, err := uuid.Parse(value); err != nil {
		http.Error(w, "invalid "+label+" id", http.StatusBadRequest)
		return false
	}
	return true
}
