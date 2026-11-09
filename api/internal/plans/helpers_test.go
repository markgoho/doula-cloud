package plans_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/reqtest"
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
	*fixture.Scene
}

// The option names this package's tests read as; each is the Scene's own.
var (
	signedInAs           = fixture.SignedInAs
	withEngagement       = fixture.WithEngagement
	withClient           = fixture.WithClient
	withPractice         = fixture.InPractice
	atInstant            = fixture.At
	attachedToEngagement = fixture.Attached
)

// withPortalLogin lets the Engagement's Client sign in to the portal.
func withPortalLogin() fixture.SceneOption {
	return withClient(fixture.WithPortalLogin())
}

type envOption = fixture.SceneOption

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	return &env{fixture.NewScene(t, plans.Mount, append([]envOption{withEngagement()}, opts...)...)}
}

// templateURL is the Practice's Plan Template route for planType.
func (e *env) templateURL(planType string) string {
	return e.PracticeURL("/plan-templates/" + planType)
}

// instanceURL is the Staff route of the Engagement's planType Plan plus path.
func (e *env) instanceURL(planType, path string) string {
	return e.instanceURLOf(e.Engagement.ID, planType, path)
}

// instanceURLOf is the Staff route of engagementID's planType Plan plus path.
func (e *env) instanceURLOf(engagementID, planType, path string) string {
	return e.PracticeURL("/engagements/" + engagementID + "/plans/" + planType + path)
}

// portalURL is the portal route of the Engagement's Birth Plan plus path.
func (e *env) portalURL(path string) string {
	return e.portalURLOf(e.Engagement.ID, path)
}

// portalURLOf is the portal route of engagementID's Birth Plan plus path.
func (e *env) portalURLOf(engagementID, path string) string {
	return e.URL("/api/portal/engagements/" + engagementID + "/birth-plan" + path)
}

// seedTemplate gives the Practice a planType template with these fields.
func (e *env) seedTemplate(planType, fieldsJSON string) {
	e.T.Helper()
	seedPlanTemplate(e.T, e.DB, e.Practice.ID, planType, fieldsJSON)
}

// seedInstance starts the Engagement's planType Plan with these fields and
// answers.
func (e *env) seedInstance(planType, fieldsJSON, answersJSON string) {
	e.T.Helper()
	seedInstance(e.T, e.DB, e.Engagement.ID, planType, fieldsJSON, answersJSON)
}

// seedBirthPlan starts the Engagement's Birth Plan with the planned
// location answered at the hospital.
func (e *env) seedBirthPlan() {
	e.T.Helper()
	e.seedInstance(birthPlanType, locationFields, locationInHospital)
}

// setAcknowledged directly sets client_acknowledged_at on the Engagement's
// planType Plan, bypassing ClientAcknowledgeBirthPlanHandler -- used to
// set up the "already acknowledged" state a test needs before exercising
// some other behavior (e.g. PutInstanceHandler's reset-on-edit).
func (e *env) setAcknowledged(planType string) {
	e.T.Helper()
	e.Exec(`UPDATE plan_instances SET client_acknowledged_at = now() WHERE engagement_id = $1 AND plan_type = $2`, e.Engagement.ID, planType)
}

// instance reads the Engagement's planType Plan as the Staff member at the
// screen.
func (e *env) instance(planType string) plans.InstanceResponse {
	e.T.Helper()
	return reqtest.Expect[plans.InstanceResponse](e.T, http.MethodGet, e.instanceURL(planType, ""), http.StatusOK, e.SignedIn)
}

// putAnswers saves answers to the Engagement's planType Plan.
func (e *env) putAnswers(planType string, answers plans.Answers) plans.InstanceResponse {
	e.T.Helper()
	return reqtest.Expect[plans.InstanceResponse](e.T, http.MethodPut, e.instanceURL(planType, ""), http.StatusOK, e.SignedIn,
		reqtest.JSON(plans.PutInstanceRequest{Answers: answers}))
}

// start POSTs the Engagement's planType Plan and requires want.
func (e *env) start(planType string, want int) plans.InstanceResponse {
	e.T.Helper()
	return reqtest.Expect[plans.InstanceResponse](e.T, http.MethodPost, e.instanceURL(planType, ""), want, e.SignedIn)
}

// putTemplate sends a PUT of fields to the Practice's planType template.
func (e *env) putTemplate(planType string, fields []plans.Field, want int) plans.TemplateResponse {
	e.T.Helper()
	return reqtest.Expect[plans.TemplateResponse](e.T, http.MethodPut, e.templateURL(planType), want, e.SignedIn,
		reqtest.JSON(plans.TemplateResponse{Fields: fields}))
}

// template reads the Practice's planType template.
func (e *env) template(planType string) plans.TemplateResponse {
	e.T.Helper()
	return reqtest.Expect[plans.TemplateResponse](e.T, http.MethodGet, e.templateURL(planType), http.StatusOK, e.SignedIn)
}

// birthPlan reads the Birth Plan as the Client at the screen.
func (e *env) birthPlan() plans.InstanceResponse {
	e.T.Helper()
	return reqtest.Expect[plans.InstanceResponse](e.T, http.MethodGet, e.portalURL(""), http.StatusOK, e.AsClient)
}

// activityActions lists the actions recorded against the Engagement,
// oldest first.
func (e *env) activityActions() []string {
	e.T.Helper()
	return testdb.Column[string](e.T, e.DB.Admin,
		`SELECT action FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 ORDER BY created_at, id`, e.Engagement.ID)
}

// recordOutcome puts a birth outcome on the Engagement the way Staff's own
// endpoint would, without going through it -- this package tests the
// portal's reading of the fact, not #293's writing of it. It opens
// 00093's correction door for the transaction, because a second call
// against the same Engagement is exactly the correction ADR-0015's
// motivating typo needs and the freeze trigger would otherwise refuse it.
func (e *env) recordOutcome(outcome string, endedOn *string) {
	e.T.Helper()
	e.DB.ExecCorrectingOutcome(e.T,
		`UPDATE engagements SET birth_outcome = $2::birth_outcome, pregnancy_ended_on = $3::date WHERE id = $1`,
		e.Engagement.ID, outcome, endedOn)
}

// refusesEngagementsItCannotReach proves a Staff route refuses an id that
// is not a uuid with 400 and an Engagement of another Practice with 404.
// path is what follows the plan type, and opts carry the body the route
// reads.
func (e *env) refusesEngagementsItCannotReach(method, planType, path string, opts ...reqtest.Option) {
	e.T.Helper()
	fixture.RefusesEngagementsItCannotReach(e.T, e.DB, method, func(engagementID string) string {
		return e.instanceURLOf(engagementID, planType, path)
	}, append([]reqtest.Option{e.SignedIn}, opts...)...)
}
