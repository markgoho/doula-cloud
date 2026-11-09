package staffauth_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/reqtest"
)

// selfRoutes are the writes a Staff member makes about herself before, or
// without, naming a Practice: no {practiceId} and no {staffId} in the
// path, which is the whole authorization story.
var selfRoutes = []struct {
	name   string
	method string
	path   string
	valid  string
}{
	{"work state", http.MethodPut, "/api/staff/work-state", `{"workState":"NJ"}`},
	{"name", http.MethodPut, "/api/staff/name", `{"firstName":"A","lastName":"B"}`},
	{"feedback", http.MethodPost, "/api/staff/feedback", `{"kind":"idea_or_request"}`},
}

// TestRLS_SelfEventsRefuseAForeignActor is the audit tables' own boundary:
// a row claiming someone else made the change is refused by the table's
// WITH CHECK (staff_work_state_events_self, staff_name_events_self), so
// actor_staff_id stays a signal rather than a field anyone may fill in.
func TestRLS_SelfEventsRefuseAForeignActor(t *testing.T) {
	cases := []struct {
		name   string
		insert string
	}{
		{"work state", `INSERT INTO staff_work_state_events (staff_id, previous_work_state, work_state, actor_staff_id) VALUES ($1, 'NY', 'TX', $2)`},
		{"name", `INSERT INTO staff_name_events (staff_id, kind, actor_staff_id) VALUES ($1, 'changed', $2)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			foreign := e.Colleague()

			setIdentity(t, e.DB, e.Staff.UID)
			if _, err := e.DB.App.ExecContext(t.Context(), tc.insert, e.Staff.ID, foreign.ID); err == nil {
				t.Fatal("insert succeeded -- an event may not name an actor who is not the caller")
			}
		})
	}
}

// TestSelfRoutes_RefuseTheUnreadableRequest covers the three ways a
// request never reaches what the route is about: no cookie, a live
// session whose identity resolves to no staff row (the shape a person who
// abandoned an invitation halfway arrives in), and a body that is not
// JSON.
func TestSelfRoutes_RefuseTheUnreadableRequest(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
		// session returns the credential the request carries.
		session func(e *env) reqtest.Option
	}{
		{"no cookie", "", http.StatusUnauthorized, func(*env) reqtest.Option { return reqtest.Session("") }},
		{"a session with no staff row", "", http.StatusNotFound, func(e *env) reqtest.Option {
			return reqtest.Session(authntest.SeedSession(e.T, e.DB.App, "session-without-a-staff-row"))
		}},
		{"a body that is not JSON", "not json", http.StatusBadRequest, func(e *env) reqtest.Option { return e.SignedIn }},
	}
	for _, route := range selfRoutes {
		for _, tc := range cases {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				e := newEnv(t)
				body := tc.body
				if body == "" {
					body = route.valid
				}

				reqtest.ExpectStatus(t, route.method, e.URL(route.path), tc.want, tc.session(e), reqtest.RawJSON([]byte(body)))
			})
		}
	}
}
