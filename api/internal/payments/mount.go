package payments

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers Stripe Connect account creation and status, per-Engagement
// Invoice creation and history, and the Practice-wide Invoice list (#265).
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, client Client) {
	ir.Exempt("POST /api/practices/{practiceId}/payments/connect",
		"lazily creates the Stripe Connect account and reuses the stored account id on any retry, row-locked against a concurrent create; a duplicate call resumes the same account, not a second one",
		false, PostConnectHandler(client))
	// Connect state rides the same row as the money it carries: ADR-0008's
	// read table gives "Stripe Connect state" to an Owner and an Admin and
	// to nobody else (#267), for the reason it already gives Invoice
	// history and the Credit ledger the same pair -- an Admin covers the
	// business side of a Practice, and this is the state of the rail her
	// Invoices are paid on. Reading is Owner-or-Admin; starting or
	// resuming hosted onboarding stays Owner-only above.
	g.Get("/api/practices/{practiceId}/payments/connect", staffauth.OwnerAndAdmin, GetConnectStatusHandler(client))
	// Newly wrapped (2026 idempotency-stance review): every call
	// unconditionally calls Stripe CreateInvoice + FinalizeInvoice and
	// inserts a new invoices row, with no dedup guard -- a double-click
	// billed the Client twice. Money-creating, same as the six routes
	// already wrapped below.
	//
	// attaching=true (#947): raising an Invoice stays open to any Staff
	// with reach -- no role gate, matching Contract's own default (#68)
	// -- but it never carried the attaching-write reach test every other
	// Engagement-scoped write does (ADR-0008), so a contractor with no
	// granted attachment could raise an Invoice on any Engagement at the
	// Practice. AttachingWrite closes that: an unattached contractor now
	// 404s the same way she already does on a Contract write.
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/invoices", true, PostInvoiceHandler(client))
	// Invoice history: Owner, Admin, and an employed Doula (ADR-0008's
	// money row as amended by #282); GetInvoicesHandler refuses a
	// contractor in-handler, so the mount stays AnyStaff. A contractor's
	// own-fee narrowing (rather than an outright no) is #317's to build
	// once the Offer/Attachment flow exists.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract/invoices", staffauth.AnyStaff, GetInvoicesHandler())
	// The Practice-wide Invoice list (#265): every Invoice the Practice
	// has billed, with the whole book's outstanding and paid totals, so
	// "who owes us money" is one screen rather than every Engagement
	// opened in turn. Same rule as per-Engagement Invoice history above --
	// Owner, Admin, and an employed Doula, refused in-handler for a
	// contractor. A contractor's own-fee narrowing has nothing to narrow
	// here -- an aggregate of the Practice's whole book is not a view of
	// her own Engagements -- so it stays where the per-Engagement Contract
	// read already puts it.
	g.Get("/api/practices/{practiceId}/invoices", staffauth.AnyStaff, GetPracticeInvoicesHandler())

	// Billing mode (#271): a Practice-level choice between billing through
	// Stripe and billing by hand. Reading is any Staff -- a Doula meets
	// this fact on the Invoice section, the same reasoning #270 already
	// applies to "whether the Practice can raise an Invoice at all".
	// Writing an already-established mode is Owner-only, by analogy to
	// Connect onboarding; the one-time initial set instead rides
	// PostInvoiceHandler's own request (billing_mode.go's own comment).
	// PUT is a full replacement, inherently idempotent (docs/api-design.md
	// section 3), so it goes through ExemptGated rather than Replayable.
	// Owner-only is declared here (#990, following #970's own move for
	// Contract writes): PutBillingModeHandler no longer calls
	// staffauth.RequireOwner itself.
	g.Get("/api/practices/{practiceId}/payments/billing-mode", staffauth.AnyStaff, GetBillingModeHandler())
	ir.ExemptGated("PUT /api/practices/{practiceId}/payments/billing-mode",
		"full-replacement PUT, inherently idempotent (docs/api-design.md section 3); no Idempotency-Key applies",
		false, staffauth.OwnerOnly, PutBillingModeHandler())

	// Payment terms (#768): how many days after an Invoice is raised it
	// falls due, and so what the Practice's own book calls late. Reading
	// is any Staff, for the reason billing mode above gives -- a Doula
	// meets the due date on every Invoice she looks at. Writing is Owner
	// and Admin, the pair #282's write table already gives every other
	// money decision; declared here, not in-handler (#990). PUT is a full
	// replacement, inherently idempotent (docs/api-design.md section 3),
	// so ExemptGated rather than Replayable.
	g.Get("/api/practices/{practiceId}/payments/payment-terms", staffauth.AnyStaff, GetPaymentTermsHandler())
	ir.ExemptGated("PUT /api/practices/{practiceId}/payments/payment-terms",
		"full-replacement PUT, inherently idempotent (docs/api-design.md section 3); no Idempotency-Key applies",
		false, staffauth.OwnerAndAdmin, PutPaymentTermsHandler())

	// Recording a Payment that did not come through Stripe (#271): Owner
	// and Admin only -- narrower than the read above, which #282 opened
	// to an employed Doula too. Reading a number and being allowed to set
	// one are different things (#282's own write table): only an Owner or
	// Admin records a Payment, overrides an amount, or voids/writes off
	// an Invoice, regardless of who may read it. Money-creating, so
	// Replayable like Invoice creation above: a double-click must not
	// record the same check twice. #990 moved the Owner-or-Admin rule from
	// an in-handler check to this declaration, through ReplayableGated --
	// PostManualPaymentHandler no longer calls
	// staffauth.RequireOwnerOrAdmin itself.
	ir.ReplayableGated("POST /api/practices/{practiceId}/invoices/{invoiceId}/payments", false, staffauth.OwnerAndAdmin, PostManualPaymentHandler(client))

	// Reversing a manually recorded Payment (#945): Owner and Admin only,
	// the same gate recording one already carries -- an additive row in
	// the same append-only payments table, never an UPDATE or DELETE.
	// Money-moving, so Replayable like recording one: a double-click must
	// not reverse the same Payment twice (resolvePaymentForReversal's own
	// already-reversed check would 409 the retry regardless, but the
	// Idempotency-Key still replays the first response rather than
	// re-running the check). Refused entirely against a Stripe-backed
	// Invoice -- see PostReversePaymentHandler's own doc comment -- so no
	// Stripe client call is threaded through here.
	ir.ReplayableGated("POST /api/practices/{practiceId}/invoices/{invoiceId}/payments/{paymentId}/reverse", false, staffauth.OwnerAndAdmin, PostReversePaymentHandler())

	// Void and write-off (#271) exist only so a by-hand Invoice -- which
	// nothing else in the model ever moves out of 'open' -- is not stuck
	// there forever on a mistyped amount. Both are state-guarded
	// transitions, refused unless the Invoice is still 'open' and by-hand,
	// so a retry after the first success 409s rather than repeating the
	// effect -- ExemptGated, not Replayable. #990 moved both handlers'
	// shared Owner-or-Admin check (transitionByHandInvoice) from in-handler
	// to these two declarations.
	ir.ExemptGated("POST /api/practices/{practiceId}/invoices/{invoiceId}/void",
		"refuses unless the Invoice is open and by-hand, so a retry 409s instead of voiding twice",
		false, staffauth.OwnerAndAdmin, PostVoidInvoiceHandler())
	ir.ExemptGated("POST /api/practices/{practiceId}/invoices/{invoiceId}/write-off",
		"refuses unless the Invoice is open and by-hand, so a retry 409s instead of writing off twice",
		false, staffauth.OwnerAndAdmin, PostWriteOffInvoiceHandler())
}
