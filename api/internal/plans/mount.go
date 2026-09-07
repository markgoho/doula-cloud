package plans

import (
	"database/sql"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Plan Template (every Staff role, ADR-0008, no
// attachment narrowing -- a Template isn't Engagement-scoped) and Plan
// Instance surface, the Birth Plan's rendered PDF (#306, Birth Plan only
// -- Care Plan has no Client-facing surface for it to mirror), plus the
// Client portal's own birth-plan read, acknowledge, and PDF.
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
	// Rendered fresh on every request, never stored (#306): a Plan
	// Instance has no "final" event the way a signed Contract does, so
	// there is nothing to cache and no staleness to manage server-side.
	// Mounted generically over :planType (same pattern as the routes
	// above) so the URL a Practice-side download button calls matches the
	// JSON route it already reads, but GetBirthPlanPDFHandler itself
	// refuses anything but birth_plan.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}/pdf", staffauth.AnyStaff, GetBirthPlanPDFHandler())

	g.OpenGet("/api/portal/engagements/{engagementId}/birth-plan", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ClientGetBirthPlanHandler()))
	g.OpenGet("/api/portal/engagements/{engagementId}/birth-plan/pdf", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ClientGetBirthPlanPDFHandler()))
	// No idempotency.Wrap: idempotency.Router keys retries off a Staff
	// id, which a Client-portal request never carries (see
	// contracts.Mount's own client/contract/sign route, the same shape).
	// Re-acknowledging is harmless anyway -- it only refreshes
	// client_acknowledged_at, never anything a duplicate would corrupt.
	g.Write("POST /api/portal/engagements/{engagementId}/birth-plan/acknowledge",
		clientauth.Middleware(db)(ClientAcknowledgeBirthPlanHandler()))
}
