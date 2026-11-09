package clientfieldtemplate_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/clientfieldtemplate"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const (
	testFieldLabel   = "Some Field"
	shortTextType    = "short_text"
	firstFieldLabel  = "First"
	secondFieldLabel = "Second"
	// intakeNoteField is a one-field template as the database stores it.
	intakeNoteField = `[{"id":"f1","type":"short_text","label":"Intake note","order":0,"archived":false}]`
)

// env is what a test in this package starts from: a Practice with the
// Client Field Template routes mounted and one Staff member at the
// screen, an employee Doula unless a test says otherwise.
type env struct {
	*fixture.Scene
}

// newEnv mounts this package's routes through clientfieldtemplate.Mount,
// the same call main.go makes on the real GatedRouter and
// idempotency.Router.
func newEnv(t *testing.T, staff ...fixture.StaffOption) *env {
	t.Helper()
	return &env{fixture.NewScene(t, fixture.Routes(clientfieldtemplate.Mount), fixture.SignedInAs(staff...))}
}

func (e *env) url() string { return e.PracticeURL("/client-field-template") }

// seedTemplate seeds a client_field_templates row directly (bypassing the
// handlers under test).
func (e *env) seedTemplate(fieldsJSON string) {
	e.T.Helper()
	e.Exec(`INSERT INTO client_field_templates (practice_id, fields) VALUES ($1, $2)`, e.Practice.ID, fieldsJSON)
}

// template reads the template as the Staff member at the screen.
func (e *env) template() clientfieldtemplate.TemplateResponse {
	e.T.Helper()
	return reqtest.Expect[clientfieldtemplate.TemplateResponse](e.T, http.MethodGet, e.url(), http.StatusOK, e.SignedIn)
}

// putBody sends body as a PUT and requires want.
func (e *env) putBody(body reqtest.Option, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodPut, e.url(), want, e.SignedIn, body)
}

// putFields puts fields and requires want.
func (e *env) putFields(want int, fields ...clientfieldtemplate.Field) {
	e.T.Helper()
	e.putBody(reqtest.JSON(clientfieldtemplate.TemplateResponse{Fields: fields}), want)
}

// putFieldsOK puts fields, requires 200 and returns the template it saved.
func (e *env) putFieldsOK(fields ...clientfieldtemplate.Field) clientfieldtemplate.TemplateResponse {
	e.T.Helper()
	return reqtest.Expect[clientfieldtemplate.TemplateResponse](e.T, http.MethodPut, e.url(), http.StatusOK, e.SignedIn,
		reqtest.JSON(clientfieldtemplate.TemplateResponse{Fields: fields}))
}

// shortText is a short-text field with a label.
func shortText(id, label string) clientfieldtemplate.Field {
	return clientfieldtemplate.Field{ID: id, Type: shortTextType, Label: label}
}

const templateAuditRows = `FROM activity WHERE practice_id = $1 AND subject_kind = 'client_field_template'`

// auditEventCount counts activity rows (subject_kind
// 'client_field_template', ADR-0022) for the Practice.
func (e *env) auditEventCount() int {
	e.T.Helper()
	return testdb.Scalar[int](e.T, e.DB.Admin, `SELECT count(*) `+templateAuditRows, e.Practice.ID)
}

// lastAuditActor is the actor_staff_id of the most recent activity row
// (subject_kind 'client_field_template') for the Practice.
func (e *env) lastAuditActor() string {
	e.T.Helper()
	return testdb.Scalar[string](e.T, e.DB.Admin, `SELECT actor_staff_id `+templateAuditRows+` ORDER BY created_at DESC LIMIT 1`, e.Practice.ID)
}
