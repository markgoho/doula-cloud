package staffauth_test

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// staffFamilyPrefix is the path every pre-Practice Staff route shares --
// the family #1024 is about, and the filter that separates it from
// mountPracticeRoutes' /api/practices/{practiceId}/... half in the same
// registry.
const staffFamilyPrefix = "/api/staff/"

// routeGroup names which of mountSessionRoutes' three groups a route
// belongs to. The groups are that doc comment's own, restated as data so
// the walk below can assert the right thing about each one instead of
// settling for "not a 2xx" everywhere.
type routeGroup int

const (
	// groupSession is a route that reads the __session cookie through
	// authn.Begin, and is therefore refused outright when the cookie
	// names a session in the other population.
	groupSession routeGroup = iota
	// groupBootstrap is a route that reads a Bearer ID token through
	// authn.BeginBootstrap and no session at all. Identity Platform
	// cannot mint a uid carrying portalaccount.Prefix, so the credential
	// is a Staff one by construction; a Portal cookie riding along is
	// #816's eviction question, not this one.
	groupBootstrap
	// groupPreAccount is a route that reads neither a session nor a
	// Bearer token -- the emailed link's own single-purpose authtoken is
	// the whole credential, and it is minted only for Staff.
	groupPreAccount
)

// staffFamilyGroups classifies every route mountSessionRoutes registers.
// It is written out rather than derived, which is the point: a route
// added to that function and not to this table fails
// TestStaffFamilyRefusesAPortalSession below, so the next person to
// mount a pre-Practice Staff route has to say which enforcement it is
// relying on before the build goes green.
var staffFamilyGroups = map[string]routeGroup{
	"GET /api/staff/session":                          groupSession,
	"PUT /api/staff/work-state":                       groupSession,
	"PUT /api/staff/email":                            groupSession,
	"POST /api/staff/verify-email/request":            groupSession,
	"POST /api/staff/mfa-recovery/saved-codes/rotate": groupSession,
	"DELETE /api/staff/mfa":                           groupSession,
	"DELETE /api/staff/account":                       groupSession,
	"POST /api/staff/signup":                          groupBootstrap,
	"POST /api/staff/accept-invite":                   groupBootstrap,
	"POST /api/staff/mfa":                             groupBootstrap,
	"POST /api/staff/verify-email":                    groupPreAccount,
	"POST /api/staff/password-reset/request":          groupPreAccount,
	"POST /api/staff/password-reset":                  groupPreAccount,
	"POST /api/staff/mfa-recovery/spend":              groupPreAccount,
}

// TestStaffFamilyRefusesAPortalSession is #1024's guardrail, and it is
// enumerated from GatedRouter's own registry rather than from a list
// somebody keeps by hand: a route mounted in mountSessionRoutes is in
// this walk the moment it is mounted.
//
// Every route in the family answers a live Client Portal session with a
// refusal. The seven that read the cookie answer authn.Begin's own 401,
// with no Set-Cookie -- a refused request must not walk away with a
// renewed session. The other seven read no session at all, so what they
// answer is whatever their own credential check says about a request
// that carries none; the assertion there is that it is never a success.
func TestStaffFamilyRefusesAPortalSession(t *testing.T) {
	db := testdb.New(t)
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, authntest.NewFakeAccountManager(),
		tasknudge.NoOpEnqueuer{}, neverSuppressed)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	token := authntest.SeedSession(t, db.App, portalaccount.NewIdentifier())

	walked := 0
	unclassified := maps.Clone(staffFamilyGroups)

	for _, route := range g.Routes() {
		if !strings.HasPrefix(route.Pattern, staffFamilyPrefix) {
			continue
		}
		walked++
		name := route.Method + " " + route.Pattern
		group, classified := staffFamilyGroups[name]
		if !classified {
			t.Errorf("%s is mounted in mountSessionRoutes but staffFamilyGroups does not classify it -- say which enforcement refuses a Client Portal session here", name)
			continue
		}
		delete(unclassified, name)

		t.Run(name, func(t *testing.T) {
			resp := callWithPortalSession(t, srv, route.Method, route.Pattern, token)
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode < http.StatusBadRequest {
				t.Fatalf("status = %d, want a refusal -- a Client Portal session reached %s", resp.StatusCode, name)
			}
			if group != groupSession {
				return
			}
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d -- a session-reading route must meet authn.Begin's own refusal",
					resp.StatusCode, http.StatusUnauthorized)
			}
			if got := apierrtest.Decode(t, resp).Message; got != authn.MsgInvalidSession {
				t.Errorf("message = %q, want %q -- the refusal must be indistinguishable from a cookie naming no session at all", got, authn.MsgInvalidSession)
			}
			if cookies := resp.Cookies(); len(cookies) != 0 {
				t.Errorf("Set-Cookie = %v, want none -- a refused request renewed the session it was refused for", cookies)
			}
		})
	}

	// Two controls, so the walk cannot pass by finding nothing. The first
	// catches a filter that stopped matching; the second catches a route
	// deleted from mountSessionRoutes while its classification stayed.
	if walked != len(staffFamilyGroups) {
		t.Errorf("walked %d routes under %s, want %d", walked, staffFamilyPrefix, len(staffFamilyGroups))
	}
	for name := range unclassified {
		t.Errorf("staffFamilyGroups classifies %s, which is not mounted -- the table has outlived the route", name)
	}
}

// callWithPortalSession issues one request at the route under test,
// carrying a live Client Portal session and nothing else a route could
// mistake for a Staff credential. The JSON body and X-Confirmed header
// are there so that no earlier guard -- a decode, RequireConfirmed --
// answers in place of the population check and lets the assertion pass
// for the wrong reason.
func callWithPortalSession(t *testing.T, srv *httptest.Server, method, pattern, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+pattern, strings.NewReader("{}"))
	if err != nil {
		// coverage:ignore reason: a malformed request built from the router's own registry, not reachable from a test
		t.Fatalf("build %s %s: %v", method, pattern, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Confirmed", "true")
	authntest.AddSessionCookie(req, token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// coverage:ignore reason: a transport failure against this test's own httptest server
		t.Fatalf("%s %s: %v", method, pattern, err)
	}
	return resp
}
