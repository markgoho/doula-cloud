package engagement_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// env is what a test in this package starts from: a Practice with this
// package's whole surface mounted on a server, one Staff member at the
// screen and one Engagement. With no option she is an employee Doula and
// the Engagement is an intake birth for a new Client; a test says only
// what it changes, with signedInAs and withEngagement.
type env struct {
	t   *testing.T
	db  *testdb.DB
	srv *httptest.Server

	Practice   fixture.PracticeRecord
	Staff      fixture.StaffRecord
	Engagement fixture.EngagementRecord

	signedIn reqtest.Option
}

type envSpec struct {
	staff      []fixture.StaffOption
	engagement []fixture.EngagementOption
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

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	var spec envSpec
	for _, opt := range opts {
		opt(&spec)
	}
	db := testdb.New(t)
	e := &env{t: t, db: db, Practice: fixture.Practice(t, db)}
	e.Staff = fixture.Staff(t, db, e.Practice, spec.staff...)
	e.Engagement = fixture.Engagement(t, db, e.Practice, spec.engagement...)
	e.srv = e.mount()
	e.signedIn = e.as(e.Staff)
	return e
}

// mount mounts this package's whole surface through engagement.Mount,
// the same call main.go makes on the real GatedRouter and
// idempotency.Router.
func (e *env) mount() *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, e.db.App)
	ir := idempotency.NewRouter(g, e.db.App)
	engagement.Mount(g, ir)
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

// anotherEngagement is a second Engagement at the Practice.
func (e *env) anotherEngagement(opts ...fixture.EngagementOption) fixture.EngagementRecord {
	e.t.Helper()
	return fixture.Engagement(e.t, e.db, e.Practice, opts...)
}

// urlOf is the route of engagementID plus path.
func (e *env) urlOf(engagementID, path string) string {
	return e.srv.URL + "/api/practices/" + e.Practice.ID + "/engagements/" + engagementID + path
}

// engagementURL is the Engagement's route plus path.
func (e *env) engagementURL(path string) string {
	return e.urlOf(e.Engagement.ID, path)
}

// detail reads the Engagement as the Staff member at the screen.
func (e *env) detail() engagement.Detail {
	e.t.Helper()
	return e.detailAs(e.Staff)
}

// detailAs reads the Engagement as who.
func (e *env) detailAs(who fixture.StaffRecord) engagement.Detail {
	e.t.Helper()
	return e.detailOf(e.Engagement.ID, who)
}

// detailOf reads engagementID as who.
func (e *env) detailOf(engagementID string, who fixture.StaffRecord) engagement.Detail {
	e.t.Helper()
	return reqtest.Expect[engagement.Detail](e.t, http.MethodGet, e.urlOf(engagementID, ""), http.StatusOK, e.as(who))
}

// exec runs one fixture statement on the superuser connection.
func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Admin.ExecContext(e.t.Context(), query, args...); err != nil {
		e.t.Fatalf("fixture %q: %v", query, err)
	}
}

// roleCase is one row of ADR-0015's role table, for a test that ranges
// over who may do a thing.
type roleCase struct {
	name string
	opts []fixture.StaffOption
}

var (
	ownerCase           = roleCase{ownerRole, []fixture.StaffOption{fixture.Owner()}}
	adminCase           = roleCase{adminRole, []fixture.StaffOption{fixture.Admin()}}
	employeeDoulaCase   = roleCase{employeeDoulaKind, []fixture.StaffOption{fixture.Doula()}}
	contractorDoulaCase = roleCase{contractorDoulaKind, []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}}
	noRoleCase          = roleCase{"no role", []fixture.StaffOption{fixture.NoRole()}}
)

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
			reqtest.ExpectStatus(t, method, e.urlOf(tc.engagementID, path), tc.want, e.signedIn, tc.body)
		})
	}
}

// ownerRole is named once so golangci-lint's goconst check doesn't see
// repeated "owner" literals across this package's test surface.
const ownerRole = "owner"

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

// contractorDoulaKind is the role-table case label a contractor Doula's
// row carries, named once for the same reason -- three role tables in
// this package name her (transition_test.go, outcome_test.go,
// kind_change_test.go).
const contractorDoulaKind = "contractor doula"
