package staffauth_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
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
	enq      tasknudge.Enqueuer
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

// withEnqueuer swaps the worker nudge.
func withEnqueuer(enq tasknudge.Enqueuer) envOption {
	return func(s *envSpec) { s.enq = enq }
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	spec := envSpec{verifier: authntest.Verifier{}, accounts: authntest.NewFakeAccountManager(), enq: tasknudge.NoOpEnqueuer{}}
	for _, opt := range opts {
		opt(&spec)
	}
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, app *sql.DB) {
		staffauth.Mount(g, ir, app, spec.verifier, spec.accounts, spec.enq, neverSuppressed)
	}
	return &env{Scene: fixture.NewScene(t, mount, spec.scene...), accounts: spec.accounts}
}

// membershipURL is the route of staffID's Membership at the Practice.
func (e *env) membershipURL(staffID string) string {
	return e.PracticeURL("/staff/" + staffID + "/membership")
}

// confirmed marks a request as the second, deliberate one a destructive
// act waits for.
var confirmed = reqtest.Header("X-Confirmed", "true")
