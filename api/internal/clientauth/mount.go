package clientauth

import (
	"database/sql"
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

// Mount registers the Client's own session surface: #617's magic-link
// sign-in, #619's sign-in-address change, the session read, and #618's
// sign-out-everywhere. A different population with a different session,
// which is why none of these reads go through GatedRouter.Get.
func Mount(g *staffauth.GatedRouter, db *sql.DB, nudge tasknudge.Enqueuer) {
	// #617: request and redeem a Client sign-in link, ADR-0026's magic
	// link. Same shape as a Staff reset request/spend pair.
	g.Write("POST /api/portal/magic-link/request",
		ratelimit.Wrap(db, "portal_magic_link_request", resetRequestRules)(RequestMagicLinkHandler(db)))
	g.Write("POST /api/portal/magic-link",
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
	g.Write("POST /api/portal/sign-in-address/request",
		ratelimit.Wrap(db, "portal_sign_in_address_request", portalAddressChangeRequestRules)(RequestAddressChangeHandler(db)))
	g.Write("POST /api/portal/sign-in-address",
		ratelimit.Wrap(db, "portal_sign_in_address", tokenSpendRules)(SpendAddressChangeHandler(db)))
	// Not rate limited: gated by authn.Begin's own __session cookie check
	// -- there is no bootstrap window here for an attacker to spend.
	g.OpenGet("/api/portal/session", PortalPopulation, SessionHandler(db))
	// #618, ADR-0026: her own "sign out everywhere". Naturally idempotent
	// -- ending zero sessions is as much a success as ending several --
	// so this carries no Idempotency-Key handling, and is gated by the
	// live session it gets to act on rather than by a rate limit.
	g.Write("DELETE /api/portal/sessions", EndAllSessionsHandler(db))
}
