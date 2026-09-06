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
	// ADR-0008's read table has no row for Stripe Connect state; mirroring
	// the write side's Owner-only gate (PostConnectHandler,
	// staffauth.RequireOwner) is the narrowest defensible default until a
	// real rule lands (#267 stays open for that rule).
	g.Get("/api/practices/{practiceId}/payments/connect", staffauth.OwnerOnly, GetConnectStatusHandler(client))
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
}
