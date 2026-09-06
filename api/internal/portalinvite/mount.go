package portalinvite

import (
	"database/sql"
	"time"

	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// portalAcceptInviteRules is tokenSpendRules' own shape for
// POST /api/portal/accept-invite (#617): since a Client has no Identity
// Platform account to bootstrap through any more, this endpoint reads no
// Bearer token either, only the invitation's own token -- so it is keyed
// on that field (HashedJSONFieldRule) rather than a BearerTokenRule.
var portalAcceptInviteRules = []ratelimit.Rule{
	ratelimit.HashedJSONFieldRule("inviteToken", 10, time.Hour),
	ratelimit.IPRule(50, time.Hour),
}

// Mount registers the Practice side of the portal invite -- inviting a
// Client to the portal, one of #350's write surfaces exempt from
// staffauth.AttachingWrite by name (offer.CreateHandler's mount comment
// records the same reasoning) -- and the pre-account acceptance itself,
// which has no session of either population yet.
func Mount(g *staffauth.GatedRouter, ir *idempotency.Router, db *sql.DB, nudge tasknudge.Enqueuer) {
	ir.Replayable("POST /api/practices/{practiceId}/engagements/{engagementId}/portal-invite", false, InviteHandler(nudge))

	// #617: a Client has no Identity Platform account to authenticate
	// through any more, so the invitation token is the whole credential.
	g.Write("POST /api/portal/accept-invite",
		ratelimit.Wrap(db, "portal_accept_invite", portalAcceptInviteRules)(AcceptInviteHandler(db, nudge)))
}
