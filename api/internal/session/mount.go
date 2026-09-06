package session

import (
	"database/sql"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// loginRules limits POST /api/session, which fires far more often than a
// once-per-person bootstrap event -- every sign-in, and Identity Platform
// ID tokens are cached and reused client-side for close to an hour, so
// the same token legitimately backs more than one call.
var loginRules = []ratelimit.Rule{
	ratelimit.BearerTokenRule(30, time.Hour),
	ratelimit.IPRule(100, time.Hour),
}

// Mount registers the Staff session itself: starting one and ending it.
// Neither carries a role declaration or an idempotency stance -- there is
// no Membership yet for the first, and ending a session is naturally
// idempotent (EndHandler's own doc comment).
func Mount(g *staffauth.GatedRouter, db *sql.DB, verifier authn.Verifier, nudge tasknudge.Enqueuer) {
	// Not rate limited: a liveness/readiness probe with no side effect and
	// no cost is a different route (the health probe); this one is the
	// real sign-in and does carry a limit.
	g.Write("POST /api/session",
		ratelimit.Wrap(db, "staff_login", loginRules)(CreateHandler(verifier, db, nudge)))
	g.Write("DELETE /api/session", EndHandler(db))
}
