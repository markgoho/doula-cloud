package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/staffauth"
)

type helloResponse struct {
	Message string `json:"message"`
}

func helloHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(helloResponse{Message: "hello world"}); err != nil {
		log.Printf("helloHandler: encode response: %v", err)
	}
}

// The routes that belong to no Practice: the health probe, the Staff
// session itself, sign-up and invitation acceptance, and #230's
// pre-account Offer read.
//
// What they have in common is that none of them can be scoped to a
// Practice, because at the moment they are called there may not be one
// yet -- or, for the work-state write, because where a person works is a
// fact about her rather than about a Membership (00043).
func registerSessionRoutes(mux *http.ServeMux, g *staffauth.GatedRouter, d Deps) {
	// Under /api like every other route: Firebase Hosting rewrites /api/** to
	// this service with the path unchanged, so a bare /hello would be
	// unreachable from the browser. CI's two smoke tests curl this same path
	// against the container and against the raw Cloud Run URL.
	mux.HandleFunc("GET /api/hello", helloHandler)
	// Not rate limited: a liveness/readiness probe with no side effect and
	// no cost, curled in a loop by CI's own smoke tests and by whatever
	// polls Cloud Run's health check -- limiting it would break exactly
	// the callers it exists for (#602's docs/api-design.md entry records
	// this).
	mux.Handle("POST /api/session",
		ratelimit.Wrap(d.DB, "staff_login", loginRules)(session.CreateHandler(d.Verifier, d.DB, d.NudgeEnqueuer)))
	mux.Handle("DELETE /api/session", session.EndHandler(d.DB))
	// Not rate limited: it only ever clears a cookie the caller already
	// holds (or does nothing if there is none) -- no credential is
	// checked, so there is nothing here for an attacker to gain by
	// calling it repeatedly.
	mux.Handle("POST /api/staff/signup",
		ratelimit.Wrap(d.DB, "staff_signup", bootstrapRules)(staffauth.SignupHandler(d.Verifier, d.DB)))
	// Not rate limited: gated by authn.Begin's own __session cookie check
	// -- there is no bootstrap window here for an attacker to spend.
	mux.Handle("GET /api/staff/session", staffauth.SessionHandler(d.DB))
	// Where she works is a fact about the person, not about a Membership
	// (00043), so its write sits beside the session probe rather than
	// under a Practice -- no {practiceId} in the path, and no staff id
	// either, which is what makes it self-edit-only by shape (#437).
	mux.Handle("PUT /api/staff/work-state", staffauth.UpdateWorkStateHandler(d.DB))
	mux.Handle("POST /api/staff/accept-invite",
		ratelimit.Wrap(d.DB, "staff_accept_invite", bootstrapRules)(staffauth.AcceptInviteHandler(d.Verifier, d.AccountManager, d.DB)))
	// #613: an email address, like a work state, is a fact about the
	// person -- same "no {practiceId}, no staff id" shape as
	// UpdateWorkStateHandler above. Not rate limited: gated by
	// authn.Begin's own __session cookie check, the same reasoning
	// GET /api/staff/session and PUT /api/staff/work-state give.
	mux.Handle("PUT /api/staff/email", staffauth.ChangeEmailHandler(d.AccountManager, d.DB))
	// The signed-in re-request AC #613 added while resolving #169: a
	// 24-hour verification link and ADR-0010's retry window are roughly
	// the same length, so this is the only way to recover from a link
	// that arrived already dead. Keyed on the session cookie
	// (ratelimit.SessionCookieRule) rather than a Bearer token -- there
	// is no bootstrap window here, the caller is already signed in.
	mux.Handle("POST /api/staff/verify-email/request",
		ratelimit.Wrap(d.DB, "staff_verify_email_request", verifyRequestRules)(staffauth.RequestVerificationHandler(d.DB)))
	// Public and pre-account: a verification link can be opened signed
	// out of everything, so this reads no Bearer token and no session --
	// the link's own token is the whole credential (docs/api-design.md
	// section 6's table records the disposition).
	mux.Handle("POST /api/staff/verify-email",
		ratelimit.Wrap(d.DB, "staff_verify_email", tokenSpendRules)(staffauth.SpendVerificationHandler(d.AccountManager, d.DB)))
	// Public and unauthenticated -- a forgotten password is, by
	// definition, no credential to present. Keyed on the request's own
	// email field (ratelimit.JSONFieldRule) since there is nothing else
	// to key on this early; answers identically whether or not the
	// address exists (#168's account-enumeration rule).
	mux.Handle("POST /api/staff/password-reset/request",
		ratelimit.Wrap(d.DB, "staff_password_reset_request", resetRequestRules)(staffauth.RequestResetHandler(d.AccountManager, d.DB)))
	// Public and pre-account, same shape as verify-email above: the reset
	// token is the whole credential.
	mux.Handle("POST /api/staff/password-reset",
		ratelimit.Wrap(d.DB, "staff_password_reset", tokenSpendRules)(staffauth.SpendResetHandler(d.AccountManager, d.DB)))
	// #230's pre-account Offer read: no session of either population, so
	// it is mounted on the raw mux and authenticated by the Invitation's
	// token plus the emailed six-digit code. ADR-0008 requires the
	// exemption declared by name in GatedRouter's own registry, in the
	// same change that mounts the route -- g.Exempt is that declaration,
	// and the guardrail test walks it.
	g.Exempt("/api/offers/{offerId}", "pre-account Offer read (ADR-0008, #230): no session exists yet -- authenticated by the Invitation token and the emailed access code")
	mux.Handle("GET /api/offers/{offerId}",
		ratelimit.Wrap(d.DB, "offer_read", offerRules)(offer.ReadHandler(d.DB)))
	mux.Handle("POST /api/offers/{offerId}/decline",
		ratelimit.Wrap(d.DB, "offer_decline", offerRules)(offer.DeclineByTokenHandler(d.DB)))
}

