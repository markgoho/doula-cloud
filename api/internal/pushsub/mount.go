package pushsub

import (
	"database/sql"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers both populations' push-subscription surface: the Staff
// one under a Practice (behind staffauth.Middleware via ir), and the
// Client-portal one under an Engagement (behind clientauth.Middleware).
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB) {
	ir.Exempt("POST /api/practices/{practiceId}/push-subscriptions",
		"upsert; registering the same endpoint again is a no-op update, not a duplicate row",
		false, RegisterHandler())
	ir.Exempt("DELETE /api/practices/{practiceId}/push-subscriptions",
		"delete; a retry after the first succeeds deletes nothing further",
		false, UnregisterHandler())

	g.Write("POST /api/portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(db)(ClientRegisterHandler()))
	g.Write("DELETE /api/portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(db)(ClientUnregisterHandler()))
}
