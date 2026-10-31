package engagement

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Engagement detail read, its activity ledger, its
// status transition, its birth-outcome write, its kind change, and the
// caller's own Attachment (#1598) and a named Doula's (#2230). AnyStaff mirrors
// visit.ListHandler: the money
// filter (Owner/Admin see every entry, everyone else never sees a
// Contract-price or Invoice/payment one, per ADR-0008) runs inside the
// handler's own query, not at this mount seam. TransitionHandler (#253)
// replaced the old completion-only POST .../complete: ADR-0015's six-move
// table is one bounded lifecycle transition, not a generic status PATCH
// a caller could half-apply, and pre-launch there is no reason to keep
// two endpoints doing overlapping halves of the same job. It carries no
// staffauth.AttachingWrite, the same as the endpoint it replaced: it is
// an Engagement lifecycle transition, not one of #350's four named write
// surfaces. RecordBirthOutcomeHandler (#293) carries none for the same
// reason and gates the same way: ADR-0015's role table refuses a
// contractor Doula outright rather than asking what she is attached to.
// ChangeKindHandler (#874) carries none and gates the same way too, for
// the identical reason.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}", staffauth.AnyStaff, DetailHandler())
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/activity", staffauth.AnyStaff, ListActivityHandler())
	ir.Exempt("PATCH /api/practices/{practiceId}/engagements/{engagementId}/status",
		"documented idempotent by construction: re-requesting the status an Engagement already holds is a no-op that only closes anything a partial earlier completion cascade left behind",
		false, TransitionHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/engagements/{engagementId}/birth-outcome",
		"naturally idempotent (docs/api-design.md rule 4): a PUT of the birth outcome the Engagement already holds writes nothing and returns the same 200",
		false, RecordBirthOutcomeHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/engagements/{engagementId}/kind",
		"naturally idempotent (docs/api-design.md rule 4): a PUT of the kind the Engagement already holds writes nothing and returns the same 200",
		false, ChangeKindHandler())
	// ChangeDueDateHandler (#2013) gates exactly as ChangeKindHandler does
	// (refuseFactWrite) and carries no staffauth.AttachingWrite, for the
	// same reason.
	ir.Exempt("PUT /api/practices/{practiceId}/engagements/{engagementId}/due-date",
		"naturally idempotent (docs/api-design.md rule 4): a PUT of the due date the Engagement already holds writes nothing and returns the same 200",
		false, ChangeDueDateHandler())
	// AttachSelfHandler (#1598) carries no staffauth.AttachingWrite: it
	// mints a granted Attachment itself, which that seam must never do,
	// and the seam attaches nobody who holds Owner or Admin. No role list
	// either: the rule is a role and an Employment type together, so the
	// handler asks it of the Reader (refuseSelfAttach).
	ir.Exempt("PUT /api/practices/{practiceId}/engagements/{engagementId}/attachments/me",
		"naturally idempotent (docs/api-design.md rule 4): a PUT by a caller who is already on the Engagement writes nothing and returns the same 200",
		false, AttachSelfHandler())
	// AttachDoulaHandler (#2230) carries no staffauth.AttachingWrite, for
	// AttachSelfHandler's reason. Putting somebody else on is the Owner's
	// or the Admin's act, a flat role list, so the mount declares it.
	// "me" above is the more specific pattern, so it keeps its handler.
	ir.ExemptGated("PUT /api/practices/{practiceId}/engagements/{engagementId}/attachments/{staffId}",
		"naturally idempotent (docs/api-design.md rule 4): a PUT for a Doula who is already on the Engagement writes nothing and returns the same 200",
		false, staffauth.OwnerAndAdmin, AttachDoulaHandler())
}
