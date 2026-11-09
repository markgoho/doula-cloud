package plans_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	carePlanType  = "care_plan"
	birthPlanType = "birth_plan"
	shortTextType = "short_text"
	// testFieldLabel is a placeholder Label used across many test cases that
	// don't care about its value, just that it's non-empty.
	testFieldLabel = "Name"
	// hospital is the one planned-birth-location answer these tests seed and
	// read back, named once so the files that assert on it agree.
	hospital = "Hospital"

	// ownerRole, adminRole and doulaRole are named once so golangci-lint's
	// goconst check doesn't see repeated literals across this package's tests.
	ownerRole = "owner"
	adminRole = "admin"
	doulaRole = "doula"

	nameFieldID = "name"
	jamieAnswer = "Jamie"
)

// Field lists and answers the tests seed, named once so goconst does not
// see independent literals across files.
const (
	f1Fields           = `[{"id":"f1","type":"short_text","label":"Name","order":0}]`
	nameFields         = `[{"id":"name","type":"short_text","label":"Name","order":0}]`
	locationFields     = `[{"id":"location","type":"single_select","label":"Planned birth location","options":["Home","Hospital"],"order":0}]`
	locationNoOptions  = `[{"id":"location","type":"single_select","label":"Planned birth location","order":0}]`
	locationInHospital = `{"location":"Hospital"}`
)

// seedPlanTemplate seeds a Plan Template row directly (bypassing the
// handlers under test). Stays local under this name rather than testdb:
// contracts/template_test.go's own seedTemplate writes contract_templates
// (prose), and clientfieldtemplate's writes client_field_templates -- three
// different tables sharing one generic old name.
func seedPlanTemplate(t *testing.T, db *testdb.DB, practiceID, planType, fieldsJSON string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO plan_templates (practice_id, plan_type, fields) VALUES ($1, $2, $3)`,
		practiceID, planType, fieldsJSON,
	); err != nil {
		t.Fatalf("seed template: %v", err)
	}
}

// seedInstance seeds a Plan Instance row directly (bypassing the handlers
// under test).
func seedInstance(t *testing.T, db *testdb.DB, engagementID, planType, fieldsJSON, answersJSON string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO plan_instances (engagement_id, plan_type, fields, answers, changed_at) VALUES ($1, $2, $3, $4, now())`,
		engagementID, planType, fieldsJSON, answersJSON,
	); err != nil {
		t.Fatalf("seed instance: %v", err)
	}
}

// env is what a test in this package starts from: a Practice with this
// package's whole surface mounted on a server, one Staff member at the
// screen and one Engagement. With no option she is an employee Doula and
// the Engagement is an intake birth for a new Client; a test says only
// what it changes, with signedInAs, withEngagement, withClient and
// atInstant. The same server answers Staff and the Client's portal, so
// asClient signs a Client in when she was seeded with a portal login.
type env struct {
	t   *testing.T
	db  *testdb.DB
	srv *httptest.Server

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
	client     []fixture.ClientOption
	at         time.Time
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

// attachedToEngagement grants the Staff member at the screen an open
// Attachment on the Engagement.
func attachedToEngagement() envOption {
	return func(s *envSpec) { s.attached = true }
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	var spec envSpec
	for _, opt := range opts {
		opt(&spec)
	}
	db := testdb.New(t)
	e := &env{t: t, db: db, Practice: fixture.Practice(t, db, spec.practice...)}
	e.Staff = fixture.Staff(t, db, e.Practice, spec.staff...)
	engagementOpts := spec.engagement
	if len(spec.client) > 0 {
		engagementOpts = append(engagementOpts, fixture.NewClient(spec.client...))
	}
	e.Engagement = fixture.Engagement(t, db, e.Practice, engagementOpts...)
	if spec.attached {
		testdb.SeedGrantedAttachment(t, db, e.Engagement.ID, e.Staff.ID)
	}
	e.srv = e.mount(spec.at)
	e.signedIn = e.as(e.Staff)
	if e.Engagement.Client.PortalUID != "" {
		e.asClient = e.as(e.Engagement.Client)
	}
	return e
}

// mount mounts this package's whole surface through plans.Mount, the same
// call main.go makes on the real GatedRouter and idempotency.Router.
func (e *env) mount(at time.Time) *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, e.db.App)
	ir := idempotency.NewRouter(g, e.db.App)
	plans.Mount(g, ir, e.db.App)
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

