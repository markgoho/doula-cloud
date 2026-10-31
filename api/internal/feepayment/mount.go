package feepayment

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Fee Payment routes (#2271). Owner and Admin only,
// for the read as for the writes: what the Practice paid one Doula is
// between her and the Practice (ADR-0008), and the contractor reads her
// own on Your offers instead. Both writes are Replayable, as recording and
// reversing a by-hand Payment are: a double-click must not record the
// same fee twice.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/doula-fees", staffauth.OwnerAndAdmin, ListHandler())
	ir.ReplayableGated("POST /api/practices/{practiceId}/engagements/{engagementId}/doula-fees/{attachmentId}/payments",
		true, staffauth.OwnerAndAdmin, RecordHandler())
	ir.ReplayableGated("POST /api/practices/{practiceId}/engagements/{engagementId}/doula-fees/{attachmentId}/payments/{paymentId}/reverse",
		true, staffauth.OwnerAndAdmin, ReverseHandler())
}
