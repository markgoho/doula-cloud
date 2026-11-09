package website_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
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
	t   *testing.T
	db  *testdb.DB
	srv *httptest.Server

	Practice fixture.PracticeRecord
	Staff    fixture.StaffRecord

	signedIn reqtest.Option
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
	db := testdb.New(t)
	e := &env{t: t, db: db, Practice: fixture.Practice(t, db, fixture.PracticeName(practiceName))}
	e.Staff = fixture.Staff(t, db, e.Practice, staff...)
	e.srv = e.mount()
	e.signedIn = e.as(e.Staff)
	return e
}

// mount mounts this package's whole surface through website.Mount, the
// same call main.go makes on the real GatedRouter and
// idempotency.Router -- so the Owner gate and the role gate are the real
// ones and not a test's approximation.
func (e *env) mount() *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, e.db.App)
	ir := idempotency.NewRouter(g, e.db.App)
	website.Mount(g, ir, &tasknudge.FakeEnqueuer{})
	srv := httptest.NewServer(mux)
	e.t.Cleanup(srv.Close)
	return srv
}

// as signs a request in as who.
func (e *env) as(who fixture.StaffRecord) reqtest.Option {
	e.t.Helper()
	return reqtest.Session(fixture.Session(e.t, e.db, who))
}

// staff is another Staff member at the Practice.
func (e *env) staff(opts ...fixture.StaffOption) fixture.StaffRecord {
	e.t.Helper()
	return fixture.Staff(e.t, e.db, e.Practice, opts...)
}

// websiteURL is the Practice's website route.
func (e *env) websiteURL() string {
	return e.srv.URL + "/api/practices/" + e.Practice.ID + "/website"
}

// rawBody sends body as it is written, with no content type, the way the
// handler reads it.
func rawBody(body string) reqtest.Option {
	return reqtest.Body("", []byte(body))
}

// put saves body as the Staff member at the screen.
func (e *env) put(body string) website.Response {
	e.t.Helper()
	return reqtest.Expect[website.Response](e.t, http.MethodPut, e.websiteURL(), http.StatusOK, e.signedIn, rawBody(body))
}

// get reads the declaration as who.
func (e *env) get(who reqtest.Option) website.Response {
	e.t.Helper()
	return reqtest.Expect[website.Response](e.t, http.MethodGet, e.websiteURL(), http.StatusOK, who)
}

// exec runs one fixture statement on the superuser connection.
func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Admin.ExecContext(e.t.Context(), query, args...); err != nil {
		e.t.Fatalf("fixture %q: %v", query, err)
	}
}
