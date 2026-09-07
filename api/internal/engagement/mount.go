package engagement

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Engagement detail read, its activity ledger, and
// its status transition. AnyStaff mirrors visit.ListHandler: the money
// filter (Owner/Admin see every entry, everyone else never sees a
// Contract-price or Invoice/payment one, per ADR-0008) runs inside the
// handler's own query, not at this mount seam. TransitionHandler (#253)
// replaced the old completion-only POST .../complete: ADR-0015's six-move
// table is one bounded lifecycle transition, not a generic status PATCH
// a caller could half-apply, and pre-launch there is no reason to keep
// two endpoints doing overlapping halves of the same job. It carries no
// staffauth.AttachingWrite, the same as the endpoint it replaced: it is
// an Engagement lifecycle transition, not one of #350's four named write
// surfaces.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}", staffauth.AnyStaff, DetailHandler())
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/activity", staffauth.AnyStaff, ListActivityHandler())
	ir.Exempt("PATCH /api/practices/{practiceId}/engagements/{engagementId}/status",
		"documented idempotent by construction: re-requesting the status an Engagement already holds is a no-op that only closes anything a partial earlier completion cascade left behind",
		false, TransitionHandler())
}
