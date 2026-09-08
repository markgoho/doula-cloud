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
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/contract/invoices", false, PostInvoiceHandler(client))
	// Invoice history rides the same money row as Contract money -- see
	// above. A contractor's own-fee narrowing (rather than an outright
	// no) is #317's to build once the Offer/Attachment flow exists.
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/contract/invoices", staffauth.OwnerAndAdmin, GetInvoicesHandler())
	// The Practice-wide Invoice list (#265): every Invoice the Practice
	// has billed, with the whole book's outstanding and paid totals, so
	// "who owes us money" is one screen rather than every Engagement
	// opened in turn. A contractor's own-fee narrowing has nothing to
	// narrow here -- an aggregate of the Practice's whole book is not a
	// view of her own Engagements -- so it stays where the per-Engagement
	// Contract read already puts it.
	g.Get("/api/practices/{practiceId}/invoices", staffauth.OwnerAndAdmin, GetPracticeInvoicesHandler())

	// Billing mode (#271): a Practice-level choice between billing through
	// Stripe and billing by hand. Reading is any Staff -- a Doula meets
	// this fact on the Invoice section, the same reasoning #270 already
	// applies to "whether the Practice can raise an Invoice at all".
	// Writing an already-established mode is Owner-only, by analogy to
	// Connect onboarding; the one-time initial set instead rides
	// PostInvoiceHandler's own request (billing_mode.go's own comment).
	// PUT is a full replacement, inherently idempotent (docs/api-design.md
	// section 3), so it goes through Exempt rather than Replayable.
	g.Get("/api/practices/{practiceId}/payments/billing-mode", staffauth.AnyStaff, GetBillingModeHandler())
	ir.Exempt("PUT /api/practices/{practiceId}/payments/billing-mode",
		"full-replacement PUT, inherently idempotent (docs/api-design.md section 3); no Idempotency-Key applies",
		false, PutBillingModeHandler())

	// Recording a Payment that did not come through Stripe (#271): Owner
	// and Admin only, matching ADR-0008's Contract-money read row and this
	// Mount's own Owner/Admin invoice-history routes above -- a write
	// gated more loosely than the read of the same data is the harder
	// position to defend. Money-creating, so Replayable like Invoice
	// creation above: a double-click must not record the same check twice.
	ir.Replayable("POST /api/practices/{practiceId}/invoices/{invoiceId}/payments", false, PostManualPaymentHandler(client))

	// Void and write-off (#271) exist only so a by-hand Invoice -- which
	// nothing else in the model ever moves out of 'open' -- is not stuck
	// there forever on a mistyped amount. Both are state-guarded
	// transitions, refused unless the Invoice is still 'open' and by-hand,
	// so a retry after the first success 409s rather than repeating the
	// effect -- Exempt, not Replayable.
	ir.Exempt("POST /api/practices/{practiceId}/invoices/{invoiceId}/void",
		"refuses unless the Invoice is open and by-hand, so a retry 409s instead of voiding twice",
		false, PostVoidInvoiceHandler())
	ir.Exempt("POST /api/practices/{practiceId}/invoices/{invoiceId}/write-off",
		"refuses unless the Invoice is open and by-hand, so a retry 409s instead of writing off twice",
		false, PostWriteOffInvoiceHandler())
}
