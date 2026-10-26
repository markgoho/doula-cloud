package notificationpref

import (
	"database/sql"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers #303's durable, reviewable push preference: GET reads
// current status, PUT turns it on or off. PUT is naturally idempotent
// (repeating the same {enabled} body re-asserts the same state), so no
// Idempotency-Key handling applies here per docs/api-design.md section 3
// -- declared at the route through ir.PortalExempt (#1924).
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB) {
	g.OpenGet("/api/portal/engagements/{engagementId}/notification-preference", clientauth.PortalPopulation,
		clientauth.Middleware(db)(GetHandler()))
	ir.PortalExempt("PUT /api/portal/engagements/{engagementId}/notification-preference",
		"PUT; repeating the same {enabled} body re-asserts the same state",
		clientauth.Middleware(db)(SetHandler()))
}
