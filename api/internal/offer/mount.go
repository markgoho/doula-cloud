package offer

import (
	"database/sql"
	"time"

	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// offerReadsPerOfferPerHour caps how many pre-account requests one Offer
// will answer in an hour. It is deliberately *not* maxAccessCodeAttempts
// (#846): the two counters count different things. The row counter
// (offer.go, 00041) counts wrong guesses and nothing else, permanently;
// this one counts every request against the Offer, right code or wrong.
// Sizing the two alike made a legitimate sequence unreachable -- ten
// fumbled guesses exhaust the row counter, the Practice re-issues the
// Offer (target.go resets access_code_attempts with the code), and the
// very next read, with the freshly mailed code, 429s here for up to an
// hour. The cap therefore has to clear a full exhaustion (10), the
// re-issued read that follows it (1), and the handful of ordinary
// re-reads a Doula makes of a page she was mailed a link to.
//
// Brute force is bounded by the row counter, not by this rule: a
// permanent ten guesses against a 10^6 space, which no amount of hourly
// budget widens. What this rule bounds is request *volume* against one
// Offer -- cost, not credential guessing -- alongside IPRule for volume
// across many different Offers from one caller.
const offerReadsPerOfferPerHour = 30

// offerRules limits the pre-account Offer routes. Neither endpoint has a
// Bearer token or an email to key on before its own token+code check
// runs (preaccount.go), so PathValueRule's offerId is the "subject"
// dimension here: the resource being probed, rather than who's probing
// it.
var offerRules = []ratelimit.Rule{
	ratelimit.PathValueRule("offerId", offerReadsPerOfferPerHour, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// Mount registers ADR-0008's Offer flow (#317): the Practice side
// (Owner/Admin -- making an Offer, taking it back, and reading who has
// been asked, which names people and so follows the Staff-roster row of
// the read table; the Doula side is her own inbox and her own decisions,
// scoped to her staff_id in SQL rather than by a role declaration), and
// the pre-account Offer read and decline (ADR-0008, #230): no session
// exists yet, authenticated by the Invitation token and the emailed
// access code, so both sit outside staffauth.Middleware entirely.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB, nudge tasknudge.Enqueuer) {
	// #1016 moved CreateHandler's Owner-or-Admin rule from an in-handler
	// staffauth.RequireOwnerOrAdmin call to this declaration, through
	// ReplayableGated (#990's door for a write that needs both a role
	// gate and Wrap): making an Offer stays Replayable, because a
	// double-click must not send the same Offer twice.
	ir.ReplayableGated("POST /api/practices/{practiceId}/engagements/{engagementId}/offers", false, staffauth.OwnerAndAdmin, CreateHandler(nudge))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/offers", staffauth.OwnerAndAdmin, EngagementListHandler())
	g.Get("/api/practices/{practiceId}/offers", staffauth.AnyStaff, InboxHandler())
	ir.Exempt("POST /api/practices/{practiceId}/offers/{offerId}/accept",
		"state-guarded UPDATE ... WHERE state = 'offered'; a retry after the first commit affects zero rows and 409s instead of granting the attachment twice",
		false, AcceptHandler())
	ir.Exempt("POST /api/practices/{practiceId}/offers/{offerId}/decline",
		"documented idempotent by design (#229): declining an already-declined Offer succeeds again rather than erroring",
		false, DeclineHandler())
	// Owner-or-Admin declared here rather than in WithdrawHandler (#1016,
	// following #970 and #990): taking an Offer back is the Practice's
	// half of #229, not a reach question, so the mount is where the rule
	// belongs.
	ir.ExemptGated("POST /api/practices/{practiceId}/offers/{offerId}/withdraw",
		"state-guarded UPDATE ... WHERE state = 'offered'; a retry after the first commit affects zero rows and 409s instead of withdrawing twice",
		false, staffauth.OwnerAndAdmin, WithdrawHandler())

	g.OpenGet("/api/offers/{offerId}",
		"pre-account Offer read (ADR-0008, #230): no session exists yet -- authenticated by the Invitation token and the emailed access code",
		ratelimit.Wrap(db, "offer_read", offerRules)(ReadHandler(db)))
	g.Write("POST /api/offers/{offerId}/decline",
		ratelimit.Wrap(db, "offer_decline", offerRules)(DeclineByTokenHandler(db)))
}
