package contracts

import (
	"database/sql"
	"net/http"

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
	if !staffauth.RequireEngagementAtPractice(w, r, tx, engagementID, practiceID) {
		return nil, "", false
	}

	return tx, engagementID, true
}
