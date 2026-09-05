package contracts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// resolveContractRequest resolves the request-scoped tx (set by
// staffauth.Middleware) and the :engagementId path segment, and confirms
// the Engagement belongs to the current Practice -- the common prologue
// shared by PostContractHandler, GetContractHandler, and
// PutContractHandler. Writes the appropriate error response itself and
// returns ok=false on any failure; callers just return in that case.
func resolveContractRequest(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, engagementID string, ok bool) {
	tx, practiceID, ok := staffauth.RequireTx(w, r)
	// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
	if !ok {
		return nil, "", false
	}

	engagementID = r.PathValue("engagementId")
	if !staffauth.ParseUUID(w, "engagement", engagementID) {
		return nil, "", false
	}
	if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return nil, "", false
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}

	return tx, engagementID, true
}

// requireEngagementAtPractice confirms engagementID exists and belongs to
// practiceID, returning sql.ErrNoRows if not -- callers translate that
// into a 404, mirroring plans.requireEngagementAtPractice.
func requireEngagementAtPractice(ctx context.Context, tx *sql.Tx, engagementID, practiceID string) error {
	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM engagements WHERE id = $1 AND practice_id = $2)`,
		engagementID, practiceID,
	).Scan(&exists)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("contracts: check engagement at practice: %w", err)
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}
