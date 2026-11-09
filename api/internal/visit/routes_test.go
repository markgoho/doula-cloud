package visit_test

import (
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/visit"
)

const (
	nobodyID      = "00000000-0000-0000-0000-000000000000"
	scheduledTime = "2027-04-01T09:00:00Z"
)

// visitRoute is one route under an Engagement's Visits, described by what
// its refusals have in common.
type visitRoute struct {
	name   string
	method string
	// path follows the Engagement's route; {visit} stands for a Visit id.
	path string
	// staff shapes the Staff member at the screen.
	staff []fixture.StaffOption
	// body is a body the route accepts from the Staff member at the screen,
	// nil for a route that reads none.
	body func(self fixture.StaffRecord) reqtest.Option
	// badValues are bodies that parse but are refused with 400.
	badValues map[string]reqtest.Option
	// attachedOnly marks a route a contractor reaches only through an
	// open Attachment: staffauth.AttachingWrite's own CanAccessEngagement
	// precheck, and the read gate, 404 her before the handler's body runs.
	attachedOnly bool
}

func visitBody(v any) reqtest.Option { return reqtest.JSON(v) }

var visitRoutes = []visitRoute{
	{
		name: "create", method: http.MethodPost, path: "/visits", attachedOnly: true,
		body: func(fixture.StaffRecord) reqtest.Option { return visitBody(visit.CreateRequest{}) },
	},
	{name: "list", method: http.MethodGet, path: "/visits", attachedOnly: true},
	{
		name: "reassign", method: http.MethodPatch, path: "/visits/{visit}", staff: []fixture.StaffOption{fixture.Owner(), fixture.Doula()},
		body: func(self fixture.StaffRecord) reqtest.Option {
			return visitBody(visit.ReassignRequest{StaffID: self.ID})
		},
		badValues: map[string]reqtest.Option{
			"a staff id that is not a uuid": visitBody(visit.ReassignRequest{StaffID: notAUUID}),
			"a staff id nobody has":         visitBody(visit.ReassignRequest{StaffID: nobodyID}),
		},
	},
	{
		name: "schedule", method: http.MethodPatch, path: "/visits/{visit}/schedule", attachedOnly: true,
		body: func(fixture.StaffRecord) reqtest.Option {
			return visitBody(visit.ScheduleRequest{ScheduledAt: new(scheduledTime)})
		},
		badValues: map[string]reqtest.Option{
			"a timestamp that is not one": visitBody(visit.ScheduleRequest{ScheduledAt: new("not-a-timestamp")}),
		},
	},
	{
		name: "notes", method: http.MethodPatch, path: "/visits/{visit}/notes", attachedOnly: true,
		body: func(fixture.StaffRecord) reqtest.Option {
			return visitBody(visit.NotesRequest{Notes: new("notes")})
		},
		badValues: map[string]reqtest.Option{
			"no notes": visitBody(visit.NotesRequest{Notes: nil}),
		},
	},
}

// routeURL is the route of engagementID, with visitID where the path names
// one.
func (e *env) routeURL(r visitRoute, engagementID, visitID string) string {
	return e.PracticeURL("/engagements/" + engagementID + strings.ReplaceAll(r.path, "{visit}", visitID))
}

// TestVisitRoutes_RefuseWhatTheyCannotReach holds each route to the same
// refusals before it weighs the values it was sent: an Engagement id that
// is not a uuid or belongs to another Practice, a Visit id that is not one
// or that nobody holds, a body that is not JSON or carries a value the
// route refuses, and a contractor with no Attachment on the Engagement.
func TestVisitRoutes_RefuseWhatTheyCannotReach(t *testing.T) {
	for _, r := range visitRoutes {
		t.Run(r.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(r.staff...))
			visitID := seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)
			var body []reqtest.Option
			if r.body != nil {
				body = append(body, r.body(e.Staff))
			}
			request := func(t *testing.T, url string, want int, session reqtest.Option, opts ...reqtest.Option) {
				t.Helper()
				reqtest.ExpectStatus(t, r.method, url, want, append([]reqtest.Option{session}, opts...)...)
			}

			fixture.RefusesEngagementsItCannotReach(t, e.DB, r.method, func(engagementID string) string {
				return e.routeURL(r, engagementID, visitID)
			}, append([]reqtest.Option{e.SignedIn}, body...)...)

			if strings.Contains(r.path, "{visit}") {
				t.Run("a visit id that is not a uuid", func(t *testing.T) {
					request(t, e.routeURL(r, e.Engagement.ID, notAUUID), http.StatusBadRequest, e.SignedIn, body...)
				})
				t.Run("a visit nobody has", func(t *testing.T) {
					request(t, e.routeURL(r, e.Engagement.ID, nobodyID), http.StatusNotFound, e.SignedIn, body...)
				})
			}
			if r.body != nil {
				t.Run("a body that is not JSON", func(t *testing.T) {
					request(t, e.routeURL(r, e.Engagement.ID, visitID), http.StatusBadRequest, e.SignedIn, reqtest.RawJSON([]byte("not json")))
				})
			}
			for name, bad := range r.badValues {
				t.Run(name, func(t *testing.T) {
					request(t, e.routeURL(r, e.Engagement.ID, visitID), http.StatusBadRequest, e.SignedIn, bad)
				})
			}
			if r.attachedOnly {
				t.Run("a contractor with no Attachment", func(t *testing.T) {
					contractor := e.As(e.Colleague(fixture.Doula(), fixture.Contractor()))
					request(t, e.routeURL(r, e.Engagement.ID, visitID), http.StatusNotFound, contractor, body...)
				})
			}
		})
	}
}