// anotherEngagement is a second Engagement at the Practice.
func (e *env) anotherEngagement() fixture.EngagementRecord {
	e.t.Helper()
	return fixture.Engagement(e.t, e.db, e.Practice)
}

// templateURL is the Practice's Plan Template route for planType.
func (e *env) templateURL(planType string) string {
	return e.srv.URL + "/api/practices/" + e.Practice.ID + "/plan-templates/" + planType
}

// instanceURL is the Staff route of the Engagement's planType Plan plus path.
func (e *env) instanceURL(planType, path string) string {
	return e.instanceURLOf(e.Engagement.ID, planType, path)
}

// instanceURLOf is the Staff route of engagementID's planType Plan plus path.
func (e *env) instanceURLOf(engagementID, planType, path string) string {
	return e.srv.URL + "/api/practices/" + e.Practice.ID + "/engagements/" + engagementID + "/plans/" + planType + path
}

// portalURL is the portal route of the Engagement's Birth Plan plus path.
func (e *env) portalURL(path string) string {
	return e.portalURLOf(e.Engagement.ID, path)
}

// portalURLOf is the portal route of engagementID's Birth Plan plus path.
func (e *env) portalURLOf(engagementID, path string) string {
	return e.srv.URL + "/api/portal/engagements/" + engagementID + "/birth-plan" + path
}

// exec runs one fixture statement on the superuser connection.
func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Admin.ExecContext(e.t.Context(), query, args...); err != nil {
		e.t.Fatalf("fixture %q: %v", query, err)
	}
}

// seedTemplate gives the Practice a planType template with these fields.
func (e *env) seedTemplate(planType, fieldsJSON string) {
	e.t.Helper()
	seedPlanTemplate(e.t, e.db, e.Practice.ID, planType, fieldsJSON)
}

// seedInstance starts the Engagement's planType Plan with these fields and
// answers.
func (e *env) seedInstance(planType, fieldsJSON, answersJSON string) {
	e.t.Helper()
	seedInstance(e.t, e.db, e.Engagement.ID, planType, fieldsJSON, answersJSON)
}

// seedBirthPlan starts the Engagement's Birth Plan with the planned
// location answered at the hospital.
func (e *env) seedBirthPlan() {
	e.t.Helper()
	e.seedInstance(birthPlanType, locationFields, locationInHospital)
}

// setAcknowledged directly sets client_acknowledged_at on the Engagement's
// planType Plan, bypassing ClientAcknowledgeBirthPlanHandler -- used to
// set up the "already acknowledged" state a test needs before exercising
// some other behavior (e.g. PutInstanceHandler's reset-on-edit).
func (e *env) setAcknowledged(planType string) {
	e.t.Helper()
	e.exec(`UPDATE plan_instances SET client_acknowledged_at = now() WHERE engagement_id = $1 AND plan_type = $2`, e.Engagement.ID, planType)
}

// instance reads the Engagement's planType Plan as the Staff member at the
// screen.
func (e *env) instance(planType string) plans.InstanceResponse {
	e.t.Helper()
	return reqtest.Expect[plans.InstanceResponse](e.t, http.MethodGet, e.instanceURL(planType, ""), http.StatusOK, e.signedIn)
}

// putAnswers saves answers to the Engagement's planType Plan.
func (e *env) putAnswers(planType string, answers plans.Answers) plans.InstanceResponse {
	e.t.Helper()
	return reqtest.Expect[plans.InstanceResponse](e.t, http.MethodPut, e.instanceURL(planType, ""), http.StatusOK, e.signedIn,
		reqtest.JSON(plans.PutInstanceRequest{Answers: answers}))
}

