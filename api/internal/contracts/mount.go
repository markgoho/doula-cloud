package contracts

import (
	"database/sql"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Contract Template, the Practice-wide
// "awaiting-signature" roll-up (#426), the per-Engagement Contract's
// scope-vs-money split (ADR-0008) and its lifecycle writes, the Signed
// PDF, and the Client portal's own contract read, sign, and PDF.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB, store objectstore.ObjectStore, pusher push.Pusher) {
	g.Get("/api/practices/{practiceId}/contract-template", staffauth.AnyStaff, GetTemplateHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/contract-template",
		"upsert (ON CONFLICT ... DO UPDATE); replaces the template wholesale, so re-sending the same body is a no-op",
		false, PutTemplateHandler())
	// The Practice-wide "Contracts awaiting signature" roll-up (#426):
	// every Draft or Sent Contract at the Practice in one read, so
	// chasing signatures is one screen rather than every Engagement
	// opened in turn. Owner and Admin, the same declaration the credit
	// balance and the Practice-wide Invoice list carry.
	g.Get("/api/practices/{practiceId}/contracts/awaiting-signature", staffauth.OwnerAndAdmin, AwaitingSignatureHandler())
	ir.Exempt("POST /api/practices/{practiceId}/engagements/{engagementId}/contract",
		"guarded by contracts' unique constraint on engagement_id; a retry after the first succeeds hits the constraint and 409s rather than creating a duplicate Contract",
		true, PostContractHandler())
	// Contract read is the sharpest #231 case: scope reaches every role
	// (narrowed by attachment for a contractor, same as above), but money
	// -- and Invoice history -- is Owner/Admin only, never a Doula's,
	// employee or contractor. GetContractHandler does the scope-vs-money
	// split itself via staffauth.Reader + ContractScope/ContractFull, so
	// the mount stays AnyStaff.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract", staffauth.AnyStaff, GetContractHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/engagements/{engagementId}/contract",
		"full-replace UPDATE of the Contract's merge field values; re-sending the same body is a no-op",
		true, PutContractHandler())
	ir.Exempt("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/send",
		"state-guarded (status != 'draft' -> 409); a retry after the first commit finds the Contract already sent and 409s instead of pushing the Client notification twice",
		true, PostSendContractHandler(pusher))
	ir.Exempt("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void",
		"state-guarded (status != 'signed' -> 409); a retry after the first commit 409s instead of voiding twice",
		true, PostVoidContractHandler())
	// The Signed PDF is a rendered, unredactable document -- it can't be
	// split into scope/money views the way the JSON Contract read can, so
	// it follows the money row: Owner/Admin only.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract/pdf", staffauth.OwnerAndAdmin, GetSignedContractPDFHandler(store))

	g.OpenGet("/api/portal/engagements/{engagementId}/contract", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ClientGetContractHandler()))
	g.Write("POST /api/portal/engagements/{engagementId}/contract/sign",
		clientauth.Middleware(db)(ClientPostSignContractHandler(store)))
	g.OpenGet("/api/portal/engagements/{engagementId}/contract/pdf", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ClientGetSignedContractPDFHandler(store)))
}
