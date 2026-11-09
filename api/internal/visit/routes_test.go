package visit_test

import (
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

const (
	nobodyID      = "00000000-0000-0000-0000-000000000000"
	scheduledTime = "2027-04-01T09:00:00Z"

	routeReassign = "reassign"
	routeNotes    = "notes"
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
		name: routeReassign, method: http.MethodPatch, path: "/visits/{visit}", staff: []fixture.StaffOption{fixture.Owner(), fixture.Doula()},
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
		name: routeNotes, method: http.MethodPatch, path: "/visits/{visit}/notes", attachedOnly: true,
		body: func(fixture.StaffRecord) reqtest.Option {
			return visitBody(visit.NotesRequest{Notes: new("some notes")})
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

// heldVisit is a Visit held by a Doula colleague.
func (e *env) heldVisit() (visitID string, holder fixture.StaffRecord) {
	e.T.Helper()
	holder = e.Colleague()
	return seedVisit(e.T, e.DB, e.Engagement.ID, holder.ID), holder
}

// TestVisitRoutes_Allow holds the reach the refusals above stop short of.
// An Admin who is not herself a Doula is the person whose job scheduling
// is (ADR-0006, ADR-0008), so handing a Visit to a colleague, setting its
// date and writing its notes are hers to do -- #268 replaced the
// Doula-only guard that used to refuse her -- and "any Staff member who may
// read a Visit may also write its notes", so a contractor with an open,
// granted attachment reaches them too even though she is not the Visit's
// own assigned staff_id. An Owner may hand a Visit to an attached
// contractor.
func TestVisitRoutes_Allow(t *testing.T) {
	cases := []struct {
		name  string
		route string
		staff []fixture.StaffOption
		// setup seeds what the request needs and returns the Visit it is
		// about and the body it carries.
		setup func(e *env) (visitID string, body reqtest.Option)
	}{
		{"reassign by an Admin who is not a Doula", routeReassign, []fixture.StaffOption{fixture.Admin()},
			func(e *env) (string, reqtest.Option) {
				visitID, holder := e.heldVisit()
				return visitID, visitBody(visit.ReassignRequest{StaffID: holder.ID})
			}},
		{"reassign to an attached contractor", routeReassign, []fixture.StaffOption{fixture.Owner(), fixture.Doula()},
			func(e *env) (string, reqtest.Option) {
				contractor := e.Colleague(fixture.Doula(), fixture.Contractor())
				testdb.SeedGrantedAttachment(e.T, e.DB, e.Engagement.ID, contractor.ID)
				return seedVisit(e.T, e.DB, e.Engagement.ID, e.Staff.ID), visitBody(visit.ReassignRequest{StaffID: contractor.ID})
			}},
		{"schedule by an Admin who is not a Doula", "schedule", []fixture.StaffOption{fixture.Admin()},
			func(e *env) (string, reqtest.Option) {
				visitID, _ := e.heldVisit()
				return visitID, visitBody(visit.ScheduleRequest{ScheduledAt: new(scheduledTime)})
			}},
		{"notes by an Admin who is not a Doula", routeNotes, []fixture.StaffOption{fixture.Admin()},
			func(e *env) (string, reqtest.Option) {
				visitID, _ := e.heldVisit()
				return visitID, visitBody(visit.NotesRequest{Notes: new("Admin recorded this on the Doula's behalf.")})
			}},
		{"notes by an attached contractor", routeNotes, []fixture.StaffOption{fixture.Doula(), fixture.Contractor()},
			func(e *env) (string, reqtest.Option) {
				testdb.SeedGrantedAttachment(e.T, e.DB, e.Engagement.ID, e.Staff.ID)
				visitID, _ := e.heldVisit()
				return visitID, visitBody(visit.NotesRequest{Notes: new("Contractor's own observation from the Visit.")})
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff...))
			r := visitRouteNamed(tc.route)
			visitID, body := tc.setup(e)

			reqtest.ExpectStatus(t, r.method, e.routeURL(r, e.Engagement.ID, visitID), http.StatusOK, e.SignedIn, body)
		})
	}
}

// visitRouteNamed is the route called name.
func visitRouteNamed(name string) visitRoute {
	for _, r := range visitRoutes {
		if r.name == name {
			return r
		}
	}
	panic("no visit route called " + name)
}

// list reads the Engagement's Visits as the Staff member at the screen.
func (e *env) list(query string) visit.ListResponse {
	e.T.Helper()
	return reqtest.Expect[visit.ListResponse](e.T, http.MethodGet, e.visitsURL(query), http.StatusOK, e.SignedIn)
}

// TestListHandler_InvalidCursorRejected mirrors
// message.TestListHandler_InvalidCursorRejected.
func TestListHandler_InvalidCursorRejected(t *testing.T) {
	e := newEnv(t)

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		reqtest.ExpectStatus(t, http.MethodGet, e.visitsURL("?cursor="+cursor), http.StatusBadRequest, e.SignedIn)
	}
}

// TestListHandler_PaginatesNewestFirst seeds more than one page of
// Visits and walks the cursor, mirroring
// message.TestListHandler_PaginatesNewestFirst.
func TestListHandler_PaginatesNewestFirst(t *testing.T) {
	e := newEnv(t)

	const total = 31 // pageSize (30) + 1, to force a second page
	for range total {
		seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)
	}

	first := e.list("")
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	second := e.list("?cursor=" + *first.NextCursor)
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
}

func TestCreateHandler_Success(t *testing.T) {
	e := newEnv(t)

	out := e.create(visit.CreateRequest{})

	if out.VisitID == "" || out.StaffID != e.Staff.ID {
		t.Fatalf("unexpected response: %+v, want staffId = %q", out, e.Staff.ID)
	}
}

// An Admin who is not a Doula has no self to put on a birth, so a create
// with no assignee is a refusal for her rather than a silent
// self-assignment (#268). Naming a colleague is what she does instead --
// TestCreateHandler_AdminWhoIsNotADoulaNamesAColleague, in assign_test.go.
func TestCreateHandler_ForbiddenForNonDoula(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))

	reqtest.ExpectStatus(t, http.MethodPost, e.visitsURL(""), http.StatusForbidden, e.SignedIn)
}

func TestListHandler_ReturnsVisitsForEngagement(t *testing.T) {
	e := newEnv(t)
	seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)

	list := e.list("").Items

	if len(list) != 1 || list[0].StaffID != e.Staff.ID {
		t.Fatalf("list = %+v, want one Visit assigned to %q", list, e.Staff.ID)
	}
}

// TestListHandler_ReadableBy covers who reads an Engagement's Visits: an
// Admin who is not a Doula, and a contractor with an open, granted
// attachment.
func TestListHandler_ReadableBy(t *testing.T) {
	cases := []struct {
		name     string
		staff    []fixture.StaffOption
		attached bool
	}{
		{"an Admin who is not a Doula", []fixture.StaffOption{fixture.Admin()}, false},
		{"a contractor with a granted attachment", []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff...))
			if tc.attached {
				testdb.SeedGrantedAttachment(t, e.DB, e.Engagement.ID, e.Staff.ID)
			}
			seedVisit(t, e.DB, e.Engagement.ID, e.Colleague().ID)

			e.list("")
		})
	}
}
