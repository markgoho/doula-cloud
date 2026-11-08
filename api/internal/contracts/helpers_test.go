package contracts_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// env is what a test in this package starts from: a Practice with this
// package's whole surface mounted on a server, one Staff member at the
// screen and one Engagement. With no option she is an employee Doula and
// the Engagement is an intake birth for a new Client; a test says only
// what it changes, with signedInAs, withEngagement, withClient and
// withStore. The same server answers Staff and the Client's portal, so
// asClient signs a Client in when she was seeded with a portal login.
type env struct {
	t   *testing.T
	db  *testdb.DB
	srv *httptest.Server

	store  objectstore.ObjectStore
	pusher *push.FakePusher

	Practice   fixture.PracticeRecord
	Staff      fixture.StaffRecord
	Engagement fixture.EngagementRecord

	signedIn reqtest.Option
	asClient reqtest.Option
}

type envSpec struct {
	practice   []fixture.PracticeOption
	staff      []fixture.StaffOption
	engagement []fixture.EngagementOption
	at         time.Time
	mounts     []func(*staffauth.GatedRouter, *idempotency.Router)
	client     []fixture.ClientOption
	store      objectstore.ObjectStore
	attached   bool
}

type envOption func(*envSpec)

// signedInAs shapes the Staff member who is at the screen.
func signedInAs(opts ...fixture.StaffOption) envOption {
	return func(s *envSpec) { s.staff = append(s.staff, opts...) }
}

// withEngagement shapes the Engagement the test is about.
func withEngagement(opts ...fixture.EngagementOption) envOption {
	return func(s *envSpec) { s.engagement = append(s.engagement, opts...) }
}

// withClient shapes the Engagement's Client.
func withClient(opts ...fixture.ClientOption) envOption {
	return func(s *envSpec) { s.client = append(s.client, opts...) }
}

// withPortalLogin lets the Engagement's Client sign in to the portal.
func withPortalLogin() envOption {
	return withClient(fixture.WithPortalLogin())
}

// withPractice shapes the Practice.
func withPractice(opts ...fixture.PracticeOption) envOption {
	return func(s *envSpec) { s.practice = append(s.practice, opts...) }
}

// atInstant fixes the instant every request is served at.
func atInstant(at time.Time) envOption {
	return func(s *envSpec) { s.at = at }
}

// alsoMounting mounts another package's routes beside this one's, for a
// test whose subject is how the two agree.
func alsoMounting(mount func(*staffauth.GatedRouter, *idempotency.Router)) envOption {
	return func(s *envSpec) { s.mounts = append(s.mounts, mount) }
}

// withStore swaps the object store the Signed PDF is written to and read from.
func withStore(store objectstore.ObjectStore) envOption {
	return func(s *envSpec) { s.store = store }
}

// attachedToEngagement grants the Staff member at the screen an open
// Attachment on the Engagement.
func attachedToEngagement() envOption {
	return func(s *envSpec) { s.attached = true }
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	spec := envSpec{store: objectstore.NewMemoryStore()}
	for _, opt := range opts {
		opt(&spec)
	}
	db := testdb.New(t)
	e := &env{t: t, db: db, store: spec.store, pusher: push.NewFakePusher(), Practice: fixture.Practice(t, db, spec.practice...)}
	e.Staff = fixture.Staff(t, db, e.Practice, spec.staff...)
	engagementOpts := spec.engagement
	if len(spec.client) > 0 {
		engagementOpts = append(engagementOpts, fixture.NewClient(spec.client...))
	}
	e.Engagement = fixture.Engagement(t, db, e.Practice, engagementOpts...)
	if spec.attached {
		testdb.SeedGrantedAttachment(t, db, e.Engagement.ID, e.Staff.ID)
	}
	e.srv = e.mount(spec.at, spec.mounts)
	e.signedIn = e.as(e.Staff)
	if e.Engagement.Client.PortalUID != "" {
		e.asClient = e.as(e.Engagement.Client)
	}
	return e
}

// mount mounts this package's whole surface through contracts.Mount, the
// same call main.go makes on the real GatedRouter and idempotency.Router.
func (e *env) mount(at time.Time, others []func(*staffauth.GatedRouter, *idempotency.Router)) *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, e.db.App)
	ir := idempotency.NewRouter(g, e.db.App)
	contracts.Mount(g, ir, e.db.App, e.store, e.pusher)
	for _, mount := range others {
		mount(g, ir)
	}
	var handler http.Handler = mux
	if !at.IsZero() {
		handler = clock.Middleware(func() time.Time { return at })(mux)
	}
	srv := httptest.NewServer(handler)
	e.t.Cleanup(srv.Close)
	return srv
}

