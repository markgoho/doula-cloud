package practicetimezone

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers a Practice's own timezone (#1166): the write stays
// Owner-or-Admin, the same authority that states the rate card and the
// payment terms, but the read widened to AnyStaff (#1280) -- see
// GetHandler's own doc comment for why. Declared here rather than checked
// inside the handlers -- #990's form, so the startup panic and the
// registry-walking guardrails can both see the role.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/timezone", staffauth.AnyStaff, GetHandler())
	ir.ExemptGated("PUT /api/practices/{practiceId}/timezone",
		"full-replace UPDATE of the Practice's one zone; re-sending the same body is a no-op that records nothing new (see PutHandler's own doc comment)",
		false, staffauth.OwnerAndAdmin, PutHandler())
}
