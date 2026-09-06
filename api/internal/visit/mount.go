package visit

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Visit list and its two writes. Both writes carry
// attaching=true: staffauth.AttachingWrite is ADR-0008's write-side seam,
// attaching the acting Doula to the Engagement once the write succeeds.
// CreateHandler was newly wrapped Replayable in the 2026 idempotency-stance
// review -- an unconditional INSERT with a fresh id and no uniqueness
// guard, so a double-click logged a Visit twice.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/visits", staffauth.AnyStaff, ListHandler())
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/visits", true, CreateHandler())
	ir.Exempt("PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}",
		"plain UPDATE staff_id = $1 WHERE id = $2; sets the assignment to the given value, so re-sending the same body is a no-op",
		true, ReassignHandler())
}
