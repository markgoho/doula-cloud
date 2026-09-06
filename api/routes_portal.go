package main

import (
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/notificationpref"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/staffauth"
)

// The Client's own surface, behind clientauth rather than staffauth.
//
// A different population with a different session, which is why none of
// these reads go through GatedRouter.Get. #836 moved most of this
// file's registrations into the feature package that already owns the
// Staff-side sibling of the same record -- plans.Mount, contracts.Mount,
// message.Mount and pushsub.Mount each register their own portal route
// alongside their Practice one (called from registerPracticeRoutes),
// since it is the same package either way. What remains here is what has
// no Staff-side sibling: the Client's own session lifecycle
// (clientauth.Mount), her Engagement detail/activity read (portal.Mount),
// and her push-notification preference (notificationpref.Mount).
//
// plans.Mount, contracts.Mount, message.Mount and pushsub.Mount already
// registered their own portal routes from registerPracticeRoutes -- a
// reader looking for /api/portal/engagements/{engagementId}/birth-plan,
// /contract(/sign|/pdf), /messages(/{messageId}/attachment), or
// /push-subscriptions finds them there, beside the Practice-side sibling
// of the same record.
func registerPortalRoutes(g *staffauth.GatedRouter, d Deps) {
	clientauth.Mount(g, d.DB, d.NudgeEnqueuer)
	portal.Mount(g, d.DB)
	notificationpref.Mount(g, d.DB)
}
