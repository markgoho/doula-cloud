package main

import (
	"os"
	"regexp"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/push"
)

// exemptGETRoutes are the GET routes routes() registers directly on the
// mux rather than through GatedRouter -- every one of them sits outside
// staffauth.Middleware entirely (a bootstrap route with no session yet,
// or a clientauth.Middleware-scoped portal route), so ADR-0008's read
// table, which is about a Staff member's reach, has nothing to say about
// them. #315's ACs are scoped to "every GET behind staffauth.Middleware";
// this list is the explicit, reviewable boundary of that scope. Adding a
// new entry here should always come with a reason, the same way
// staffauth.AnyStaff must be named rather than defaulted into.
var exemptGETRoutes = map[string]bool{
	"GET /api/hello":                                                             true, // no auth at all -- a health probe
	"GET /api/staff/session":                                                     true, // lists a person's own memberships before any :practiceId is chosen
	"GET /api/portal/session":                                                    true, // clientauth.Middleware, not staffauth -- a different population
	"GET /api/portal/engagements/{engagementId}":                                 true,
	"GET /api/portal/engagements/{engagementId}/birth-plan":                      true,
	"GET /api/portal/engagements/{engagementId}/contract":                        true,
	"GET /api/portal/engagements/{engagementId}/contract/pdf":                    true,
	"GET /api/portal/engagements/{engagementId}/messages":                        true,
	"GET /api/portal/engagements/{engagementId}/messages/{messageId}/attachment": true,
}

// muxGetPattern finds every GET route registered directly on the raw mux
// (mux.Handle or mux.HandleFunc) in main.go's source -- the routes()
// function's own text, not a running server, since Go's http.ServeMux
// has no public API to enumerate its registered patterns.
var muxGetPattern = regexp.MustCompile(`mux\.(?:Handle|HandleFunc)\(\s*"(GET [^"]+)"`)

// TestRoutes_EveryDeclaredGETHasRoleDeclaration is the rlsguardrail-shaped
// assertion #315's AC asks for, run against the real registry routes()
// builds: every GET GatedRouter mounted carries a non-empty role
// declaration. GatedRouter.Get already panics before an empty one could
// reach this table, so this test is the belt to that braces --
// catastrophic failure (a panic taking the whole binary down) and an
// ordinary failing test should both catch the same mistake.
func TestRoutes_EveryDeclaredGETHasRoleDeclaration(t *testing.T) {
	_, registry := routes(authntest.Verifier{}, nil, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), "whsec_test", payments.NewFakeClient(), "whsec_connect_test", "whsec_account_test", testWorker, testWorkerSecret, "mailgun_webhook_test_key", testLowCreditWorker, testPayoutOutboxWorker, testPaymentOutboxWorker, testSessionNoticeOutboxWorker, testStaffInviteOutboxWorker, testNudgeEnqueuer, []string{testExpectedOrigin})
	if len(registry) == 0 {
		t.Fatal("routes() registered zero GETs through GatedRouter -- did routes() stop wiring g.Get calls?")
	}
	for _, route := range registry {
		if len(route.Roles) == 0 {
			t.Errorf("route %q has no role declaration", route.Pattern)
		}
	}
}

// TestRoutes_NoGETBypassesTheGate is #231's other finding made concrete:
// GatedRouter's startup panic only fires for a route someone actually
// declares through it. Nothing stops a future change from registering a
// GET straight on mux, skipping the gate (and the panic) entirely. This
// test closes that hole by scanning main.go's own source for every direct
// mux GET registration and failing if one isn't accounted for by either
// the GatedRouter registry or exemptGETRoutes above.
func TestRoutes_NoGETBypassesTheGate(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}

	_, registry := routes(authntest.Verifier{}, nil, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), "whsec_test", payments.NewFakeClient(), "whsec_connect_test", "whsec_account_test", testWorker, testWorkerSecret, "mailgun_webhook_test_key", testLowCreditWorker, testPayoutOutboxWorker, testPaymentOutboxWorker, testSessionNoticeOutboxWorker, testStaffInviteOutboxWorker, testNudgeEnqueuer, []string{testExpectedOrigin})
	gated := make(map[string]bool, len(registry))
	for _, route := range registry {
		gated["GET "+route.Pattern] = true
	}

	matches := muxGetPattern.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("found zero direct mux GET registrations in main.go -- did the regex stop matching the source?")
	}
	for _, match := range matches {
		pattern := match[1]
		if gated[pattern] || exemptGETRoutes[pattern] {
			continue
		}
		t.Errorf("route %q is registered directly on mux, bypassing GatedRouter and undeclared in exemptGETRoutes -- mount it through g.Get with a role declaration, or add it to exemptGETRoutes with a reason", pattern)
	}
}
