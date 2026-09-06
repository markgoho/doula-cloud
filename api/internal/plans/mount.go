package plans

import (
	"database/sql"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Plan Template (every Staff role, ADR-0008, no
// attachment narrowing -- a Template isn't Engagement-scoped) and Plan
// Instance surface, plus the Client portal's own birth-plan read.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB) {
	g.Get("/api/practices/{practiceId}/plan-templates/{planType}", staffauth.AnyStaff, GetTemplateHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/plan-templates/{planType}",
		"upsert (ON CONFLICT ... DO UPDATE); replaces the template wholesale, so re-sending the same body is a no-op",
		false, PutTemplateHandler())
	ir.Exempt("POST /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		"guarded by plan_instances' unique constraint on (engagement_id, plan_type); a retry after the first succeeds hits the constraint and 409s rather than creating a duplicate Plan Instance",
		true, PostInstanceHandler())
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}", staffauth.AnyStaff, GetInstanceHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}",
		"full-replace UPDATE of the Plan Instance's answers; re-sending the same body is a no-op",
		true, PutInstanceHandler())

	g.OpenGet("/api/portal/engagements/{engagementId}/birth-plan", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ClientGetBirthPlanHandler()))
}
