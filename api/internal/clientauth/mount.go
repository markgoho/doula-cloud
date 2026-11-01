package clientauth

import (
	"database/sql"
	"net/http"
	"time"

	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// resetRequestRules is #602's own sizing note for the magic-link request
// endpoint: a public, pre-account endpoint keyed on the caller's own
// posted email address -- "5 per address, 20 per IP, per hour".
var resetRequestRules = []ratelimit.Rule{
	ratelimit.JSONFieldRule("email", 5, time.Hour),
	ratelimit.IPRule(20, time.Hour),
}

// tokenSpendRules limits a pre-account spend endpoint: neither carries a
// Bearer token or a session, only the link's own token, so
// HashedJSONFieldRule's digest is the "subject" dimension -- repeatedly
// spending or guessing around one token -- alongside IPRule for volume
// across many tokens.
var tokenSpendRules = []ratelimit.Rule{
	ratelimit.HashedJSONFieldRule("token", 10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// portalAddressChangeRequestRules limits #619's sign-in-address change
// request. Three dimensions: gated by a live session already
// (SessionCookieRule), and it is the one endpoint in the product that
// mails an address the caller chose rather than one the product already
// holds -- so JSONFieldRule bounds the caller who would use it to mail
// somebody else's inbox repeatedly, and IPRule bounds volume across many
// addresses.
var portalAddressChangeRequestRules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(5, time.Hour),
	ratelimit.JSONFieldRule("email", 5, time.Hour),
	ratelimit.IPRule(20, time.Hour),
}

// feedbackRules is #1523's own sizing: 20 per hour, per sender. Same
// choice staffauth's own feedbackRules makes -- SessionCookieRule is the
// closest fit this toolkit offers to "per sender" for an
// already-signed-in write.
var feedbackRules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(20, time.Hour),
}

// PortalWriteRouter is the one door Mount needs from
// idempotency.Router: each portal write here declares, at its route, why
// it runs without an Idempotency-Key (#1924). An interface rather than
// *idempotency.Router because idempotency imports this package for
// clientauth.Middleware and WrapClient's Client scope, so this package
// cannot import it back.
type PortalWriteRouter interface {
	PortalExempt(pattern, reason string, h http.Handler)
}

// Mount registers the Client's own session surface: #617's magic-link
// sign-in, #619's sign-in-address change, the session read, and #618's
// sign-out-everywhere. A different population with a different session,
// which is why none of these reads go through GatedRouter.Get.
func Mount(g *staffauth.GatedRouter, ir PortalWriteRouter, db *sql.DB, nudge tasknudge.Enqueuer) {
	// #617: request and redeem a Client sign-in link, ADR-0026's magic
	// link. Same shape as a Staff reset request/spend pair.
	ir.PortalExempt("POST /api/portal/magic-link/request",
		"pre-session; a repeat re-mints the token and resets the one pending outbox row rather than sending a second mail",
		ratelimit.Wrap(db, "portal_magic_link_request", resetRequestRules)(RequestMagicLinkHandler(db)))
	ir.PortalExempt("POST /api/portal/magic-link",
		"pre-session; the link's token is single-use, so a repeat is refused rather than signing in twice",
		ratelimit.Wrap(db, "portal_magic_link", tokenSpendRules)(RedeemMagicLinkHandler(db, nudge)))
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
	ir.PortalExempt("POST /api/portal/sign-in-address/request",
		"a repeat mints a fresh token and resets the pending outbox row rather than sending a second mail",
		ratelimit.Wrap(db, "portal_sign_in_address_request", portalAddressChangeRequestRules)(RequestAddressChangeHandler(db)))
	ir.PortalExempt("POST /api/portal/sign-in-address",
		"pre-session; the confirmation link's token is single-use, so a repeat changes nothing",
		ratelimit.Wrap(db, "portal_sign_in_address", tokenSpendRules)(SpendAddressChangeHandler(db)))
	// Not rate limited: gated by authn.Begin's own __session cookie check
	// -- there is no bootstrap window here for an attacker to spend.
	g.OpenGet("/api/portal/session", PortalPopulation, SessionHandler(db))
	// #2536: a signed-in Client who opens a dead link of her own lands
	// on the page it was asked for from. A POST only so the token stays
	// out of request logs; it reads and changes nothing.
	ir.PortalExempt("POST /api/portal/session/prior-link",
		"a read; it changes nothing, so a repeat answers the same",
		PriorLinkReturnHandler(db))
	// #618, ADR-0026: her own "sign out everywhere". Naturally idempotent
	// -- ending zero sessions is as much a success as ending several --
	// so this carries no Idempotency-Key handling, and is gated by the
	// live session it gets to act on rather than by a rate limit.
	ir.PortalExempt("DELETE /api/portal/sessions",
		"delete; ending zero sessions is as much a success as ending several",
		EndAllSessionsHandler(db))
	// #1523: any signed-in Client sends a piece of Feedback from any
	// portal screen. Not Engagement-scoped -- most of this route's
	// screens name no single Engagement -- so it sits beside the
	// session-level routes above rather than in portal.Mount.
	ir.PortalExempt("POST /api/portal/feedback",
		"#1498 Q2: redundant feedback is better than no feedback, and the route is not Engagement-scoped, so it has no clientauth.Middleware tx to key a retry on",
		ratelimit.Wrap(db, "portal_feedback", feedbackRules)(FeedbackHandler(db, nudge)))
}
