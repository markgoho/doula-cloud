package staffauth

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// RequireEngagement is the preamble of every route under
// /engagements/{engagementId}: the request-scoped tx, the Practice, and
// the path's Engagement id, parsed and confirmed to belong to that
// Practice. It writes the 400, 404 or 500 itself.
func RequireEngagement(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID, engagementID string, ok bool) {
	tx, practiceID, ok = RequireTx(w, r)
	// coverage:ignore reason: Middleware always sets a tx before this handler runs
	if !ok {
		return nil, "", "", false
	}
	engagementID = r.PathValue("engagementId")
	if !ParseUUID(w, "engagement", engagementID) {
		return nil, "", "", false
	}
	if !RequireEngagementAtPractice(w, r, tx, engagementID, practiceID) {
		return nil, "", "", false
	}
	return tx, practiceID, engagementID, true
}

// RequireEngagementReach is RequireEngagement narrowed by ADR-0008's
// attachment rule: a contractor Doula who is not attached gets notFound.
func RequireEngagementReach(w http.ResponseWriter, r *http.Request, notFound string) (tx *sql.Tx, practiceID, engagementID string, ok bool) {
	tx, practiceID, engagementID, ok = RequireEngagement(w, r)
	if !ok {
		return nil, "", "", false
	}
	if !RequireEngagementAccess(w, r, tx, engagementID, notFound) {
		return nil, "", "", false
	}
	return tx, practiceID, engagementID, true
}

// RequireEngagementAtPractice confirms engagementID belongs to practiceID,
// writing a 404 if it does not (or a 500 if the lookup fails). It stops a
// route from reaching an Engagement at another Practice, even one a caller
// guesses the id of.
func RequireEngagementAtPractice(w http.ResponseWriter, r *http.Request, tx *sql.Tx, engagementID, practiceID string) bool {
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

// RequireEngagementAccess confirms the caller's Reader may reach
// engagementID (ADR-0008's read table, Reader.CanAccessEngagement),
// writing a 404 with notFound when she may not, so a contractor without
// access cannot tell "not attached" from "does not exist".
func RequireEngagementAccess(w http.ResponseWriter, r *http.Request, tx *sql.Tx, engagementID, notFound string) bool {
	reader, ok := RequireReader(w, r)
	if !ok {
		// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
		return false
	}
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
