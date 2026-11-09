package staffauth_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// Named once so goconst does not see independent literals across this
// package's tests.
const (
	notJSON      = "not json"
	notAUUID     = "not-a-uuid"
	noStaffRow   = "a session with no staff row"
	freshToken   = "fresh-id-token"
	firstNameKey = "firstName"
	lastNameKey  = "lastName"
	workStateKey = "workState"
)

// env is what a handler test in this package starts from once a Staff
// member is signed in: a Practice with this package's whole surface
// mounted through staffauth.Mount, the same call main.go makes on the real
// GatedRouter -- because the Owner/Admin-vs-Doula boundary lives at that
// mount and not inside the handlers -- and one Staff member at the screen,
// an employee Doula unless a test says otherwise. A test that runs before
// anyone is signed in (signup, login, reset) starts from its own server.
type env struct {
	*fixture.Scene
	accounts *authntest.FakeAccountManager
}

type envSpec struct {
	scene    []fixture.SceneOption
	verifier authn.Verifier
	accounts *authntest.FakeAccountManager
}

type envOption func(*envSpec)

func viaScene(opts ...fixture.SceneOption) envOption {
	return func(s *envSpec) { s.scene = append(s.scene, opts...) }
}

// signedInAs shapes the Staff member who is at the screen.
func signedInAs(opts ...fixture.StaffOption) envOption {
	return viaScene(fixture.SignedInAs(opts...))
}

// withVerifier swaps the Identity Platform token verifier.
func withVerifier(verifier authn.Verifier) envOption {
	return func(s *envSpec) { s.verifier = verifier }
}

// withAccounts swaps the Identity Platform account manager, for a test
// that seeds or inspects accounts.
func withAccounts(accounts *authntest.FakeAccountManager) envOption {
	return func(s *envSpec) { s.accounts = accounts }
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	spec := envSpec{verifier: authntest.Verifier{}, accounts: authntest.NewFakeAccountManager()}
	for _, opt := range opts {
		opt(&spec)
	}
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, app *sql.DB) {
		staffauth.Mount(g, ir, app, spec.verifier, spec.accounts, tasknudge.NoOpEnqueuer{}, neverSuppressed)
	}
	return &env{Scene: fixture.NewScene(t, mount, spec.scene...), accounts: spec.accounts}
}

// serveStaffauth mounts this package's whole surface through
// staffauth.Mount on a server of its own, for a test that seeds by hand
// and signs in with a session it mints itself.
func serveStaffauth(db *testdb.DB, verifier authn.Verifier, accounts authn.AccountManager) *httptest.Server {
	return httptest.NewServer(staffauthMux(db, verifier, accounts))
}

// staffauthMux is serveStaffauth's handler, for a test that wraps it.
func staffauthMux(db *testdb.DB, verifier authn.Verifier, accounts authn.AccountManager) *http.ServeMux {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, verifier, accounts, tasknudge.NoOpEnqueuer{}, neverSuppressed)
	return mux
}

// membershipURL is the route of staffID's Membership at the Practice.
func (e *env) membershipURL(staffID string) string {
	return e.PracticeURL("/staff/" + staffID + "/membership")
}

// confirmed marks a request as the second, deliberate one a destructive
// act waits for.
var confirmed = reqtest.Header("X-Confirmed", "true")
