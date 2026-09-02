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
		ratelimit.Wrap(d.DB, "staff_accept_invite", bootstrapRules)(staffauth.AcceptInviteHandler(d.Verifier, d.DB)))
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
