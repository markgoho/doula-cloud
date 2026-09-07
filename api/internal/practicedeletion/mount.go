package practicedeletion

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers #871's three routes: the pending-deletion read, the
// act that starts the 30-day window, and the act that undoes it.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/deletion", staffauth.OwnerOnly, StatusHandler())
	// InitiateHandler locks the practices row FOR UPDATE and refuses a
	// repeat while deletion is already pending or final, the same
	// self-idempotent shape client/mount.go's own erasure POST argues for
	// Exempt.
	ir.Exempt("POST /api/practices/{practiceId}/deletion",
		"InitiateHandler locks the practices row FOR UPDATE and refuses a repeat while deletion is already pending or final, so a retry after the first commit 409s instead of enqueueing a second reminder and finalization",
		false, InitiateHandler())
	// DELETE is naturally idempotent (docs/api-design.md section 3):
	// restoring an already-restored Practice 409s rather than no-opping,
	// but that refusal is itself stable under a retry.
	ir.Exempt("DELETE /api/practices/{practiceId}/deletion",
		"DELETE is naturally idempotent per docs/api-design.md section 3; RestoreHandler's own 409 on nothing-pending is stable under a retry",
		false, RestoreHandler())
}
