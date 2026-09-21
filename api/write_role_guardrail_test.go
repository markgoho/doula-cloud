package main

import "testing"

// The reasons below that more than one route gives, named once so
// goconst's repeated-literal threshold is met and so two routes making
// the same argument can never drift onto two wordings of it.
const (
	reasonReachNotRole = "ADR-0008's write table is a reach question, not a role one: open to any Staff member who reaches the Engagement, which attaching=true (staffauth.AttachingWrite) enforces at this mount in place of a role list"
	reasonOwnOffer     = "the Doula's own decision on her own Offer (ADR-0008, #317): scoped to her staff_id in SQL, which is an identity rule rather than a role one"
	reasonOwnBrowser   = "a Staff member registering or forgetting her own browser for push; there is no seat this could be reserved to"
	reasonPerGap       = "attaching=true is the whole reach gate (oncall.Mount); who may name whom is decided per gap inside the handler rather than per route, so it cannot be a flat mount role list"
)

// roleFreeWriteRoutes are the mutating routes that deliberately carry no
// mount-level role declaration, each with the reason its rule is not a
// flat role list. A route is here because reaching the Engagement or the
// Client is the whole gate (staffauth.AttachingWrite, or a Reader
// narrowing inside the handler), because the rule is an identity one
// rather than a role one, or because the rule is state-dependent and a
// single role list could not express it.
//
// One entry is none of those and says so: PUT /plan-templates/{planType}
// is a flat Owner-only rule still checked in-handler. It is outside
// #1028's named ten, so moving it is its own work (#1407) rather than
// this ticket's to widen into.
var roleFreeWriteRoutes = map[string]string{
	"POST /api/practices/{practiceId}/offers/{offerId}/accept":  reasonOwnOffer,
	"POST /api/practices/{practiceId}/offers/{offerId}/decline": reasonOwnOffer,
	"POST /api/practices/{practiceId}/push-subscriptions":       reasonOwnBrowser,
	"DELETE /api/practices/{practiceId}/push-subscriptions":     reasonOwnBrowser,

	"POST /api/practices/{practiceId}/engagements/{engagementId}/coverage-gaps":           reasonPerGap,
	"PUT /api/practices/{practiceId}/engagements/{engagementId}/coverage-gaps/{gapId}":    reasonPerGap,
	"DELETE /api/practices/{practiceId}/engagements/{engagementId}/coverage-gaps/{gapId}": reasonPerGap,

	"POST /api/practices/{practiceId}/engagements/{engagementId}/messages":                   reasonReachNotRole,
	"POST /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}":           reasonReachNotRole,
	"PUT /api/practices/{practiceId}/engagements/{engagementId}/plans/{planType}":            reasonReachNotRole,
	"POST /api/practices/{practiceId}/engagements/{engagementId}/contract/invoices":          reasonReachNotRole,
	"POST /api/practices/{practiceId}/engagements/{engagementId}/visits":                     reasonReachNotRole,
	"PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}":          reasonReachNotRole,
	"PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}/schedule": reasonReachNotRole,
	"PATCH /api/practices/{practiceId}/engagements/{engagementId}/visits/{visitId}/notes":    reasonReachNotRole,

	"POST /api/practices/{practiceId}/engagements/{engagementId}/portal-invite": "open to any Staff member at the Practice: portalinvite.InviteHandler asserts no role of its own, and inviting the Client to the portal is one of #350's write surfaces exempt from AttachingWrite by name",

	"PATCH /api/practices/{practiceId}/engagements/{engagementId}/status":      "ADR-0015's six-move table decides per move who may make it (engagement.TransitionHandler: an Owner or Admin for a reopen, an Owner or Admin or Doula for the rest), so the rule is move-dependent and no single mount role list expresses it",
	"PUT /api/practices/{practiceId}/engagements/{engagementId}/birth-outcome": "ADR-0015's role table, narrowed again once the Engagement is frozen (outcome.go's frozen-and-not-Owner branch): state-dependent, so no single mount role list expresses it",
	"PUT /api/practices/{practiceId}/engagements/{engagementId}/kind":          "ADR-0015's role table (engagement.refuseFactWrite): an Owner, an Admin or an employee Doula, but not a contractor Doula who also holds the doula role -- a role-and-employment-type rule a flat mount role list cannot express",

	"POST /api/practices/{practiceId}/clients":                                  "any Staff member but a contractor Doula, refused in-handler through staffauth.Reader (client.Mount): a contractor narrowing, not one of ADR-0008's role seats",
	"POST /api/practices/{practiceId}/clients/{clientId}/engagement-requests":   "any Staff member but a contractor Doula (engagementrequest.Mount), and enforced independently by engagement_requests_insert's own RLS policy: a contractor narrowing, not a role seat",
	"POST /api/practices/{practiceId}/engagement-requests/{requestId}/withdraw": "the requester alone (UPDATE ... WHERE requested_by = $1): an identity rule, not a role one",
	"PUT /api/practices/{practiceId}/clients/{clientId}":                        "any Staff member, narrowed to the Clients she can reach by reader.CanAccessClient inside the handler (client.Mount): a row narrowing, not a role list",
	"POST /api/practices/{practiceId}/clients/{clientId}/merge":                 "any Staff member, narrowed the same way the Client edit beside it is -- reader.CanAccessClient in-handler, which 404s rather than 403s",

	// Not a reach, identity or state rule: a flat Owner-only check
	// (plans.PutTemplateHandler's own hasOwnerRole) that belongs at the
	// mount exactly the way #1028's ten now declare theirs. It is not one
	// of the ten, so it is listed rather than moved here; see #1407.
	"PUT /api/practices/{practiceId}/plan-templates/{planType}": "Owner-only, still checked in-handler rather than declared at the mount -- outside #1028's named ten, so its move is its own work, #1407",
}

