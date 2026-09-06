package engagement

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Engagement detail read, its activity ledger, and
// completing it. AnyStaff mirrors visit.ListHandler: the money filter
// (Owner/Admin see every entry, everyone else never sees a Contract-price
// or Invoice/payment one, per ADR-0008) runs inside the handler's own
// query, not at this mount seam. Completing an Engagement runs ADR-0008's
// cascade -- open Offers withdrawn, open attachments ended -- so it is
// one endpoint, not a generic status PATCH a caller could half-apply, and
// it carries no staffauth.AttachingWrite: it is an Engagement lifecycle
// transition, not one of #350's four named write surfaces.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}", staffauth.AnyStaff, DetailHandler())
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/activity", staffauth.AnyStaff, ListActivityHandler())
	ir.Exempt("POST /api/practices/{practiceId}/engagements/{engagementId}/complete",
		"documented idempotent by construction: re-running the completion cascade on an already-completed Engagement is a no-op that only closes anything a partial earlier run left behind",
		false, CompleteHandler())
}
