package main

import (
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/notificationpref"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/pushsub"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/staffauth"
)

// The Client's own surface, behind clientauth rather than staffauth.
//
// portalPopulation is why every read here is an OpenGet rather than a
// Get: ADR-0008's role table describes Staff at a Practice, and a Client
// holds no Membership to check against, so there is nothing for a role
// declaration to be about. These used to be listed in the guardrail
// test's own exemptGETRoutes map; the reason now sits at the mount.
const portalPopulation = "clientauth.Middleware, not staffauth -- a Client holds no Membership, so ADR-0008's read table has nothing to say about this route"

// A different population with a different session, which is why none of
// these reads go through GatedRouter.Get.
func registerPortalRoutes(g *staffauth.GatedRouter, d Deps) {
	// #617: a Client has no Identity Platform account to authenticate
	// through any more, so the invitation token is the whole credential --
	// tokenSpendRules' shape, not bootstrapRules', keyed on inviteToken
	// rather than a Bearer token there is none of.
	g.Write("POST /api/portal/accept-invite",
		ratelimit.Wrap(d.DB, "portal_accept_invite", portalAcceptInviteRules)(portalinvite.AcceptInviteHandler(d.DB, d.NudgeEnqueuer)))
	// #617: request and redeem a Client sign-in link, ADR-0026's magic
	// link. Same shape as staffauth's own reset request/spend pair --
	// resetRequestRules is #602's own sizing note for this exact endpoint,
	// reused verbatim.
	g.Write("POST /api/portal/magic-link/request",
		ratelimit.Wrap(d.DB, "portal_magic_link_request", resetRequestRules)(clientauth.RequestMagicLinkHandler(d.DB)))
	g.Write("POST /api/portal/magic-link",
		ratelimit.Wrap(d.DB, "portal_magic_link", tokenSpendRules)(clientauth.RedeemMagicLinkHandler(d.DB, d.NudgeEnqueuer)))
	// #619: she changes her own sign-in address, proved by a link to the
	// new one. The request is authenticated by her live portal session
	// and the spend is not -- the confirmation link is read in the new
	// mailbox, which may be on a device she has never signed in on -- so
	// the two carry different rate-limit shapes, and neither is
	// Engagement-scoped: a Portal Account reaches Clients at more than
	// one Practice (ADR-0015), so this is not clientauth.Middleware's
	// surface. Not idempotency-keyed: the request mints a fresh token and
	// resets the pending outbox row rather than sending a second mail
	// (docs/api-design.md section 3), and the spend's own single-use
	// token already makes a repeat a no-op.
	g.Write("POST /api/portal/sign-in-address/request",
		ratelimit.Wrap(d.DB, "portal_sign_in_address_request", portalAddressChangeRequestRules)(clientauth.RequestAddressChangeHandler(d.DB)))
	g.Write("POST /api/portal/sign-in-address",
		ratelimit.Wrap(d.DB, "portal_sign_in_address", tokenSpendRules)(clientauth.SpendAddressChangeHandler(d.DB)))
	// Not rate limited: gated by authn.Begin's own __session cookie check,
	// like staffauth.SessionHandler below -- there is no bootstrap window
	// here for an attacker to spend.
	g.OpenGet("/api/portal/session", portalPopulation, clientauth.SessionHandler(d.DB))
	// #618, ADR-0026: her own "sign out everywhere", reusing
	// authn.EndAllSessions the same way staffauth.EndSessionsHandler's
	// Owner-driven revoke already does. Naturally idempotent -- ending
	// zero sessions is as much a success as ending several -- so this
	// carries no Idempotency-Key handling, and is gated by the live
	// session it gets to act on rather than by a rate limit.
	g.Write("DELETE /api/portal/sessions", clientauth.EndAllSessionsHandler(d.DB))
	g.OpenGet("/api/portal/engagements/{engagementId}", portalPopulation,
		clientauth.Middleware(d.DB)(portal.DetailHandler()))
	// #486 AC4/AC5: the same record-scoped ledger the staff Engagement page
	// gets, behind a closed disclosure on this side (the design brief's own
	// placement decision).
	g.OpenGet("/api/portal/engagements/{engagementId}/activity", portalPopulation,
		clientauth.Middleware(d.DB)(portal.ActivityHandler()))
	g.OpenGet("/api/portal/engagements/{engagementId}/birth-plan", portalPopulation,
		clientauth.Middleware(d.DB)(plans.ClientGetBirthPlanHandler()))
	g.OpenGet("/api/portal/engagements/{engagementId}/contract", portalPopulation,
		clientauth.Middleware(d.DB)(contracts.ClientGetContractHandler()))
	g.Write("POST /api/portal/engagements/{engagementId}/contract/sign",
		clientauth.Middleware(d.DB)(contracts.ClientPostSignContractHandler(d.Store)))
	g.OpenGet("/api/portal/engagements/{engagementId}/contract/pdf", portalPopulation,
		clientauth.Middleware(d.DB)(contracts.ClientGetSignedContractPDFHandler(d.Store)))
	g.OpenGet("/api/portal/engagements/{engagementId}/messages", portalPopulation,
		clientauth.Middleware(d.DB)(message.ClientListHandler()))
	g.Write("POST /api/portal/engagements/{engagementId}/messages",
		clientauth.Middleware(d.DB)(message.ClientCreateHandler(d.Store, d.Pusher)))
	g.OpenGet("/api/portal/engagements/{engagementId}/messages/{messageId}/attachment", portalPopulation,
		clientauth.Middleware(d.DB)(message.ClientAttachmentHandler(d.Store)))
	g.Write("POST /api/portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(d.DB)(pushsub.ClientRegisterHandler()))
	g.Write("DELETE /api/portal/engagements/{engagementId}/push-subscriptions",
		clientauth.Middleware(d.DB)(pushsub.ClientUnregisterHandler()))
	// #303: a durable, reviewable push preference -- GET reads current
	// status, PUT turns it on or off. PUT is naturally idempotent (repeating
	// the same {enabled} body re-asserts the same state), so no
	// Idempotency-Key handling applies here per docs/api-design.md section 3.
	g.OpenGet("/api/portal/engagements/{engagementId}/notification-preference", portalPopulation,
		clientauth.Middleware(d.DB)(notificationpref.GetHandler()))
	g.Write("PUT /api/portal/engagements/{engagementId}/notification-preference",
		clientauth.Middleware(d.DB)(notificationpref.SetHandler()))
}