// bootstrapRules limits the three once-per-person events -- signup and
// both invitation acceptances -- that read a Bearer ID token via
// authn.BeginBootstrap before any session exists (see authn.go's own
// comment naming the three). BearerTokenRule bounds how many times one
// presented credential may retry the same bootstrap event; IPRule bounds
// the address minting fresh credentials to keep retrying it. Values are
// generous rather than tight: nothing has ever limited this endpoint
// before, so any finite cap is a real improvement, and #602's own
// decision for the still-unbuilt magic-link request endpoint (5 per
// email, 20 per IP, per hour -- docs/api-design.md) is the tighter
// reference point once real traffic exists to tune against. Sized above
// the Playwright e2e suite's own signup/accept-invite volume, which runs
// every test against one shared BFF and one IP within a single run, and
// above a 14-doula agency's pilot-sized onboarding burst (#602 sizing
// note): several people fumbling an invite from the same birth-centre
// connection in one hour must not lock each other out.
var bootstrapRules = []ratelimit.Rule{
	ratelimit.BearerTokenRule(5, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// loginRules is bootstrapRules' shape for POST /api/session, which fires
// far more often than a once-per-person bootstrap event -- every sign-in,
// and Identity Platform ID tokens are cached and reused client-side for
// close to an hour, so the same token legitimately backs more than one
// call. Both limits sit above bootstrapRules' for that reason.
var loginRules = []ratelimit.Rule{
	ratelimit.BearerTokenRule(30, time.Hour),
	ratelimit.IPRule(100, time.Hour),
}

// offerRules limits the pre-account Offer routes. Neither endpoint has a
// Bearer token or an email to key on before its own token+code check
// runs (preaccount.go), so PathValueRule's offerId is the "subject"
// dimension here: the resource being probed, rather than who's probing
// it. Brute-forcing one Offer's six-digit code is already bounded
// permanently by maxAccessCodeAttempts (offer.go, 00041) -- 10, which
// PathValueRule's cap matches -- so what this rule set adds is a per-hour
// cap on the same thing, plus IPRule for volume across many different
// Offers from one caller.
var offerRules = []ratelimit.Rule{
	ratelimit.PathValueRule("offerId", 10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// verifyRequestRules limits the signed-in "send me a fresh verification
// link" re-request. Generous, like bootstrapRules: this is a low-risk
// self-service action gated by a live session already, not a bootstrap
// credential someone could be minting fresh copies of.
var verifyRequestRules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// resetRequestRules is #602's own sizing note for the still-unbuilt
// magic-link request endpoint (docs/api-design.md), reused verbatim:
// both are a public, pre-account endpoint keyed on the caller's own
// posted email address, so the same "5 per address, 20 per IP, per
// hour" reasoning applies unchanged.
var resetRequestRules = []ratelimit.Rule{
	ratelimit.JSONFieldRule("email", 5, time.Hour),
	ratelimit.IPRule(20, time.Hour),
}

// tokenSpendRules limits both #613 pre-account spend endpoints
// (verify-email, password-reset): neither carries a Bearer token or a
// session, only the link's own token, so HashedJSONFieldRule's digest is
// the "subject" dimension -- repeatedly spending or guessing around one
// token -- alongside IPRule for volume across many tokens. Brute-forcing
// the token itself is infeasible regardless (128+ bits of randomness,
// authtoken.Mint); this bounds a script hammering one captured link, the
// same shape offerRules bounds one Offer's access code.
var tokenSpendRules = []ratelimit.Rule{
	ratelimit.HashedJSONFieldRule("token", 10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}
