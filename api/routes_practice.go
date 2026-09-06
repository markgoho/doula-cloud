package main

import (
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/clientfieldtemplate"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/pushsub"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/visit"
	"doula-cloud/api/internal/website"
)

// Everything under /api/practices: the Staff roster, billing, Stripe
// Connect, the website answer, Clients, Engagements and their Visits,
// Messages, Plans, Contracts, Invoices and Offers.
//
// #836 collapsed this file from 41 individual registrations, each
// hand-assembling staffauth.Middleware(d.DB)(...) and, where the route
// mutates, idempotency.Wrap(...) in the right order, into one call per
// feature package. Each package's own Mount now names only
// Replayable-or-Exempt, attaching-or-not, and roles; idempotency.Router
// applies Middleware and Wrap itself (see internal/idempotency/router.go).
//
// staffauth.Mount is not called from here: it registers its own
// Practice-scoped routes alongside the pre-session ones it also owns
// (sign-up, sign-in, invitation acceptance), so it is called once from
// registerSessionRoutes instead -- see that file's own comment.
func registerPracticeRoutes(g *staffauth.GatedRouter, ir *idempotency.Router, d Deps) {
	activityfeed.Mount(g)
	billing.Mount(g, ir, d.StripeClient)
	payments.Mount(g, ir, d.PaymentsClient)
	website.Mount(g, ir, d.NudgeEnqueuer)
	client.Mount(g, ir, d.NudgeEnqueuer)
	engagementrequest.Mount(g, ir, d.DB, d.NudgeEnqueuer)
	engagement.Mount(g, ir)
	visit.Mount(g, ir)
	message.Mount(g, ir, d.DB, d.Store, d.Pusher)
	plans.Mount(g, ir, d.DB)
	clientfieldtemplate.Mount(g, ir)
	contracts.Mount(g, ir, d.DB, d.Store, d.Pusher)
	offer.Mount(g, ir, d.DB, d.NudgeEnqueuer)
	pushsub.Mount(g, ir, d.DB)
	portalinvite.Mount(g, ir, d.DB, d.NudgeEnqueuer)
	mailsuppress.Mount(g, ir, d.BounceClearer)
}
