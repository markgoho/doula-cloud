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
// Every route here is a write, and since #1924 the portal ones register
// through ir too, so the GatedRouter is unused -- kept in the signature
// that every feature Mount shares.
func Mount(_ *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB) {
	ir.Exempt("POST /api/practices/{practiceId}/push-subscriptions",
		"upsert; registering the same endpoint again is a no-op update, not a duplicate row",
		false, RegisterHandler())
	ir.Exempt("DELETE /api/practices/{practiceId}/push-subscriptions",
		"delete; a retry after the first succeeds deletes nothing further",
		false, UnregisterHandler())

	ir.PortalExempt("POST /api/portal/engagements/{engagementId}/push-subscriptions",
		"upsert; registering the same endpoint again is a no-op update, not a duplicate row",
		clientauth.Middleware(db)(ClientRegisterHandler()))
	ir.PortalExempt("DELETE /api/portal/engagements/{engagementId}/push-subscriptions",
		"delete; a retry after the first succeeds deletes nothing further",
		clientauth.Middleware(db)(ClientUnregisterHandler()))
}
