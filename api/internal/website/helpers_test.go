package website_test

import (
	"database/sql"
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/website"
)

// ownerName is the Owner every handler test seeds, named once because
// the response echoes it back and several tests assert on it.
const ownerName = "Maya Chen"

// Fixture answers, shared because goconst counts repeats and because a
// test reading "the same words as last time" should be looking at the
// same literal.
const (
	policyText = "Two weeks' notice."
	// The slug the Practice's name mints (00046).
	firstSlug    = "rochester-doulas"
	practiceName = "Rochester Doulas"
	ownSiteURL   = "https://rochesterdoulas.com"

	hostedBody = `{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"Two weeks' notice."}`
	ownBody    = `{"mode":"own","ownUrl":"https://rochesterdoulas.com"}`
)

// env is what a test in this package starts from: a Practice named
// Rochester Doulas with this package's whole surface mounted on a server,
// and its Owner, Maya Chen, at the screen -- the only role that may write
// a website declaration. A test says only what it changes, with
// signedInAs.
type env struct {
	*fixture.Scene
}

type envOption func(*[]fixture.StaffOption)

// signedInAs replaces the Owner at the screen with a Staff member of
// these options.
func signedInAs(opts ...fixture.StaffOption) envOption {
	return func(s *[]fixture.StaffOption) { *s = opts }
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	staff := []fixture.StaffOption{fixture.Owner(), fixture.StaffName(ownerName)}
	for _, opt := range opts {
		opt(&staff)
	}
	return &env{fixture.NewScene(t, mount,
		fixture.InPractice(fixture.PracticeName(practiceName)), fixture.SignedInAs(staff...))}
}

// mount mounts this package's whole surface through website.Mount, the
// same call main.go makes on the real GatedRouter and
// idempotency.Router -- so the Owner gate and the role gate are the real
// ones and not a test's approximation.
func mount(g *staffauth.GatedRouter, ir *idempotency.Router, _ *sql.DB) {
	website.Mount(g, ir, &tasknudge.FakeEnqueuer{})
}

// websiteURL is the Practice's website route.
func (e *env) websiteURL() string {
	return e.PracticeURL("/website")
}

// rawBody sends body as it is written, with no content type, the way the
// handler reads it.
func rawBody(body string) reqtest.Option {
	return reqtest.Body("", []byte(body))
}

// put saves body as the Staff member at the screen.
func (e *env) put(body string) website.Response {
	e.T.Helper()
	return reqtest.Expect[website.Response](e.T, http.MethodPut, e.websiteURL(), http.StatusOK, e.SignedIn, rawBody(body))
}

// get reads the declaration as who.
func (e *env) get(who reqtest.Option) website.Response {
	e.T.Helper()
	return reqtest.Expect[website.Response](e.T, http.MethodGet, e.websiteURL(), http.StatusOK, who)
}
