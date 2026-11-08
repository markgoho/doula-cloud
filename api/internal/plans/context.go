package plans

import (
	"database/sql"
	"net/http"

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
	if !staffauth.RequireEngagementAtPractice(w, r, tx, engagementID, practiceID) {
		return nil, "", "", false
	}

	planType, ok = planTypeFromPath(w, r)
	if !ok {
		return nil, "", "", false
	}
	return tx, engagementID, planType, true
}
