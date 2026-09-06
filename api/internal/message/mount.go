package message

import (
	"database/sql"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers both populations' Message surface: the Staff thread
// under an Engagement (behind staffauth.Middleware via ir) and the
// Practice-wide "waiting on a reply" roll-up (#455), and the Client
// portal's own thread (behind clientauth.Middleware).
//
// staffauth.AttachingWrite is ADR-0008's write-side seam: CreateHandler
// attaches the acting Doula, accrued, once it has succeeded -- newly
// wrapped Replayable in the 2026 idempotency-stance review, since it is
// an unconditional INSERT with no uniqueness guard.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB, store objectstore.ObjectStore, pusher push.Pusher) {
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/messages", staffauth.AnyStaff, ListHandler())
	// The Practice-wide "waiting on a reply" roll-up (#455): every
	// Engagement whose thread's latest Message came from the Client, so a
	// doula sees who is waiting without opening every Engagement in turn.
	// AnyStaff, the same as the thread read above -- the contractor's
	// attachment narrowing is enforced inside the handler's own query, not
	// at this mount, mirroring ListHandler's own CanAccessEngagement
	// narrowing rather than a role declaration.
	g.Get("/api/practices/{practiceId}/messages/awaiting-reply", staffauth.AnyStaff, AwaitingReplyHandler())
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/messages", true, CreateHandler(store, pusher))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/messages/{messageId}/attachment", staffauth.AnyStaff, AttachmentHandler(store))

	g.OpenGet("/api/portal/engagements/{engagementId}/messages", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ClientListHandler()))
	g.Write("POST /api/portal/engagements/{engagementId}/messages",
		clientauth.Middleware(db)(ClientCreateHandler(store, pusher)))
	g.OpenGet("/api/portal/engagements/{engagementId}/messages/{messageId}/attachment", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ClientAttachmentHandler(store)))
}
