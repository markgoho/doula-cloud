package staffauth_test

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// strandedUID is an Identity Platform uid holding a live Staff session
// and no `staff` row at all -- the caller every route in this file's
// walk has to refuse, and the exact caller #1024 found a route minting
// tokens for.
const strandedUID = "staff-session-with-no-staff-row"

// selfRoute is what a pre-Practice route needs to be driven far enough
// to reach its own self-resolution: the body an earlier decode or
// validation step demands, so that a 400 cannot answer in the guard's
// place and let the assertion pass for the wrong reason.
type selfRoute struct {
	body   string
	bearer bool // reads a Bearer ID token (authn.BeginBootstrap) rather than the session cookie
}

// selfResolvingRoutes is the pre-Practice family this package owes one
// answer: a route here resolves the caller's own `staff` row before it
// does anything, and refuses with 404 MsgNoMatchingStaffAccount when
// there is none.
//
// Written out rather than derived, the same reason staffFamilyGroups is:
// a route added to mountSessionRoutes and to neither this map nor
// notSelfResolving below fails TestEveryPrePracticeRouteRefusesAStrandedSession,
// so the next person to mount one has to say which of the two it is
// before the build goes green. That is the whole enforcement against a
// new route quietly skipping the guard.
var selfResolvingRoutes = map[string]selfRoute{
	"GET /api/staff/session":                          {},
	"PUT /api/staff/work-state":                       {body: `{"workState":"NY"}`},
	"PUT /api/staff/email":                            {body: `{"newEmail":"moved@example.com"}`},
	"POST /api/staff/verify-email/request":            {},
	"POST /api/staff/mfa-recovery/saved-codes/rotate": {},
	"POST /api/staff/mfa":                             {bearer: true},
	"DELETE /api/staff/mfa":                           {},
	"DELETE /api/staff/account":                       {},
}

// notSelfResolving is the other half of the registry: a pre-Practice
// route that legitimately never asks "which Staff person is calling?",
// with the reason it does not, so that leaving a route out of the guard
// is a written decision rather than an omission.
var notSelfResolving = map[string]string{
	"POST /api/staff/signup":                 "creates the staff row rather than resolving one -- there is deliberately no row yet",
	"POST /api/staff/accept-invite":          "creates or claims the staff row from the invitation's own token, for the same reason as signup",
	"POST /api/staff/verify-email":           "pre-account: the emailed link's token is the whole credential and there is no session to resolve an identity from",
	"POST /api/staff/password-reset/request": "public and pre-account, and must answer identically whether or not the address exists (#168)",
	"POST /api/staff/password-reset":         "pre-account, same shape as the verification spend above",
	"POST /api/staff/mfa-recovery/spend":     "unauthenticated by construction -- a locked-out person cannot sign in first (#605), so the code itself names whose row it is",
}

// TestEveryPrePracticeRouteRefusesAStrandedSession walks GatedRouter's
// own registry, not a hand-kept list, and drives every self-resolving
// pre-Practice route with a credential for a uid that has no `staff`
// row. One status, one message, from all of them: the answer the app's
// no-practice screen and its sign-in specs already key on.
func TestEveryPrePracticeRouteRefusesAStrandedSession(t *testing.T) {
	db := testdb.New(t)
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	// One verifier for the whole mount: the bootstrap routes in this
	// family read a Bearer ID token, and it has to verify to the same
	// stranded uid the session cookie names.
	staffauth.Mount(g, ir, db.App, authntest.Verifier{UID: strandedUID, SecondFactor: true},
		authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	token := authntest.SeedSession(t, db.App, strandedUID)

	walked := 0
	unwalked := maps.Clone(selfResolvingRoutes)
	unmounted := maps.Clone(notSelfResolving)

	for _, route := range g.Routes() {
		if !strings.HasPrefix(route.Pattern, staffFamilyPrefix) {
			continue
		}
		walked++
		name := route.Method + " " + route.Pattern
		spec, resolves := selfResolvingRoutes[name]
		if _, excused := notSelfResolving[name]; excused {
			delete(unmounted, name)
			if resolves {
				t.Errorf("%s is in both selfResolvingRoutes and notSelfResolving -- it is one or the other", name)
			}
			continue
		}
		if !resolves {
			t.Errorf("%s is mounted in mountSessionRoutes but neither selfResolvingRoutes nor notSelfResolving names it -- say whether it resolves the caller's own staff row, or why it does not", name)
			continue
		}
		delete(unwalked, name)

		t.Run(name, func(t *testing.T) {
			resp := callWithStrandedSession(t, srv, route.Method, route.Pattern, token, spec)
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want %d -- a pre-Practice route answers 404 when the caller's identity resolves to no staff row; a bare 403 renders on the app's error page as \"your role does not have permission\", which is not what happened", resp.StatusCode, http.StatusNotFound)
			}
			if got := apierrtest.Decode(t, resp).Message; got != staffauth.MsgNoMatchingStaffAccount {
				t.Errorf("message = %q, want %q", got, staffauth.MsgNoMatchingStaffAccount)
			}
		})
	}

	if walked != len(selfResolvingRoutes)+len(notSelfResolving) {
		t.Errorf("walked %d routes under %s, want %d", walked, staffFamilyPrefix, len(selfResolvingRoutes)+len(notSelfResolving))
	}
	for name := range unwalked {
		t.Errorf("selfResolvingRoutes names %s, which is not mounted -- the table has outlived the route", name)
	}
	for name := range unmounted {
		t.Errorf("notSelfResolving names %s, which is not mounted -- the table has outlived the route", name)
	}
}

// callWithStrandedSession issues one request at the route under test,
// carrying whichever credential that route reads for the uid with no
// staff row, plus the body and headers an earlier guard would otherwise
// refuse on.
func callWithStrandedSession(t *testing.T, srv *httptest.Server, method, pattern, token string, spec selfRoute) *http.Response {
	t.Helper()
	body := spec.body
	if body == "" {
		body = "{}"
	}
	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+pattern, strings.NewReader(body))
	if err != nil {
		// coverage:ignore reason: a malformed request built from the router's own registry, not reachable from a test
		t.Fatalf("build %s %s: %v", method, pattern, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Confirmed", "true")
	if spec.bearer {
		req.Header.Set("Authorization", "Bearer stranded")
	}
	authntest.AddSessionCookie(req, token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// coverage:ignore reason: a transport failure against this test's own httptest server
		t.Fatalf("%s %s: %v", method, pattern, err)
	}
	return resp
}