// as signs a request in as who.
func (e *env) as(who fixture.Signer) reqtest.Option {
	e.t.Helper()
	return reqtest.Session(fixture.Session(e.t, e.db, who))
}

// staff is another Staff member at the Practice.
func (e *env) staff(opts ...fixture.StaffOption) fixture.StaffRecord {
	e.t.Helper()
	return fixture.Staff(e.t, e.db, e.Practice, opts...)
}

// anotherEngagement is a second Engagement at the Practice.
func (e *env) anotherEngagement(opts ...fixture.EngagementOption) fixture.EngagementRecord {
	e.t.Helper()
	return fixture.Engagement(e.t, e.db, e.Practice, opts...)
}

// contractURL is the Staff route of the Engagement's Contract plus path.
func (e *env) contractURL(path string) string {
	return e.contractURLOf(e.Engagement.ID, path)
}

// contractURLOf is the Staff route of engagementID's Contract plus path.
func (e *env) contractURLOf(engagementID, path string) string {
	return e.srv.URL + "/api/practices/" + e.Practice.ID + "/engagements/" + engagementID + "/contract" + path
}

// portalURL is the portal route of the Engagement's Contract plus path.
func (e *env) portalURL(path string) string {
	return e.portalURLOf(e.Engagement.ID, path)
}

// portalURLOf is the portal route of engagementID's Contract plus path.
func (e *env) portalURLOf(engagementID, path string) string {
	return e.srv.URL + "/api/portal/engagements/" + engagementID + "/contract" + path
}

// templateURL is the Practice's Contract Template route.
func (e *env) templateURL() string {
	return e.srv.URL + "/api/practices/" + e.Practice.ID + "/contract-template"
}

// putting is the body of a PUT that sets a Draft's merge field values.
func putting(values contracts.MergeFieldValues) reqtest.Option {
	return reqtest.JSON(contracts.PutContractRequest{Values: values})
}

// contract reads the Contract as the Staff member at the screen.
func (e *env) contract() contracts.ContractResponse {
	e.t.Helper()
	return reqtest.Expect[contracts.ContractResponse](e.t, http.MethodGet, e.contractURL(""), http.StatusOK, e.signedIn)
}

// exec runs one fixture statement on the superuser connection.
func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Admin.ExecContext(e.t.Context(), query, args...); err != nil {
		e.t.Fatalf("fixture %q: %v", query, err)
	}
}

// seedTemplate gives the Practice a Contract Template with this prose.
func (e *env) seedTemplate(prose string) {
	e.t.Helper()
	seedContractTemplate(e.t, e.db, e.Practice.ID, prose)
}

// readyToDraft gives the Practice a Contract Template with this prose and
// a birth rate, the two things POST needs before it can create a Contract.
func (e *env) readyToDraft(prose string) {
	e.t.Helper()
	e.seedTemplate(prose)
	testdb.SeedPracticeRate(e.t, e.db, e.Practice.ID, "birth", testRateAmountCents)
}

// seedContract seeds the Engagement's Contract at status, with the
// package's usual merge-field prose.
func (e *env) seedContract(status string) {
	e.t.Helper()
	seedContract(e.t, e.db, e.Engagement.ID, status, mergeFieldProse)
}

// seedDraft seeds the Engagement's Contract as a draft.
func (e *env) seedDraft() {
	e.t.Helper()
	e.seedContract(statusDraft)
}

// refusesEngagementsItCannotReach proves a route refuses an id that is not
// a uuid with 400 and an Engagement of another Practice with 404. path is
// what follows "/contract", and opts carry the body the route reads.
func (e *env) refusesEngagementsItCannotReach(method, path string, opts ...reqtest.Option) {
	e.t.Helper()
	other := testdb.SeedPractice(e.t, e.db, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(e.t, e.db, other)
	cases := []struct {
		name         string
		engagementID string
		want         int
	}{
		{"id is not a uuid", "not-a-uuid", http.StatusBadRequest},
		{"engagement of another Practice", otherEngagementID, http.StatusNotFound},
	}
	for _, c := range cases {
		e.t.Run(c.name, func(t *testing.T) {
			reqtest.ExpectStatus(t, method, e.contractURLOf(c.engagementID, path), c.want, append([]reqtest.Option{e.signedIn}, opts...)...)
		})
	}
}
