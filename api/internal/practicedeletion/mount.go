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
	// Owner-only is declared here rather than checked in each handler
	// (#1028, following #970, #990 and #1016) -- the same seat the
	// pending-deletion read above already declares, and never a reach
	// question: ending the Practice is the Owner's act outright.
	ir.ExemptGated("POST /api/practices/{practiceId}/deletion",
		"InitiateHandler locks the practices row FOR UPDATE and refuses a repeat while deletion is already pending or final, so a retry after the first commit 409s instead of enqueueing a second reminder and finalization",
		false, staffauth.OwnerOnly, InitiateHandler())
	// DELETE is naturally idempotent (docs/api-design.md section 3):
	// restoring an already-restored Practice 409s rather than no-opping,
	// but that refusal is itself stable under a retry.
	ir.ExemptGated("DELETE /api/practices/{practiceId}/deletion",
		"DELETE is naturally idempotent per docs/api-design.md section 3; RestoreHandler's own 409 on nothing-pending is stable under a retry",
		false, staffauth.OwnerOnly, RestoreHandler())
}
