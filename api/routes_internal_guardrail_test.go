package main

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// wantInternalRoutes is every route this binary serves under
// /api/internal/**, written out rather than derived -- the boundary
// #1052 asked to have on the record, from the router rather than from
// memory. A route added here is a route reachable from the public
// internet on a service that carries allUsers -> roles/run.invoker
// (ADR-0037), so adding one should cost a line in this list.
var wantInternalRoutes = []string{
	"POST /api/internal/billing/founding-grants",
	"POST /api/internal/billing/refunds",
	"GET /api/internal/billing/dormant-practices",
	"POST /api/internal/clients/process-erasure-outbox",
	"POST /api/internal/notifications/process-connect-nudge-outbox",
	"POST /api/internal/notifications/process-engagement-request-outbox",
	"POST /api/internal/notifications/process-low-credit-outbox",
	"POST /api/internal/notifications/process-mfa-recovery-outbox",
	"POST /api/internal/notifications/process-offer-outbox",
	"POST /api/internal/notifications/process-outbox",
	"POST /api/internal/notifications/process-payment-outbox",
	"POST /api/internal/notifications/process-payout-outbox",
	"POST /api/internal/notifications/process-portal-address-change-outbox",
	"POST /api/internal/notifications/process-portal-magic-link-outbox",
	"POST /api/internal/notifications/process-practice-deletion-outbox",
	"POST /api/internal/notifications/process-session-notice-outbox",
	"POST /api/internal/notifications/process-staff-email-change-outbox",
	"POST /api/internal/notifications/process-staff-invite-outbox",
	"POST /api/internal/notifications/process-staff-token-mail-outbox",
	"POST /api/internal/outboxes/drain",
	"POST /api/internal/site/process-build-outbox",
	"POST /api/internal/site/verify-pages",
	"POST /api/internal/staffauth/mfa-recovery/support-clear",
}

// internalRoutes reads every /api/internal/** route out of the registry
// routes() builds, as "METHOD /path".
func internalRoutes(t *testing.T) []string {
	t.Helper()
	_, gRoutes, _ := routes(testDeps())

	found := make([]string, 0, len(wantInternalRoutes))
	for _, route := range gRoutes {
		if strings.HasPrefix(route.Pattern, "/api/internal/") {
			found = append(found, route.Method+" "+route.Pattern)
		}
	}
	sort.Strings(found)
	return found
}

// TestInternalRoutes_AreTheOnesOnTheRecord is #1052's first acceptance
// criterion as a test rather than a paragraph: the list of what is
// reachable under /api/internal/** is read from the router, so it cannot
// quietly grow.
func TestInternalRoutes_AreTheOnesOnTheRecord(t *testing.T) {
	want := append([]string(nil), wantInternalRoutes...)
	sort.Strings(want)

	got := internalRoutes(t)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("internal routes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestInternalRoutes_EveryOneRefusesAnUnauthenticatedCaller is the
// boundary itself. doula-api carries allUsers -> roles/run.invoker, so
// every one of these addresses answers the public internet; the guard in
// front of each is the only thing between the internet and, among other
// things, the outbox drain.
//
// The request carries no Origin header, which csrf.Wrap lets through
// ("no Origin, no rejection"), and no credentials of any kind -- so a
// route that answers anything but 401 is a route that lost its guard.
func TestInternalRoutes_EveryOneRefusesAnUnauthenticatedCaller(t *testing.T) {
	handler, _, _ := routes(testDeps())

	routePatterns := internalRoutes(t)
	if len(routePatterns) == 0 {
		t.Fatal("found zero /api/internal routes -- did registerInternalRoutes stop wiring them?")
	}

	for _, pattern := range routePatterns {
		t.Run(pattern, func(t *testing.T) {
			method, path, _ := strings.Cut(pattern, " ")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader("{}")))

			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s answered %d to a request carrying nothing, want %d", pattern, w.Code, http.StatusUnauthorized)
			}
		})
	}
}

// TestInternalRoutes_RefuseTheHeaderWhenNoSecretIsConfigured is the
// production posture: Cloud Run sets no NOTIFICATION_WORKER_SECRET, so
// the header mechanism is off there and only a caller identity gets in.
// An empty configured secret must refuse an empty header rather than
// matching it.
func TestInternalRoutes_RefuseTheHeaderWhenNoSecretIsConfigured(t *testing.T) {
	deps := testDeps()
	deps.InternalAuth = internalGuard(func(string) string { return "" })
	handler, _, _ := routes(deps)

	for _, header := range []string{"", "guessed-secret"} {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/internal/outboxes/drain", strings.NewReader("{}"))
		r.Header.Set("X-Internal-Secret", header)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("drain answered %d to X-Internal-Secret %q with no secret configured, want %d", w.Code, header, http.StatusUnauthorized)
		}
	}
}
