package main

import (
	"regexp"
	"strings"
	"testing"
)

// exemptEngagementWriteRoutes are mutating {engagementId} routes mux
// registers directly, without going through staffauth.AttachingWrite --
// #350's write-side mirror of exemptGETRoutes above. Each is a route
// outside ADR-0008's write table (Visits, Messages, Plan Instances,
// Contract actions) or outside the Staff population that table governs,
// named here with a reason for the same purpose exemptGETRoutes exists:
// so TestRoutes_NoEngagementWriteBypassesTheAttachingWriteGate sees a
// deliberate declaration instead of an absence.
// reasonPortalWrite is shared by every /api/portal/... exemption below --
// goconst's threshold for one literal repeated across the map.
const reasonPortalWrite = "clientauth.Middleware-scoped portal write, not a Staff population write ADR-0008's write table governs"

var exemptEngagementWriteRoutes = map[string]string{
	"POST /api/practices/{practiceId}/engagements/{engagementId}/complete":          "engagement.CompleteHandler runs its own ADR-0008 cascade (ending every open attachment); it is an Engagement lifecycle transition, not one of #350's four named write surfaces",
	"POST /api/practices/{practiceId}/engagements/{engagementId}/contract/invoices": "payments.PostInvoiceHandler is open to any Staff with practice access by design (#68); it is not one of #350's four named write surfaces",
	"POST /api/practices/{practiceId}/engagements/{engagementId}/offers":            "offer.CreateHandler is the Practice side of the Offer flow (Owner/Admin, per its own mount comment in main.go); it is not one of #350's four named write surfaces",
	"POST /api/practices/{practiceId}/engagements/{engagementId}/portal-invite":     "portalinvite.InviteHandler invites the Client to the portal; it is not one of #350's four named write surfaces",
	"POST /api/portal/engagements/{engagementId}/contract/sign":                     reasonPortalWrite,
	"POST /api/portal/engagements/{engagementId}/messages":                          reasonPortalWrite,
	"POST /api/portal/engagements/{engagementId}/push-subscriptions":                reasonPortalWrite,
	"DELETE /api/portal/engagements/{engagementId}/push-subscriptions":              reasonPortalWrite,
}

// muxHandleEngagementWrite finds the start of every mux.Handle
// registration in this package's own source for a mutating verb whose
// route pattern carries {engagementId} -- the route files' own text, not
// a running server, mirroring muxGetPattern's approach above.
var muxHandleEngagementWrite = regexp.MustCompile(`mux\.Handle\(\s*"(POST|PUT|PATCH|DELETE) ([^"]*\{engagementId\}[^"]*)"`)

// balancedParenStatement returns the text of the parenthesized call
// starting at the "(" found at or after openFrom in text, e.g. the full
// `mux.Handle("POST ...", staffauth.Middleware(db)(staffauth.AttachingWrite(...)))`
// statement -- so a search for "staffauth.AttachingWrite(" only matches
// within the one registration it actually wires, not into whatever
// registration or comment happens to follow it in the source.
func balancedParenStatement(text string, openFrom int) string {
	start := strings.Index(text[openFrom:], "(") + openFrom
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return text[start:]
}

// TestRoutes_NoEngagementWriteBypassesTheAttachingWriteGate is #350's
// write-side mirror of TestRoutes_NoGETBypassesTheGate: every mutating
// {engagementId} route mux.Handle registers must be wrapped in
// staffauth.AttachingWrite -- the seam that, since #350, refuses an
// unattached contractor's write before the wrapped handler ever runs --
// or be declared exempt above, by name, with a reason. This is what
// closes the hole a future Engagement-scoped write could otherwise fall
// into by never being wrapped at all.
func TestRoutes_NoEngagementWriteBypassesTheAttachingWriteGate(t *testing.T) {
	text := packageSource(t)

	matches := muxHandleEngagementWrite.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		t.Fatal("found zero mutating {engagementId} mux registrations in this package -- did the regex stop matching the source?")
	}
	for _, m := range matches {
		pattern := text[m[2]:m[3]] + " " + text[m[4]:m[5]]
		stmt := balancedParenStatement(text, m[0])

		if strings.Contains(stmt, "staffauth.AttachingWrite(") {
			continue
		}
		reason, exempted := exemptEngagementWriteRoutes[pattern]
		if exempted {
			if reason == "" {
				t.Errorf("exempt route %q carries no reason", pattern)
			}
			continue
		}
		t.Errorf("route %q mutates an Engagement without staffauth.AttachingWrite -- ADR-0008's write table gets no attachment enforcement for it. Wrap it in staffauth.AttachingWrite, or add it to exemptEngagementWriteRoutes with a reason.", pattern)
	}
}
