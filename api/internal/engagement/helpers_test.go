package engagement_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// env is what a test in this package starts from: a Practice with this
// package's whole surface mounted on a server, one Staff member at the
// screen and one Engagement. With no option she is an employee Doula and
// the Engagement is an intake birth for a new Client; a test says only
// what it changes, with signedInAs and withEngagement.
type env struct {
	*fixture.Scene
}

type envOption = fixture.SceneOption

// The option names this package's tests read as; each is the Scene's own.
var (
	signedInAs     = fixture.SignedInAs
	withEngagement = fixture.WithEngagement
)

// newEnv mounts this package's whole surface through engagement.Mount,
// the same call main.go makes on the real GatedRouter and
// idempotency.Router.
func newEnv(t *testing.T, opts ...fixture.SceneOption) *env {
	t.Helper()
	return &env{fixture.NewScene(t, fixture.Routes(engagement.Mount), append([]fixture.SceneOption{withEngagement()}, opts...)...)}
}

// urlOf is the route of engagementID plus path.
func (e *env) urlOf(engagementID, path string) string {
	return e.PracticeURL("/engagements/" + engagementID + path)
}

// engagementURL is the Engagement's route plus path.
func (e *env) engagementURL(path string) string {
	return e.urlOf(e.Engagement.ID, path)
}

// detail reads the Engagement as the Staff member at the screen.
func (e *env) detail() engagement.Detail {
	e.T.Helper()
	return e.detailAs(e.Staff)
}

// detailAs reads the Engagement as who.
func (e *env) detailAs(who fixture.StaffRecord) engagement.Detail {
	e.T.Helper()
	return e.detailOf(e.Engagement.ID, who)
}

// detailOf reads engagementID as who.
func (e *env) detailOf(engagementID string, who fixture.StaffRecord) engagement.Detail {
	e.T.Helper()
	return reqtest.Expect[engagement.Detail](e.T, http.MethodGet, e.urlOf(engagementID, ""), http.StatusOK, e.As(who))
}

// eachFactWriter runs run once per row of ADR-0015's role table for
// writing an Engagement's facts (refuseFactWrite): an Owner, an Admin and
// an employee Doula may; a contractor Doula and a member with no role are
// refused. Each row gets its own env, signed in as that role.
func eachFactWriter(t *testing.T, run func(t *testing.T, e *env, wantOK bool), opts ...envOption) {
	t.Helper()
	for _, tc := range []struct {
		role   fixture.RoleCase
		wantOK bool
	}{
		{fixture.OwnerCase, true},
		{fixture.AdminCase, true},
		{fixture.EmployeeDoulaCase, true},
		{fixture.ContractorDoulaCase, false},
		{fixture.NoRoleCase, false},
	} {
		t.Run(tc.role.Name, func(t *testing.T) {
			run(t, newEnv(t, append([]envOption{signedInAs(tc.role.Opts...)}, opts...)...), tc.wantOK)
		})
	}
}

// okOrForbidden is the status a write answers a role that may do it, 200,
// and one that may not, 403.
func okOrForbidden(allowed bool) int {
	if allowed {
		return http.StatusOK
	}
	return http.StatusForbidden
}

// refusesAnUnreadableRequest is the three refusals every write on an
// Engagement gives before it weighs the values it was sent: an id that is
// not a uuid, an id nobody has, and a body that does not decode. valid is
// a body the endpoint would accept.
func (e *env) refusesAnUnreadableRequest(t *testing.T, method, path string, valid reqtest.Option) {
	t.Helper()
	cases := []struct {
		name         string
		engagementID string
		body         reqtest.Option
		want         int
	}{
		{"an id that is not a uuid", "not-a-uuid", valid, http.StatusBadRequest},
		{"an engagement that does not exist", "00000000-0000-0000-0000-000000000000", valid, http.StatusNotFound},
		{"a body that is not JSON", e.Engagement.ID, reqtest.RawJSON([]byte("{")), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqtest.ExpectStatus(t, method, e.urlOf(tc.engagementID, path), tc.want, e.SignedIn, tc.body)
		})
	}
}

// employeeType is named once for the same reason, shared by every test
// that seeds a Staff member's employment_type.

// careCompleteReason is named once for the same reason, shared by every
// transition test that completes an Engagement without caring which of
// the six legal ending reasons it carries.
const careCompleteReason = "care_complete"

// contractorType is named once for the same reason, shared by every test
// that seeds ADR-0008's other employment type.

// employeeDoulaKind is the role-table case label an employee Doula's row
// carries, named once for the same reason -- two role tables in this
// package name her, and one of them branches on the label.
const employeeDoulaKind = "employee doula"
