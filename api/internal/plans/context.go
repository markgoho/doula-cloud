package plans

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// resolveInstanceRequest resolves the request-scoped tx (set by
// staffauth.Middleware), the :engagementId and :planType path segments,
// and confirms the Engagement belongs to the current Practice -- the
// common prologue shared by PostInstanceHandler, GetInstanceHandler, and
// PutInstanceHandler. Writes the appropriate error response itself and
// returns ok=false on any failure; callers just return in that case.
func resolveInstanceRequest(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, engagementID, planType string, ok bool) {
	tx, practiceID, ok := staffauth.RequireTx(w, r)
	// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
	if !ok {
		return nil, "", "", false
	}

	engagementID = r.PathValue("engagementId")
	if !staffauth.ParseUUID(w, "engagement", engagementID) {
		return nil, "", "", false
	}
	if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return nil, "", "", false
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return nil, "", "", false
	}

	planType, ok = planTypeFromPath(w, r)
	if !ok {
		return nil, "", "", false
	}
	return tx, engagementID, planType, true
}

// requireEngagementAtPractice confirms engagementID exists and belongs to
// practiceID, returning sql.ErrNoRows if not -- callers translate that
// into a 404, mirroring visit.requireEngagementAtPractice.
func requireEngagementAtPractice(ctx context.Context, tx *sql.Tx, engagementID, practiceID string) error {
	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM engagements WHERE id = $1 AND practice_id = $2)`,
		engagementID, practiceID,
	).Scan(&exists)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("plans: check engagement at practice: %w", err)
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}
