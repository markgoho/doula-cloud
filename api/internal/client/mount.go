package client

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// Mount registers the Client write surface (#397): search, lookup-before-insert
// create, the detail read, edit, merge and erasure. Saving or editing a
// Client is free and creates no Engagement -- that split off into a
// separate Engagement Request, built elsewhere. Role gating beyond "any
// Staff member" (the contractor create/search refusal, the attached-Clients
// narrowing on edit/detail) is enforced inside each handler via
// staffauth.Reader, the same pattern engagement.DetailHandler uses for
// CanAccessEngagement.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, nudge tasknudge.Enqueuer) {
	g.Get("/api/practices/{practiceId}/clients", staffauth.AnyStaff, ListHandler())
	g.Get("/api/practices/{practiceId}/clients/search", staffauth.AnyStaff, SearchHandler())
	ir.Replayable("POST /api/practices/{practiceId}/clients", false, CreateHandler())
	g.Get("/api/practices/{practiceId}/clients/{clientId}", staffauth.AnyStaff, DetailHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/clients/{clientId}",
		"PUT replaces the Client record wholesale; re-sending the same body is a no-op",
		false, EditHandler())
	// ADR-0017's amendment (#814): gate two's "This is her". Sets
	// merged_into on the absorbed row exactly once -- clients_update's
	// own USING clause (00080, merged_into IS NULL) refuses a second
	// write to an already-tombstoned row, so a retry after the first
	// commit 409s instead of double-recording the absorb, the same shape
	// EraseHandler's own exemption below already argues.
	ir.Exempt("POST /api/practices/{practiceId}/clients/{clientId}/merge",
		"setMergedInto writes merged_into on the absorbed row exactly once; clients_update's USING clause (merged_into IS NULL) refuses a second write to an already-tombstoned row, so a retry after the first commit 409s instead of absorbing it twice",
		false, MergeHandler())
	// #691's precheck: the same unsettled-invoice fact the POST below
	// 409s on, read ahead of time so an Owner never reaches that 409 by
	// way of the confirmation screen. Owner-only, mirroring the POST's
	// own gate, per the payments/connect precedent.
	g.Get("/api/practices/{practiceId}/clients/{clientId}/erasure", staffauth.OwnerOnly, EraseEligibilityHandler())
	// #394's erasure, ADR-0027: the one act in the product that destroys
	// a fact, so Owner-only (enforced inside the handler by
	// staffauth.RequireOwner, the same seat as the MFA switch), and the
	// one route here whose repeat is a mistake worth naming -- it locks
	// the row FOR UPDATE and 409s on an erased_at that is already set,
	// so a retry after the first commit refuses rather than erasing
	// twice or double-calling Stripe.
	ir.Exempt("POST /api/practices/{practiceId}/clients/{clientId}/erasure",
		"erase() locks the clients row FOR UPDATE and refuses a row whose erased_at is already set; a retry after the first commit 409s instead of enqueuing a second set of Stripe and Identity Platform acts",
		false, EraseHandler(nudge))
}
