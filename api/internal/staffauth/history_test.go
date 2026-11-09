package staffauth_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
)

// historyPage is what a paginated history route answered, reduced to what
// the cursor tests read.
type historyPage struct {
	count          int
	hasMore        bool
	next           *string
	newest, oldest time.Time
}

// historyKind is one of the two Staff histories a roster hangs off the
// same way: each reads through the roster's gate, resolves the Staff
// member through her Membership, and pages newest first through a cursor.
type historyKind struct {
	name  string
	route string
	// seed lays out total moves on staff's trail, so a page boundary falls
	// inside them.
	seed func(e *env, staff fixture.StaffRecord, total int)
	page func(e *env, staffID, query string) historyPage
}

var historyKinds = []historyKind{
	{
		name:  "Membership history",
		route: "membership-history",
		seed: func(e *env, staff fixture.StaffRecord, total int) {
			types := [2]string{employeeType, "contractor"}
			start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			for i := range total {
				seedMembershipEvent(e.T, e.DB, e.Practice.ID, staff.ID, "employment_type_changed",
					"", "", types[i%2], types[(i+1)%2], e.Staff.ID, start.AddDate(0, i, 0))
			}
		},
		page: func(e *env, staffID, query string) historyPage {
			h := e.history(staffID, query)
			return historyPage{len(h.Items), h.HasMore, h.NextCursor, h.Items[0].CreatedAt, h.Items[len(h.Items)-1].CreatedAt}
		},
	},
	{
		name:  "work state history",
		route: "work-state-history",
		seed: func(e *env, staff fixture.StaffRecord, total int) {
			// Alternating moves, so every row survives the changes-only
			// filter.
			states := [2]string{"NY", "NJ"}
			start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			seedWorkStateEvent(e.T, e.DB, staff.ID, "", states[0], start)
			for i := 1; i < total; i++ {
				seedWorkStateEvent(e.T, e.DB, staff.ID, states[(i-1)%2], states[i%2], start.AddDate(0, i, 0))
			}
		},
		page: func(e *env, staffID, query string) historyPage {
			h := reqtest.Expect[staffauth.WorkStateHistory](e.T, http.MethodGet, e.staffRoute(staffID, "work-state-history"+query), http.StatusOK, e.SignedIn)
			return historyPage{len(h.Items), h.HasMore, h.NextCursor, h.Items[0].CreatedAt, h.Items[len(h.Items)-1].CreatedAt}
		},
	},
}

// staffRoute is the route of staffID's resource under the Practice.
func (e *env) staffRoute(staffID, resource string) string {
	return e.PracticeURL("/staff/" + staffID + "/" + resource)
}

// TestHistory_Refuses holds each history's reach at the boundary that
// enforces it: a Doula has neither the roster nor what hangs off it (#459,
// #872), a cursor or staff id that is not one is a 400, and a real Staff
// member at somebody else's Practice is a 404 -- the Membership read
// doubles as the existence check, and an empty history would confirm she
// exists.
func TestHistory_Refuses(t *testing.T) {
	cases := []struct {
		name  string
		owner bool
		// request returns the staff id and query the request carries.
		request func(e *env) (staffID, query string)
		want    int
	}{
		{"a Doula", false, func(e *env) (string, string) { return e.Staff.ID, "" }, http.StatusForbidden},
		{"a cursor that is not one", true, func(e *env) (string, string) { return e.Staff.ID, "?cursor=not-a-cursor" }, http.StatusBadRequest},
		{"a staff id that is not a uuid", true, func(*env) (string, string) { return notAUUID, "" }, http.StatusBadRequest},
		{"a stranger at another Practice", true, func(e *env) (string, string) {
			return fixture.Staff(e.T, e.DB, fixture.Practice(e.T, e.DB)).ID, ""
		}, http.StatusNotFound},
	}
	for _, kind := range historyKinds {
		for _, tc := range cases {
			t.Run(kind.name+"/"+tc.name, func(t *testing.T) {
				var opts []envOption
				if tc.owner {
					opts = append(opts, signedInAs(fixture.Owner()))
				}
				e := newEnv(t, opts...)
				staffID, query := tc.request(e)

				reqtest.ExpectStatus(t, http.MethodGet, e.staffRoute(staffID, kind.route+query), tc.want, e.SignedIn)
			})
		}
	}
}

// TestHistory_PaginatesOldestPagesLast is the AC about a long-lived
// Membership or a Practice whose people have corrected their work state
// many times: the screen asks for one page and the rest stays reachable
// through the cursor rather than being cut off.
func TestHistory_PaginatesOldestPagesLast(t *testing.T) {
	for _, kind := range historyKinds {
		t.Run(kind.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()))
			doula := e.Colleague()
			const total = 25
			kind.seed(e, doula, total)

			page1 := kind.page(e, doula.ID, "")
			if page1.count != 20 {
				t.Fatalf("first page items = %d, want 20", page1.count)
			}
			if !page1.hasMore || page1.next == nil {
				t.Fatalf("first page hasMore = %v, nextCursor = %v, want more", page1.hasMore, page1.next)
			}

			page2 := kind.page(e, doula.ID, "?cursor="+*page1.next)
			if page2.count != total-20 {
				t.Fatalf("second page items = %d, want %d", page2.count, total-20)
			}
			if page2.hasMore {
				t.Error("second page hasMore = true, want false")
			}
			if page2.newest.After(page1.oldest) {
				t.Error("second page starts newer than the first page ended")
			}
		})
	}
}
