package notificationpref

import (
	"database/sql"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers #303's durable, reviewable push preference: GET reads
// current status, PUT turns it on or off. PUT is naturally idempotent
// (repeating the same {enabled} body re-asserts the same state), so no
// Idempotency-Key handling applies here per docs/api-design.md section 3.
func Mount(g *staffauth.GatedRouter, db *sql.DB) {
	g.OpenGet("/api/portal/engagements/{engagementId}/notification-preference", clientauth.PortalPopulation,
		clientauth.Middleware(db)(GetHandler()))
	g.Write("PUT /api/portal/engagements/{engagementId}/notification-preference",
		clientauth.Middleware(db)(SetHandler()))
}
