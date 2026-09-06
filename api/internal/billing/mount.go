package billing

import (
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
)

// Mount registers the Practice's Credit balance and ledger (Owner and
// Admin only, ADR-0008) and the Checkout Session purchase kicks off.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, stripeClient StripeClient) {
	g.Get("/api/practices/{practiceId}/billing", staffauth.OwnerAndAdmin, GetBalanceHandler())
	ir.Exempt("POST /api/practices/{practiceId}/billing/purchases",
		"creates a Stripe Checkout Session URL only; the ledger is credited by the purchase webhook against the actual completed payment, so a duplicate call yields an extra unused Checkout Session, never a double charge or double credit",
		false, PostPurchaseHandler(stripeClient))
}
