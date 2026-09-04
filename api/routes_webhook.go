package main

import (
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/staffauth"
)

// What Stripe and Mailgun call.
//
// Outside every session middleware, like the internal endpoints, but
// authenticated differently: each verifies a signature over the request
// body against its own secret. They are inside csrf.Wrap all the same --
// it exempts a request with no Origin header, which is what these are.
func registerWebhookRoutes(g *staffauth.GatedRouter, d Deps) {
	g.Write("POST /api/stripe/webhook", billing.PostPurchaseWebhookHandler(d.DB, d.StripeWebhookSecret))
	g.Write("POST /api/stripe/connect-webhook", payments.PostConnectWebhookHandler(d.DB, d.PaymentsClient, d.PaymentsWebhookSecret, d.NudgeEnqueuer))
	// A second Connect route, not a second feature: Stripe's v2 account
	// events are thin and a destination carries one payload type, so they
	// cannot share connect-webhook's endpoint or its secret (#247).
	g.Write("POST /api/stripe/account-webhook", payments.PostAccountWebhookHandler(d.DB, d.PaymentsClient, d.PaymentsAccountWebhookSecret, d.NudgeEnqueuer))
	// #340/ADR-0010: Mailgun's bounce/complaint delivery-event webhook,
	// same no-staffauth shape as the Stripe webhooks above -- signature
	// verified instead of a session.
	g.Write("POST /api/mailgun/webhook", portalinvite.PostBounceWebhookHandler(d.DB, d.MailgunWebhookSigningKey))
}
