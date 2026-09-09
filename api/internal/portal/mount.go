package portal

import (
	"database/sql"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Client portal's own Engagement detail read, #486
// AC4/AC5's activity ledger, behind a closed disclosure (the design
// brief's own placement decision), and #478's "Your visits".
//
// The Visit read lives here rather than in visit.Mount, unlike the
// pairs #836 moved into their feature package: its DTO shares nothing
// with the Staff-side visit.Visit but the id -- no staff_id, no notes,
// no derived type -- so there is no one record's reader to keep in one
// place. What it does share is this package's own subject: the
// Engagement the signed-in Client is on, read off the context rather
// than the path.
func Mount(g *staffauth.GatedRouter, db *sql.DB) {
	g.OpenGet("/api/portal/engagements/{engagementId}", clientauth.PortalPopulation,
		clientauth.Middleware(db)(DetailHandler()))
	g.OpenGet("/api/portal/engagements/{engagementId}/activity", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ActivityHandler()))
	g.OpenGet("/api/portal/engagements/{engagementId}/visits", clientauth.PortalPopulation,
		clientauth.Middleware(db)(VisitsHandler()))
}
