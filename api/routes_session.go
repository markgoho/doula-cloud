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
func registerSessionRoutes(g *staffauth.GatedRouter, d Deps) {
	// Under /api like every other route: Firebase Hosting rewrites /api/** to
	// this service with the path unchanged, so a bare /hello would be
	// unreachable from the browser. CI's two smoke tests curl this same path
	// against the container and against the raw Cloud Run URL.
	g.OpenGet("/api/hello", "no auth at all -- a health probe", http.HandlerFunc(helloHandler))
	// Not rate limited: a liveness/readiness probe with no side effect and
	// no cost, curled in a loop by CI's own smoke tests and by whatever
	// polls Cloud Run's health check -- limiting it would break exactly
	// the callers it exists for (#602's docs/api-design.md entry records
	// this).
	g.Write("POST /api/session",
		ratelimit.Wrap(d.DB, "staff_login", loginRules)(session.CreateHandler(d.Verifier, d.DB, d.NudgeEnqueuer)))
	g.Write("DELETE /api/session", session.EndHandler(d.DB))
	// Not rate limited: it only ever clears a cookie the caller already
	// holds (or does nothing if there is none) -- no credential is
	// checked, so there is nothing here for an attacker to gain by
	// calling it repeatedly.
	g.Write("POST /api/staff/signup",
		ratelimit.Wrap(d.DB, "staff_signup", bootstrapRules)(staffauth.SignupHandler(d.Verifier, d.DB)))
	// Not rate limited: gated by authn.Begin's own __session cookie check
	// -- there is no bootstrap window here for an attacker to spend.
	g.OpenGet("/api/staff/session",
		"lists a person's own memberships before any {practiceId} is chosen -- there is no Membership yet for a role declaration to be about",
		staffauth.SessionHandler(d.DB))
	// Where she works is a fact about the person, not about a Membership
	// (00043), so its write sits beside the session probe rather than
	// under a Practice -- no {practiceId} in the path, and no staff id
	// either, which is what makes it self-edit-only by shape (#437).
	g.Write("PUT /api/staff/work-state", staffauth.UpdateWorkStateHandler(d.DB))
	g.Write("POST /api/staff/accept-invite",
		ratelimit.Wrap(d.DB, "staff_accept_invite", bootstrapRules)(staffauth.AcceptInviteHandler(d.Verifier, d.AccountManager, d.DB)))
	// #613: an email address, like a work state, is a fact about the
	// person -- same "no {practiceId}, no staff id" shape as
	// UpdateWorkStateHandler above. Not rate limited: gated by
	// authn.Begin's own __session cookie check, the same reasoning
	// GET /api/staff/session and PUT /api/staff/work-state give.
	g.Write("PUT /api/staff/email", staffauth.ChangeEmailHandler(d.AccountManager, d.DB))
	// The signed-in re-request AC #613 added while resolving #169: a
	// 24-hour verification link and ADR-0010's retry window are roughly
	// the same length, so this is the only way to recover from a link
	// that arrived already dead. Keyed on the session cookie
	// (ratelimit.SessionCookieRule) rather than a Bearer token -- there
	// is no bootstrap window here, the caller is already signed in.
	g.Write("POST /api/staff/verify-email/request",
		ratelimit.Wrap(d.DB, "staff_verify_email_request", verifyRequestRules)(staffauth.RequestVerificationHandler(d.DB)))
	// Public and pre-account: a verification link can be opened signed
	// out of everything, so this reads no Bearer token and no session --
	// the link's own token is the whole credential (docs/api-design.md
	// section 6's table records the disposition).
	g.Write("POST /api/staff/verify-email",
		ratelimit.Wrap(d.DB, "staff_verify_email", tokenSpendRules)(staffauth.SpendVerificationHandler(d.AccountManager, d.DB)))
	// Public and unauthenticated -- a forgotten password is, by
	// definition, no credential to present. Keyed on the request's own
	// email field (ratelimit.JSONFieldRule) since there is nothing else
	// to key on this early; answers identically whether or not the
	// address exists (#168's account-enumeration rule).
	g.Write("POST /api/staff/password-reset/request",
		ratelimit.Wrap(d.DB, "staff_password_reset_request", resetRequestRules)(staffauth.RequestResetHandler(d.AccountManager, d.DB)))
	// Public and pre-account, same shape as verify-email above: the reset
	// token is the whole credential.
	g.Write("POST /api/staff/password-reset",
		ratelimit.Wrap(d.DB, "staff_password_reset", tokenSpendRules)(staffauth.SpendResetHandler(d.AccountManager, d.DB)))
	// #615: the one unauthenticated endpoint for all three MFA-recovery
	// paths' spend. Public and pre-account -- a locked-out person cannot
	// sign in first (#605's sequence) -- keyed on the request's own email
	// field for #602's per-account throttle (mfaRecoverySpendRules), the
	// same shape resetRequestRules uses.
	g.Write("POST /api/staff/mfa-recovery/spend",
		ratelimit.Wrap(d.DB, "staff_mfa_recovery_spend", mfaRecoverySpendRules)(staffauth.SpendMFARecoveryHandler(d.AccountManager, d.DB)))
	// Self-only, same "no {practiceId}, no staff id" shape as
	// PUT /api/staff/work-state above -- who is currently a sole Owner is
	// a fact about the person, not a Membership, and this is the only
	// path by which she ever sees a saved code's plaintext (#615).
	g.Write("POST /api/staff/mfa-recovery/saved-codes/rotate",
		ratelimit.Wrap(d.DB, "staff_mfa_recovery_rotate", mfaRecoveryRotateRules)(staffauth.RotateSavedCodesHandler(d.DB)))
	// #606: enrolment is per person, not per Practice (the brief), so
	// finishing one sits beside work-state and email above, not under a
	// Practice. Rate limited with loginRules, not bootstrapRules: unlike
	// signup or invite acceptance this can fire more than once per
	// person (voluntary enrolment, a later re-enrolment after removing a
	// factor), the same "cached token reused, sign-in fires more than
	// once" reasoning loginRules exists for.
	g.Write("POST /api/staff/mfa",
		ratelimit.Wrap(d.DB, "staff_mfa_enroll", loginRules)(staffauth.FinishEnrollmentHandler(d.Verifier, d.DB)))
	// Voluntary removal, guarded by RequireRecentAuth's step-up rather
	// than by rate limiting -- the same reasoning verifyRequestRules
	// gives for a low-risk, already-signed-in self-service action.
	g.Write("DELETE /api/staff/mfa",
		ratelimit.Wrap(d.DB, "staff_mfa_remove", verifyRequestRules)(staffauth.RemoveSecondFactorHandler(d.Verifier, d.AccountManager, d.DB)))
	// #230's pre-account Offer read: no session of either population, so
	// it sits outside both middlewares and is authenticated by the
	// Invitation's token plus the emailed six-digit code. ADR-0008
	// requires the exemption declared by name in GatedRouter's own
	// registry, in the same change that mounts the route -- which used to
	// be a g.Exempt call beside a mux.Handle call, two statements that had
	// to agree with each other. OpenGet is both at once.
	g.OpenGet("/api/offers/{offerId}",
		"pre-account Offer read (ADR-0008, #230): no session exists yet -- authenticated by the Invitation token and the emailed access code",
		ratelimit.Wrap(d.DB, "offer_read", offerRules)(offer.ReadHandler(d.DB)))
	g.Write("POST /api/offers/{offerId}/decline",
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

// portalAcceptInviteRules is tokenSpendRules' own shape for
// POST /api/portal/accept-invite (#617): since a Client has no Identity
// Platform account to bootstrap through any more, this endpoint reads no
// Bearer token either, only the invitation's own token -- so it is keyed
// on that field (HashedJSONFieldRule) rather than bootstrapRules'
// BearerTokenRule.
var portalAcceptInviteRules = []ratelimit.Rule{
	ratelimit.HashedJSONFieldRule("inviteToken", 10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// mfaRecoverySpendRules is #615's AC read literally: "spend is throttled
// per account on failed attempts, not only per IP" -- a short issued
// code (8 decimal digits) read aloud over a phone call, live for 24
// hours, is brute-forceable without a per-account limit tighter than
// tokenSpendRules' 128-bit link tokens ever needed. Keyed on the
// request's own email field (HashedJSONFieldRule, like resetRequestRules)
// rather than a session or Bearer token -- there is neither this early.
// 10 per hour comfortably covers a person mistyping a hand-copied code a
// few times while still bounding a script trying every value in the
// 10^8 keyspace against one address.
var mfaRecoverySpendRules = []ratelimit.Rule{
	ratelimit.HashedJSONFieldRule("email", 10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// mfaRecoveryRotateRules limits the signed-in "show me my saved codes
// again" self-service action. Generous, like verifyRequestRules: a
// low-risk action gated by a live session already, not a bootstrap
// credential.
var mfaRecoveryRotateRules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}