// start POSTs the Engagement's planType Plan and requires want.
func (e *env) start(planType string, want int) plans.InstanceResponse {
	e.t.Helper()
	return reqtest.Expect[plans.InstanceResponse](e.t, http.MethodPost, e.instanceURL(planType, ""), want, e.signedIn)
}

// putTemplate sends a PUT of fields to the Practice's planType template.
func (e *env) putTemplate(planType string, fields []plans.Field, want int) plans.TemplateResponse {
	e.t.Helper()
	return reqtest.Expect[plans.TemplateResponse](e.t, http.MethodPut, e.templateURL(planType), want, e.signedIn,
		reqtest.JSON(plans.TemplateResponse{Fields: fields}))
}

// template reads the Practice's planType template.
func (e *env) template(planType string) plans.TemplateResponse {
	e.t.Helper()
	return reqtest.Expect[plans.TemplateResponse](e.t, http.MethodGet, e.templateURL(planType), http.StatusOK, e.signedIn)
}

// birthPlan reads the Birth Plan as the Client at the screen.
func (e *env) birthPlan() plans.InstanceResponse {
	e.t.Helper()
	return reqtest.Expect[plans.InstanceResponse](e.t, http.MethodGet, e.portalURL(""), http.StatusOK, e.asClient)
}

// activityActions lists the actions recorded against the Engagement,
// oldest first.
func (e *env) activityActions() []string {
	e.t.Helper()
	return testdb.Column[string](e.t, e.db.Admin,
		`SELECT action FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 ORDER BY created_at, id`, e.Engagement.ID)
}

// recordOutcome puts a birth outcome on the Engagement the way Staff's own
// endpoint would, without going through it -- this package tests the
// portal's reading of the fact, not #293's writing of it. It opens
// 00093's correction door for the transaction, because a second call
// against the same Engagement is exactly the correction ADR-0015's
// motivating typo needs and the freeze trigger would otherwise refuse it.
func (e *env) recordOutcome(outcome string, endedOn *string) {
	e.t.Helper()
	tx, err := e.db.Admin.BeginTx(e.t.Context(), nil)
	if err != nil {
		e.t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(e.t.Context(),
		`SELECT set_config('app.allow_outcome_correction', 'on', true)`); err != nil {
		e.t.Fatalf("open the correction door: %v", err)
	}
	if _, err := tx.ExecContext(e.t.Context(),
		`UPDATE engagements SET birth_outcome = $2::birth_outcome, pregnancy_ended_on = $3::date WHERE id = $1`,
		e.Engagement.ID, outcome, endedOn); err != nil {
		e.t.Fatalf("record the birth outcome %q: %v", outcome, err)
	}
	if err := tx.Commit(); err != nil {
		e.t.Fatalf("commit the birth outcome %q: %v", outcome, err)
	}
}

// refusesEngagementsItCannotReach proves a Staff route refuses an id that
// is not a uuid with 400 and an Engagement of another Practice with 404.
// path is what follows the plan type, and opts carry the body the route
// reads.
func (e *env) refusesEngagementsItCannotReach(method, planType, path string, opts ...reqtest.Option) {
	e.t.Helper()
	fixture.RefusesEngagementsItCannotReach(e.t, e.db, method, func(engagementID string) string {
		return e.instanceURLOf(engagementID, planType, path)
	}, append([]reqtest.Option{e.signedIn}, opts...)...)
}

// roleCase is one Staff member who may read a Plan Template, for a test
// that ranges over who may do a thing.
type roleCase struct {
	name string
	opts []fixture.StaffOption
}

var (
	ownerCase           = roleCase{ownerRole, []fixture.StaffOption{fixture.Owner()}}
	adminCase           = roleCase{adminRole, []fixture.StaffOption{fixture.Admin()}}
	employeeDoulaCase   = roleCase{doulaRole, []fixture.StaffOption{fixture.Doula()}}
	contractorDoulaCase = roleCase{"contractor doula", []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}}
)
