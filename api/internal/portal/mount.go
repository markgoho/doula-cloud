package portal

import (
	"database/sql"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Client portal's own Engagement detail read and
// #486 AC4/AC5's activity ledger, behind a closed disclosure (the design
// brief's own placement decision).
func Mount(g *staffauth.GatedRouter, db *sql.DB) {
	g.OpenGet("/api/portal/engagements/{engagementId}", clientauth.PortalPopulation,
		clientauth.Middleware(db)(DetailHandler()))
	g.OpenGet("/api/portal/engagements/{engagementId}/activity", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ActivityHandler()))
}
