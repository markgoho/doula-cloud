package engagementrequest

import (
	"database/sql"

	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// Mount registers ADR-0017's Engagement Request (#398): the ask for paid
// work with a Client, and the act that creates an Engagement. Request is
// any Staff member but a contractor Doula (enforced here and,
// independently, by engagement_requests_insert's RLS policy);
// approve/refuse are Owner/Admin, and so is the approval screen's own
// read (#502) -- the seat that decides is the seat that reads, and the
// balance the read carries is Owner/Admin-only on its own; withdraw is
// the requester alone, so it carries no role declaration.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB, nudge tasknudge.Enqueuer) {
	ir.Replayable("POST /api/practices/{practiceId}/clients/{clientId}/engagement-requests", false, RequestHandler(db, nudge))
	// Where pending Requests gather (#503) -- the same Owner/Admin seat,
	// registered before the {requestId} read so the two paths read in the
	// order a person meets them.
	g.Get("/api/practices/{practiceId}/engagement-requests", staffauth.OwnerAndAdmin, ListHandler())
	g.Get("/api/practices/{practiceId}/engagement-requests/{requestId}", staffauth.OwnerAndAdmin, DetailHandler())
	ir.Exempt("POST /api/practices/{practiceId}/engagement-requests/{requestId}/approve",
		"approve() locks the Request FOR UPDATE and checks state = pending inside the same transaction; a retry after the first commit finds it already decided and 409s instead of creating a second Engagement or spending a second Credit",
		false, ApproveHandler(db, nudge))
	ir.Exempt("POST /api/practices/{practiceId}/engagement-requests/{requestId}/refuse",
		"state-guarded UPDATE ... WHERE state = 'pending'; a retry after the first commit affects zero rows and 409s instead of refusing twice",
		false, RefuseHandler())
	ir.Exempt("POST /api/practices/{practiceId}/engagement-requests/{requestId}/withdraw",
		"state-guarded UPDATE ... WHERE requested_by = $1 AND state = 'pending'; a retry after the first commit affects zero rows and 409s instead of withdrawing twice",
		false, WithdrawHandler())
}
