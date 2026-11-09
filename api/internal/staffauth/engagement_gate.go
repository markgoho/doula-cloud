package staffauth

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// requireEngagementAtPractice confirms engagementID belongs to practiceID,
// writing a 404 if it does not (or a 500 if the lookup fails). It stops a
// route from reaching an Engagement at another Practice, even one a caller
// guesses the id of.
func requireEngagementAtPractice(w http.ResponseWriter, r *http.Request, tx *sql.Tx, engagementID, practiceID string) bool {
	var exists bool
	err := tx.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM engagements WHERE id = $1 AND practice_id = $2)`,
		engagementID, practiceID,
	).Scan(&exists)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if !exists {
		apierr.WriteError(w, "engagement not found", http.StatusNotFound)
		return false
	}
	return true
}

// requireEngagementAccess confirms reader may reach engagementID
// (ADR-0008's read table, Reader.CanAccessEngagement), writing a 404 with
// notFound when she may not, so a contractor without access cannot tell
// "not attached" from "does not exist".
func requireEngagementAccess(w http.ResponseWriter, r *http.Request, tx *sql.Tx, reader Reader, engagementID, notFound string) bool {
	canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if !canAccess {
		apierr.WriteError(w, notFound, http.StatusNotFound)
		return false
	}
	return true
}
