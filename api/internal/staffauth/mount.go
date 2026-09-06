package staffauth

import (
	"database/sql"
	"net/http"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/tasknudge"
)

// WriteRouter is the idempotency.Router surface Mount needs: Replayable
// and Exempt, the only two doors a mutating Practice route can be
// registered through. An interface, not *idempotency.Router, because
// idempotency imports staffauth to apply Middleware and AttachingWrite
// itself -- staffauth importing idempotency back would cycle.
// *idempotency.Router satisfies this without either package naming the
// other.
type WriteRouter interface {
	Replayable(pattern string, attaching bool, h http.Handler)
	Exempt(pattern, reason string, attaching bool, h http.Handler)
}

// bootstrapRules limits the once-per-person events -- signup and
// invitation acceptance -- that read a Bearer ID token via
// authn.BeginBootstrap before any session exists. BearerTokenRule bounds
// how many times one presented credential may retry the same bootstrap
// event; IPRule bounds the address minting fresh credentials to keep
// retrying it. Values are generous rather than tight: nothing has ever
// limited this endpoint before, so any finite cap is a real improvement.
// Sized above the Playwright e2e suite's own signup/accept-invite volume,
// which runs every test against one shared BFF and one IP within a
// single run, and above a 14-doula agency's pilot-sized onboarding burst
// (#602 sizing note): several people fumbling an invite from the same
// birth-centre connection in one hour must not lock each other out.
var bootstrapRules = []ratelimit.Rule{
	ratelimit.BearerTokenRule(5, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// loginRules is bootstrapRules' shape for a self-service action that
// fires more than once per person -- Identity Platform ID tokens are
// cached and reused client-side for close to an hour, so the same token
// legitimately backs more than one call.
var loginRules = []ratelimit.Rule{
	ratelimit.BearerTokenRule(30, time.Hour),
	ratelimit.IPRule(100, time.Hour),
}

// verifyRequestRules limits the signed-in "send me a fresh verification
// link" re-request and the MFA removal step-up. Generous, like
// bootstrapRules: both are low-risk self-service actions gated by a live
// session already, not a bootstrap credential someone could be minting
// fresh copies of.
var verifyRequestRules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// resetRequestRules is #602's own sizing note, reused verbatim: a public,
// pre-account endpoint keyed on the caller's own posted email address --
// "5 per address, 20 per IP, per hour". Answers identically whether or
// not the address exists (#168's account-enumeration rule).
var resetRequestRules = []ratelimit.Rule{
	ratelimit.JSONFieldRule("email", 5, time.Hour),
	ratelimit.IPRule(20, time.Hour),
}

// tokenSpendRules limits both pre-account spend endpoints (verify-email,
// password-reset): neither carries a Bearer token or a session, only the
// link's own token, so HashedJSONFieldRule's digest is the "subject"
// dimension -- repeatedly spending or guessing around one token --
// alongside IPRule for volume across many tokens.
var tokenSpendRules = []ratelimit.Rule{
	ratelimit.HashedJSONFieldRule("token", 10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// mfaRecoverySpendRules is #615's AC read literally: "spend is throttled
// per account on failed attempts, not only per IP" -- a short issued
// code (8 decimal digits) read aloud over a phone call, live for 24
// hours, is brute-forceable without a per-account limit tighter than
// tokenSpendRules' 128-bit link tokens ever needed.
var mfaRecoverySpendRules = []ratelimit.Rule{
	ratelimit.HashedJSONFieldRule("email", 10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// mfaRecoveryRotateRules limits the signed-in "show me my saved codes
// again" self-service action. Generous, like verifyRequestRules.
var mfaRecoveryRotateRules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// Mount registers every route this package owns: the Practice-scoped
// Staff roster, Membership, Invitation, session-management and MFA
// surface (behind Middleware via ir), and the routes that precede a
// Practice session entirely -- sign-up, sign-in, invitation acceptance,
// and the person-level facts (work state, email, MFA enrolment) that
// #437 and #613 keep off any one Membership (mounted directly on g,
// since there is no session yet for Middleware to establish).
func Mount(g *GatedRouter, ir WriteRouter, db *sql.DB, verifier authn.Verifier, accounts authn.AccountManager, enq tasknudge.Enqueuer) {
	mountPracticeRoutes(g, ir, verifier, accounts, enq)
	mountSessionRoutes(g, db, verifier, accounts)
}

// mountPracticeRoutes is the Practice-scoped half of Mount: Staff roster,
// Membership, Invitation, ending sessions, and the MFA-required switch.
func mountPracticeRoutes(g *GatedRouter, ir WriteRouter, verifier authn.Verifier, accounts authn.AccountManager, enq tasknudge.Enqueuer) {
	g.Get("/api/practices/{practiceId}/session", AnyStaff, PracticeSessionHandler())
	// Roles and employment type are edited together on one surface
	// (RA-G2, #261) -- ADR-0008 makes them the two halves of what a
	// person is at a Practice, so there is one endpoint, not two.
	ir.Exempt("PATCH /api/practices/{practiceId}/staff/{staffId}/membership",
		"PATCH replaces roles and employment type wholesale to the caller's given values, and records an audit event only for an axis that actually changed -- a repeated call with the same body is a no-op on both",
		false, UpdateMembershipHandler())
	// The route #291 found missing: without it a roster row nobody wants
	// can never be taken off.
	ir.Exempt("DELETE /api/practices/{practiceId}/staff/{staffId}/membership",
		"delete; a retry after the first succeeds finds no membership row left and 404s instead of removing or recording removal twice",
		false, RemoveMembershipHandler())
	ir.Replayable("POST /api/practices/{practiceId}/staff/invitations", false, InviteHandler(enq))
	ir.Exempt("POST /api/practices/{practiceId}/staff/invitations/{invitationId}/revoke",
		"state-guarded UPDATE ... WHERE status = 'pending'; a retry after the first commit affects zero rows and 404s instead of revoking twice",
		false, RevokeInvitationHandler())
	// Staff roster -- members and pending invitations both: Owner and
	// Admin only (ADR-0008's read table) -- a Doula has no reason to see
	// the full roster.
	g.Get("/api/practices/{practiceId}/staff", OwnerAndAdmin, ListStaffHandler())
	// The history behind one roster row's "Works from" value (#459). Same
	// gate as the roster it hangs off, and the same rows 00043's existing
	// policy already admits -- a reader, not a widening.
	g.Get("/api/practices/{practiceId}/staff/{staffId}/work-state-history",
		OwnerAndAdmin, ListWorkStateHistoryHandler())
	ir.Exempt("DELETE /api/practices/{practiceId}/staff/{staffId}/sessions",
		"EndAllSessions ends whatever remains and no-ops once already ended, and QueueSessionRevoked's own ON CONFLICT ... WHERE status = 'pending' DO NOTHING dedupes the notification; a retry can't double-notify",
		false, EndSessionsHandler(enq))
	// #615: an Owner vouching for a locked-out colleague. Replayable, not
	// Exempt -- unlike EndSessions, a bare retry here is not a no-op: it
	// would invalidate the just-minted code (authtoken.MintCode's own
	// re-request rule) and queue a second email before the Owner has
	// necessarily read the first.
	ir.Replayable("POST /api/practices/{practiceId}/staff/{staffId}/mfa-recovery/vouch", false, VouchHandler(verifier, enq))
	// #606: the switch's pre-throw count -- how many Staff it will
	// affect, read before an Owner ever calls the PUT below. Owner-only,
	// the same population who may throw the switch at all.
	g.Get("/api/practices/{practiceId}/mfa-required/impact", OwnerOnly, GetMFAImpactHandler(accounts))
	// A retry with the same body reads the same current value and writes
	// nothing new -- see PutMFARequiredHandler's own doc comment.
	ir.Exempt("PUT /api/practices/{practiceId}/mfa-required",
		"idempotent by construction: the handler reads the current value first and updates/records only when the given value actually differs from it",
		false, PutMFARequiredHandler())
}

// mountSessionRoutes is the pre-Practice half of Mount: sign-in, sign-up,
// invitation acceptance, and the person-level facts (work state, email,
// MFA) that #437 and #613 keep off any one Membership.
func mountSessionRoutes(g *GatedRouter, db *sql.DB, verifier authn.Verifier, accounts authn.AccountManager) {
	// Not rate limited: gated by authn.Begin's own __session cookie check
	// -- there is no bootstrap window here for an attacker to spend.
	g.Write("POST /api/staff/signup",
		ratelimit.Wrap(db, "staff_signup", bootstrapRules)(SignupHandler(verifier, db)))
	// Not rate limited: gated by authn.Begin's own __session cookie check
	// -- there is no bootstrap window here for an attacker to spend.
	g.OpenGet("/api/staff/session",
		"lists a person's own memberships before any {practiceId} is chosen -- there is no Membership yet for a role declaration to be about",
		SessionHandler(db))
	// Where she works is a fact about the person, not about a Membership
	// (00043), so its write sits beside the session probe rather than
	// under a Practice -- no {practiceId} in the path, and no staff id
	// either, which is what makes it self-edit-only by shape (#437).
	g.Write("PUT /api/staff/work-state", UpdateWorkStateHandler(db))
	g.Write("POST /api/staff/accept-invite",
		ratelimit.Wrap(db, "staff_accept_invite", bootstrapRules)(AcceptInviteHandler(verifier, accounts, db)))
	// #613: an email address, like a work state, is a fact about the
	// person -- same "no {practiceId}, no staff id" shape as
	// UpdateWorkStateHandler above. Not rate limited: gated by
	// authn.Begin's own __session cookie check.
	g.Write("PUT /api/staff/email", ChangeEmailHandler(accounts, db))
	// The signed-in re-request AC #613 added while resolving #169: a
	// 24-hour verification link and ADR-0010's retry window are roughly
	// the same length, so this is the only way to recover from a link
	// that arrived already dead. Keyed on the session cookie
	// (ratelimit.SessionCookieRule) rather than a Bearer token -- there
	// is no bootstrap window here, the caller is already signed in.
	g.Write("POST /api/staff/verify-email/request",
		ratelimit.Wrap(db, "staff_verify_email_request", verifyRequestRules)(RequestVerificationHandler(db)))
	// Public and pre-account: a verification link can be opened signed
	// out of everything, so this reads no Bearer token and no session --
	// the link's own token is the whole credential.
	g.Write("POST /api/staff/verify-email",
		ratelimit.Wrap(db, "staff_verify_email", tokenSpendRules)(SpendVerificationHandler(accounts, db)))
	// Public and unauthenticated -- a forgotten password is, by
	// definition, no credential to present. Keyed on the request's own
	// email field (ratelimit.JSONFieldRule) since there is nothing else
	// to key on this early; answers identically whether or not the
	// address exists (#168's account-enumeration rule).
	g.Write("POST /api/staff/password-reset/request",
		ratelimit.Wrap(db, "staff_password_reset_request", resetRequestRules)(RequestResetHandler(accounts, db)))
	// Public and pre-account, same shape as verify-email above: the reset
	// token is the whole credential.
	g.Write("POST /api/staff/password-reset",
		ratelimit.Wrap(db, "staff_password_reset", tokenSpendRules)(SpendResetHandler(accounts, db)))
	// #615: the one unauthenticated endpoint for all three MFA-recovery
	// paths' spend. Public and pre-account -- a locked-out person cannot
	// sign in first (#605's sequence) -- keyed on the request's own email
	// field for #602's per-account throttle (mfaRecoverySpendRules), the
	// same shape resetRequestRules uses.
	g.Write("POST /api/staff/mfa-recovery/spend",
		ratelimit.Wrap(db, "staff_mfa_recovery_spend", mfaRecoverySpendRules)(SpendMFARecoveryHandler(accounts, db)))
	// Self-only, same "no {practiceId}, no staff id" shape as
	// PUT /api/staff/work-state above -- who is currently a sole Owner is
	// a fact about the person, not a Membership, and this is the only
	// path by which she ever sees a saved code's plaintext (#615).
	g.Write("POST /api/staff/mfa-recovery/saved-codes/rotate",
		ratelimit.Wrap(db, "staff_mfa_recovery_rotate", mfaRecoveryRotateRules)(RotateSavedCodesHandler(db)))
	// #606: enrolment is per person, not per Practice (the brief), so
	// finishing one sits beside work-state and email above, not under a
	// Practice. Rate limited with loginRules, not bootstrapRules: unlike
	// signup or invite acceptance this can fire more than once per
	// person (voluntary enrolment, a later re-enrolment after removing a
	// factor), the same "cached token reused, sign-in fires more than
	// once" reasoning loginRules exists for.
	g.Write("POST /api/staff/mfa",
		ratelimit.Wrap(db, "staff_mfa_enroll", loginRules)(FinishEnrollmentHandler(verifier, db)))
	// Voluntary removal, guarded by RequireRecentAuth's step-up rather
	// than by rate limiting -- the same reasoning verifyRequestRules
	// gives for a low-risk, already-signed-in self-service action.
	g.Write("DELETE /api/staff/mfa",
		ratelimit.Wrap(db, "staff_mfa_remove", verifyRequestRules)(RemoveSecondFactorHandler(verifier, accounts, db)))
}
