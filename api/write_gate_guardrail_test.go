package main

import (
	"strings"
	"testing"
)

// exemptEngagementWriteRoutes are mutating {engagementId} routes
// registered without attaching=true (staffauth.AttachingWrite) --
// #350's write-side gate. Each is a route outside ADR-0008's write table
// (Visits, Messages, Plan Instances, Contract actions) or outside the
// Staff population that table governs, named here with a reason so
// TestRoutes_NoEngagementWriteBypassesTheAttachingWriteGate sees a
// deliberate declaration instead of an absence.
//
// reasonPortalWrite is shared by every /api/portal/... exemption below --
// goconst's threshold for one literal repeated across the map.
const reasonPortalWrite = "clientauth.Middleware-scoped portal write, not a Staff population write ADR-0008's write table governs"

var exemptEngagementWriteRoutes = map[string]string{
	"POST /api/practices/{practiceId}/engagements/{engagementId}/complete":          "engagement.CompleteHandler runs its own ADR-0008 cascade (ending every open attachment); it is an Engagement lifecycle transition, not one of #350's four named write surfaces",
	"POST /api/practices/{practiceId}/engagements/{engagementId}/contract/invoices": "payments.PostInvoiceHandler is open to any Staff with practice access by design (#68); it is not one of #350's four named write surfaces",
	"POST /api/practices/{practiceId}/engagements/{engagementId}/offers":            "offer.CreateHandler is the Practice side of the Offer flow (Owner/Admin, per its own Mount comment); it is not one of #350's four named write surfaces",
	"POST /api/practices/{practiceId}/engagements/{engagementId}/portal-invite":     "portalinvite.InviteHandler invites the Client to the portal; it is not one of #350's four named write surfaces",
	"POST /api/portal/engagements/{engagementId}/contract/sign":                     reasonPortalWrite,
	"POST /api/portal/engagements/{engagementId}/messages":                          reasonPortalWrite,
	"POST /api/portal/engagements/{engagementId}/push-subscriptions":                reasonPortalWrite,
	"DELETE /api/portal/engagements/{engagementId}/push-subscriptions":              reasonPortalWrite,
	"PUT /api/portal/engagements/{engagementId}/notification-preference":            reasonPortalWrite,
}

// TestRoutes_NoEngagementWriteBypassesTheAttachingWriteGate is #350's
// write-side gate: every mutating {engagementId} route this binary
// registers must carry attaching=true (staffauth.AttachingWrite, ADR-0008's
// write-side seam that refuses an unattached contractor's write before
// the wrapped handler ever runs) or be declared exempt above, by name,
// with a reason.
//
// #836 moved every feature's registration into that feature's own Mount,
// so a source scan across this package's files can no longer see them
// all. This walks the real registries routes() builds instead: g.Routes()
// carries every mutating {engagementId} route regardless of which door
// mounted it (idempotency.Router's Replayable/Exempt, which mount
// through GatedRouter.Write, or a direct g.Write for a portal route),
// and idempotency.Router's own registry carries the Attaching bit for
// the ones that went through it.
func TestRoutes_NoEngagementWriteBypassesTheAttachingWriteGate(t *testing.T) {
	_, gRoutes, irRoutes := routes(testDeps())

	attaching := make(map[string]bool, len(irRoutes))
	for _, route := range irRoutes {
		attaching[route.Pattern] = route.Attaching
	}

	found := 0
	for _, route := range gRoutes {
		if !route.Write || !strings.Contains(route.Pattern, "{engagementId}") {
			continue
		}
		found++
		pattern := route.Method + " " + route.Pattern
		if attaching[pattern] {
			continue
		}
		if reason, exempted := exemptEngagementWriteRoutes[pattern]; exempted {
			if reason == "" {
				t.Errorf("exempt route %q carries no reason", pattern)
			}
			continue
		}
		t.Errorf("route %q mutates an Engagement without staffauth.AttachingWrite -- ADR-0008's write table gets no attachment enforcement for it. Register it with attaching=true, or add it to exemptEngagementWriteRoutes with a reason.", pattern)
	}
	if found == 0 {
		t.Fatal("found zero mutating {engagementId} routes in g.Routes() -- did the registries stop reporting them?")
	}
}
