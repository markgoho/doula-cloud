package contracts_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// Names of table rows that more than one table repeats, named once so
// goconst does not see independent literals across this package's tests.
const (
	caseAdmin       = "admin"
	caseBodyNotJSON = "body is not JSON"
	caseNoContract  = "no contract"
)

// env is what a test in this package starts from: a Practice with this
// package's whole surface mounted on a server, one Staff member at the
// screen and one Engagement. With no option she is an employee Doula and
// the Engagement is an intake birth for a new Client; a test says only
// what it changes, with signedInAs, withEngagement, withClient and
// withStore. The same server answers Staff and the Client's portal, so
// AsClient signs a Client in when she was seeded with a portal login.
type env struct {
	*fixture.Scene

	store  objectstore.ObjectStore
	pusher *push.FakePusher
}

type envSpec struct {
	store objectstore.ObjectStore
	scene []fixture.SceneOption
}

type envOption func(*envSpec)

func viaScene(opts ...fixture.SceneOption) envOption {
	return func(s *envSpec) { s.scene = append(s.scene, opts...) }
}

// The option names this package's tests read as; each is the Scene's own.
func signedInAs(opts ...fixture.StaffOption) envOption {
	return viaScene(fixture.SignedInAs(opts...))
}

func withClient(opts ...fixture.ClientOption) envOption {
	return viaScene(fixture.WithClient(opts...))
}

func withPortalLogin() envOption { return withClient(fixture.WithPortalLogin()) }

func withPractice(opts ...fixture.PracticeOption) envOption {
	return viaScene(fixture.InPractice(opts...))
}

func atInstant(at time.Time) envOption { return viaScene(fixture.At(at)) }

func attachedToEngagement() envOption { return viaScene(fixture.Attached()) }

// alsoMounting mounts another package's routes beside this one's, for a
// test whose subject is how the two agree.
func alsoMounting(mount func(*staffauth.GatedRouter, *idempotency.Router)) envOption {
	return viaScene(fixture.AlsoMounting(fixture.Routes(mount)))
}

// withStore swaps the object store the Signed PDF is written to and read from.
func withStore(store objectstore.ObjectStore) envOption {
	return func(s *envSpec) { s.store = store }
}

// newEnv mounts this package's whole surface through contracts.Mount, the
// same call main.go makes on the real GatedRouter and idempotency.Router.
func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	spec := envSpec{store: objectstore.NewMemoryStore(), scene: []fixture.SceneOption{fixture.WithEngagement()}}
	for _, opt := range opts {
		opt(&spec)
	}
	e := &env{store: spec.store, pusher: push.NewFakePusher()}
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, app *sql.DB) {
		contracts.Mount(g, ir, app, e.store, e.pusher)
	}
	e.Scene = fixture.NewScene(t, mount, spec.scene...)
	return e
}

// contractURL is the Staff route of the Engagement's Contract plus path.
func (e *env) contractURL(path string) string {
	return e.contractURLOf(e.Engagement.ID, path)
}

// contractURLOf is the Staff route of engagementID's Contract plus path.
func (e *env) contractURLOf(engagementID, path string) string {
	return e.PracticeURL("/engagements/" + engagementID + "/contract" + path)
}

// portalURL is the portal route of the Engagement's Contract plus path.
func (e *env) portalURL(path string) string {
	return e.portalURLOf(e.Engagement.ID, path)
}

// portalURLOf is the portal route of engagementID's Contract plus path.
func (e *env) portalURLOf(engagementID, path string) string {
	return e.URL("/api/portal/engagements/" + engagementID + "/contract" + path)
}

// templateURL is the Practice's Contract Template route.
func (e *env) templateURL() string {
	return e.PracticeURL("/contract-template")
}

// putting is the body of a PUT that sets a Draft's merge field values.
func putting(values contracts.MergeFieldValues) reqtest.Option {
	return reqtest.JSON(contracts.PutContractRequest{Values: values})
}

// contract reads the Contract as the Staff member at the screen.
func (e *env) contract() contracts.ContractResponse {
	e.T.Helper()
	return reqtest.Expect[contracts.ContractResponse](e.T, http.MethodGet, e.contractURL(""), http.StatusOK, e.SignedIn)
}

// seedTemplate gives the Practice a Contract Template with this prose.
func (e *env) seedTemplate(prose string) {
	e.T.Helper()
	seedContractTemplate(e.T, e.DB, e.Practice.ID, prose)
}

// readyToDraft gives the Practice a Contract Template with this prose and
// a birth rate, the two things POST needs before it can create a Contract.
func (e *env) readyToDraft(prose string) {
	e.T.Helper()
	e.seedTemplate(prose)
	testdb.SeedPracticeRate(e.T, e.DB, e.Practice.ID, "birth", testRateAmountCents)
}

// seedContract seeds the Engagement's Contract at status, with the
// package's usual merge-field prose.
func (e *env) seedContract(status string) {
	e.T.Helper()
	seedContract(e.T, e.DB, e.Engagement.ID, status, mergeFieldProse)
}

// seedDraft seeds the Engagement's Contract as a draft.
func (e *env) seedDraft() {
	e.T.Helper()
	e.seedContract(statusDraft)
}

// refusesEngagementsItCannotReach proves a route refuses an id that is not
// a uuid with 400 and an Engagement of another Practice with 404. path is
// what follows "/contract", and opts carry the body the route reads.
func (e *env) refusesEngagementsItCannotReach(method, path string, opts ...reqtest.Option) {
	e.T.Helper()
	fixture.RefusesEngagementsItCannotReach(e.T, e.DB, method, func(engagementID string) string {
		return e.contractURLOf(engagementID, path)
	}, append([]reqtest.Option{e.SignedIn}, opts...)...)
}
