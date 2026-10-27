package plans

import (
	"database/sql"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Plan Template and Plan Instance surface, the Birth
// Plan's rendered PDF (#306, Birth Plan only -- Care Plan has no
// Client-facing surface for it to mirror), plus the Client portal's own
// birth-plan read, acknowledge, and PDF.
//
// The Plan Template's read and write differ. The GET is open to every
// Staff role (ADR-0008's read table, its "Plan Template and Contract
// Template" row), with no attachment narrowing -- a Template isn't
// Engagement-scoped. The PUT is Owner-only (not Admin), the same rule as
// its Contract Template sibling in contracts.Mount.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB) {
	g.Get("/api/practices/{practiceId}/plan-templates/{planType}", staffauth.AnyStaff, GetTemplateHandler())
	// Owner-only (not Admin): #1407 moved this rule from an in-handler
	// check (reader.Has("owner")) to this declaration without widening
	// it, registered through ir.ExemptGated, which panics if the role list
	// is ever left empty -- PutTemplateHandler itself no longer checks the
	// caller's role at all.
	ir.ExemptGated("PUT /api/practices/{practiceId}/plan-templates/{planType}",
		"upsert (ON CONFLICT ... DO UPDATE); replaces the template wholesale, so re-sending the same body is a no-op",
		false, staffauth.OwnerOnly, PutTemplateHandler())
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
	ir.PortalExempt("POST /api/portal/engagements/{engagementId}/birth-plan/acknowledge",
		"re-acknowledging only refreshes client_acknowledged_at on the one row; a retry adds nothing a duplicate could corrupt",
		clientauth.Middleware(db)(ClientAcknowledgeBirthPlanHandler()))
}
