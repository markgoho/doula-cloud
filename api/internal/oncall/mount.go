package oncall

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the on-call surface (#1093).
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/on-call", staffauth.AnyStaff, RosterHandler())
	_ = ir
}
