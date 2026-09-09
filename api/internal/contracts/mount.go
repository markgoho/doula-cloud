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
// "awaiting-signature" and "void requests awaiting" roll-ups (#426,
// #971), the per-Engagement Contract's scope-vs-money split (ADR-0008)
// and its lifecycle writes, the Signed PDF, and the Client portal's own
// contract read, sign, and PDF.
//
// Every Contract write below carries a role declaration at the mount
// (#970), registered through ir.ExemptGated rather than the role-free
// ir.Exempt, which panics if the role list is ever left empty, the same
// guarantee a GET's role list already carries. Create, set values, send
// and #971's own void-request are staffauth.AnyStaff -- #282's write
// table gives them to whoever reaches the Engagement at all (Owner,
// Admin, an employed Doula, or a contractor on a granted attachment), so
// the mount adds no role restriction beyond the AttachingWrite reach
// test attaching=true already applies. Void, the void-request decline,
// and the Template's own write are narrower -- OwnerAndAdmin and
// OwnerOnly.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB, store objectstore.ObjectStore, pusher push.Pusher) {
	g.Get("/api/practices/{practiceId}/contract-template", staffauth.AnyStaff, GetTemplateHandler())
	// Owner-only (not Admin): #970 moved this rule from an in-handler
	// check (reader.Has("owner")) to this declaration without widening
	// it -- PutTemplateHandler itself no longer checks the caller's role
	// at all.
	ir.ExemptGated("PUT /api/practices/{practiceId}/contract-template",
		"upsert (ON CONFLICT ... DO UPDATE); replaces the template wholesale, so re-sending the same body is a no-op",
		false, staffauth.OwnerOnly, PutTemplateHandler())
	// The Practice-wide "Contracts awaiting signature" roll-up (#426):
	// every Draft or Sent Contract at the Practice in one read, so
	// chasing signatures is one screen rather than every Engagement
	// opened in turn. AwaitingItem carries no amount, so this is not a
	// money read and does not carry the credit balance / Invoice-list
	// declaration; it follows ADR-0008's Engagements/Visits/Messages row
	// instead, the same as message's own "awaiting-reply" roll-up below --
	// AnyStaff, with the contractor's attachment narrowing enforced
	// inside the handler's own query (#973).
	g.Get("/api/practices/{practiceId}/contracts/awaiting-signature", staffauth.AnyStaff, AwaitingSignatureHandler())
	// #971: the mirror roll-up for a void a Doula has asked for but
	// nobody has decided yet -- same reach as the row above, since void
	// itself is Owner/Admin-only (#970).
	g.Get("/api/practices/{practiceId}/contracts/void-requests", staffauth.OwnerAndAdmin, VoidRequestsAwaitingHandler())
	ir.ExemptGated("POST /api/practices/{practiceId}/engagements/{engagementId}/contract",
		"guarded by contracts' unique constraint on engagement_id; a retry after the first succeeds hits the constraint and 409s rather than creating a duplicate Contract",
		true, staffauth.AnyStaff, PostContractHandler())
	// Contract read (narrowed by attachment for a contractor, same as
	// above): #282 retired the scope-vs-money split #231 built here, so
	// GetContractHandler now returns the whole Contract to any reader who
	// reaches the Engagement at all. The mount stays AnyStaff.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract", staffauth.AnyStaff, GetContractHandler())
	ir.ExemptGated("PUT /api/practices/{practiceId}/engagements/{engagementId}/contract",
		"full-replace UPDATE of the Contract's merge field values; re-sending the same body is a no-op",
		true, staffauth.AnyStaff, PutContractHandler())
	// #967: overriding a Contract's rate-card-derived amount is Owner and
	// Admin only. #970 moved every Contract write's role to the mount, so
	// the declaration lives here rather than inside the handler. Not
	// attaching -- Owner and Admin bypass Attachment entirely.
	ir.ExemptGated("PUT /api/practices/{practiceId}/engagements/{engagementId}/contract/amount",
		"full-replace UPDATE of the Contract's amount; a retry with the same value changes nothing and records nothing new",
		false, staffauth.OwnerAndAdmin, PutContractAmountHandler())
	ir.ExemptGated("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/send",
		"state-guarded (status != 'draft' -> 409); a retry after the first commit finds the Contract already sent and 409s instead of pushing the Client notification twice",
		true, staffauth.AnyStaff, PostSendContractHandler(pusher))
	// Owner and Admin only (#282, #970): every Doula, employee or
	// contractor, is refused regardless of attachment, so this is exactly
	// the case AttachingWrite's reach test cannot express -- reach and
	// role are two different questions here.
	ir.ExemptGated("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void",
		"state-guarded (status != 'signed' -> 409); a retry after the first commit 409s instead of voiding twice",
		true, staffauth.OwnerAndAdmin, PostVoidContractHandler())
	// #971: a Doula who reaches the Engagement at all -- the same AnyStaff
	// reach Create/Send/Edit already carry -- asks for a Signed Contract
	// to be voided instead of voiding it herself, which #970 refuses her.
	// State-guarded (status != 'signed' -> 409) and guarded again by the
	// one-open-per-requester unique index (-> 409), so a retry after the
	// first commit never opens a second request.
	ir.ExemptGated("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void-request",
		"state-guarded (status != 'signed' -> 409) and unique-constraint-guarded (one open request per requester -> 409)",
		true, staffauth.AnyStaff, PostVoidRequestHandler())
	// The other half of #971's ask: an Owner or an Admin declines one
	// specific open request. Owner and Admin only, same as Void itself;
	// attaching=true the same way Void's own mount line is -- an Owner or
	// Admin reaches every Engagement regardless, so AttachingWrite's
	// reach test is always satisfied for them and never mints an
	// accrued attachment (attachActor is a no-op for anyone but a plain
	// Doula), but registering it keeps write_gate_guardrail_test.go's
	// "every mutating {engagementId} route accounted for" sweep honest
	// without a second exemption entry.
	ir.ExemptGated("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/void-request/{requestId}/decline",
		"state-guarded (status != 'open' -> 404, naming a request already decided or unknown)",
		true, staffauth.OwnerAndAdmin, PostVoidRequestDeclineHandler())
	// The Signed PDF is a rendered, unredactable document -- it can't be
	// split into scope/money views the way the JSON Contract read can, so
	// it follows the money row wholesale: Owner, Admin, and an employed
	// Doula (ADR-0008 as amended by #282), refused in-handler for a
	// contractor since her fee is never on this document at all. Mount
	// stays AnyStaff; GetSignedContractPDFHandler enforces the refusal.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract/pdf", staffauth.AnyStaff, GetSignedContractPDFHandler(store))

	g.OpenGet("/api/portal/engagements/{engagementId}/contract", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ClientGetContractHandler()))
	g.Write("POST /api/portal/engagements/{engagementId}/contract/sign",
		clientauth.Middleware(db)(ClientPostSignContractHandler(store)))
	g.OpenGet("/api/portal/engagements/{engagementId}/contract/pdf", clientauth.PortalPopulation,
		clientauth.Middleware(db)(ClientGetSignedContractPDFHandler(store)))
}