// TestRoutes_EveryMutatingWriteDeclaresRolesOrIsExempt inverts what
// #970, #990 and #1016 each left behind: three per-ticket allowlists
// (contractWriteRoutes, paymentRateWriteRoutes,
// bookingAndSettingsWriteRoutes), each naming only the routes one ticket
// happened to fix. An allowlist proves nothing about a write nobody
// wrote down, so a new mutating route that role-gates only in-handler
// passed all three by never being mentioned.
//
// This walks the whole registry instead. Every route
// idempotency.Router knows must either carry a mount-level role
// declaration (ExemptGated / ReplayableGated, which staffauth.GatedRouter
// .GatedWrite enforces and panics on an empty list for) or appear in
// roleFreeWriteRoutes below with a reason. A route on neither side fails
// here on the day it is written, without anyone remembering to add it to
// a list.
func TestRoutes_EveryMutatingWriteDeclaresRolesOrIsExempt(t *testing.T) {
	_, _, irRoutes := routes(testDeps())

	seen := make(map[string]bool, len(roleFreeWriteRoutes))
	for _, route := range irRoutes {
		reason, exempted := roleFreeWriteRoutes[route.Pattern]
		if exempted {
			seen[route.Pattern] = true
		}
		if len(route.Roles) > 0 {
			if exempted {
				t.Errorf("route %q declares roles at the mount but is still listed in roleFreeWriteRoutes -- remove the stale exemption", route.Pattern)
			}
			continue
		}
		if !exempted {
			t.Errorf("mutating route %q carries no mount-level role declaration -- register it through ir.ExemptGated/ir.ReplayableGated with its role list, or add it to roleFreeWriteRoutes with a reason saying why its rule is not a flat role list.", route.Pattern)
			continue
		}
		if reason == "" {
			t.Errorf("role-free route %q carries no reason", route.Pattern)
		}
	}
	for pattern := range roleFreeWriteRoutes {
		if !seen[pattern] {
			t.Errorf("role-free route %q is not in the registry -- did it move, or stop being mounted through idempotency.Router?", pattern)
		}
	}
	if len(irRoutes) == 0 {
		t.Fatal("found zero mutating routes in the idempotency registry -- did registerPracticeRoutes stop wiring feature Mounts?")
	}
}
