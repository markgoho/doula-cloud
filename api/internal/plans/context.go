package plans

import (
	"net/http"

	"doula-cloud/api/internal/staffauth"
)

const noPlanInstanceFound = "no plan instance found for this engagement and plan type"

// instanceHandler is the common prologue shared by PostInstanceHandler,
// GetInstanceHandler, PutInstanceHandler and GetBirthPlanPDFHandler: the
// :engagementId and :planType path segments, validated, and the Engagement
// confirmed to belong to the current Practice. With reach, it is also
// narrowed by ADR-0008's attachment rule, a 404 for a contractor Doula who
// is not attached. fn receives the validated plan type.
func instanceHandler(fn func(w http.ResponseWriter, r *http.Request, c staffauth.Call, planType string), reach bool) http.Handler {
	access := staffauth.Engagement()
	if reach {
		access = staffauth.EngagementReach(noPlanInstanceFound)
	}
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		fn(w, r, c, c.ID("planType"))
	}, staffauth.EngagementID(), planTypeGate(), access)
}
