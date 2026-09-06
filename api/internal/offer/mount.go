package offer

import (
	"database/sql"
	"time"

	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

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

// Mount registers ADR-0008's Offer flow (#317): the Practice side
// (Owner/Admin -- making an Offer, taking it back, and reading who has
// been asked, which names people and so follows the Staff-roster row of
// the read table; the Doula side is her own inbox and her own decisions,
// scoped to her staff_id in SQL rather than by a role declaration), and
// the pre-account Offer read and decline (ADR-0008, #230): no session
// exists yet, authenticated by the Invitation token and the emailed
// access code, so both sit outside staffauth.Middleware entirely.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB, nudge tasknudge.Enqueuer) {
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/offers", false, CreateHandler(nudge))
	g.Get("/api/practices/{practiceId}/engagements/{engagementId}/offers", staffauth.OwnerAndAdmin, EngagementListHandler())
	g.Get("/api/practices/{practiceId}/offers", staffauth.AnyStaff, InboxHandler())
	ir.Exempt("POST /api/practices/{practiceId}/offers/{offerId}/accept",
		"state-guarded UPDATE ... WHERE state = 'offered'; a retry after the first commit affects zero rows and 409s instead of granting the attachment twice",
		false, AcceptHandler())
	ir.Exempt("POST /api/practices/{practiceId}/offers/{offerId}/decline",
		"documented idempotent by design (#229): declining an already-declined Offer succeeds again rather than erroring",
		false, DeclineHandler())
	ir.Exempt("POST /api/practices/{practiceId}/offers/{offerId}/withdraw",
		"state-guarded UPDATE ... WHERE state = 'offered'; a retry after the first commit affects zero rows and 409s instead of withdrawing twice",
		false, WithdrawHandler())

	g.OpenGet("/api/offers/{offerId}",
		"pre-account Offer read (ADR-0008, #230): no session exists yet -- authenticated by the Invitation token and the emailed access code",
		ratelimit.Wrap(db, "offer_read", offerRules)(ReadHandler(db)))
	g.Write("POST /api/offers/{offerId}/decline",
		ratelimit.Wrap(db, "offer_decline", offerRules)(DeclineByTokenHandler(db)))
}
