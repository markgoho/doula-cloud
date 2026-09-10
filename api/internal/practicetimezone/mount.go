package practicetimezone

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers a Practice's own timezone (#1166): the read and the
// write, both Owner-or-Admin, declared here rather than checked inside
// the handlers -- #990's form, so the startup panic and the
// registry-walking guardrails can both see the role.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/timezone", staffauth.OwnerAndAdmin, GetHandler())
	ir.ExemptGated("PUT /api/practices/{practiceId}/timezone",
		"full-replace UPDATE of the Practice's one zone; re-sending the same body is a no-op that records nothing new (see PutHandler's own doc comment)",
		false, staffauth.OwnerAndAdmin, PutHandler())
}
