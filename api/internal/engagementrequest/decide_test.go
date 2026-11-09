package engagementrequest_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// requestRoute is one route addressed by a Request id.
type requestRoute struct {
	name   string
	method string
	tail   string
	opts   []reqtest.Option
	// decides is true for a route that decides the Request, so a second call
	// is a conflict.
	decides bool
	// staffOnly is true for a route Owner and Admin hold and a plain Doula
	// does not.
	staffOnly bool
}

var requestRoutes = []requestRoute{
	{name: "approve", method: http.MethodPost, tail: "/approve", decides: true, staffOnly: true},
	{
		name: "refuse", method: http.MethodPost, tail: "/refuse", decides: true, staffOnly: true,
		opts: []reqtest.Option{reqtest.JSON(engagementrequest.RefuseRequest{Reason: "no"})},
	},
	{name: "withdraw", method: http.MethodPost, tail: "/withdraw", decides: true},
	{name: "detail", method: http.MethodGet, staffOnly: true},
}

func (e *env) call(r requestRoute, requestID string, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, r.method, e.requestURL(requestID, r.tail), want, append([]reqtest.Option{e.SignedIn}, r.opts...)...)
}

// The person at the screen in these tests is an Admin who is also a Doula,
// so one seat can reach every route and also be the one who asked.
var asksAndDecides = signedInAs(fixture.Admin(), fixture.Doula())

// TestRequestRoutes_RejectAMalformedOrUnknownRequestID proves a bad id is a
// 400 and a well-formed id nobody here owns is a 404 -- RLS scopes the read,
// so another Practice's Request is indistinguishable from none.
func TestRequestRoutes_RejectAMalformedOrUnknownRequestID(t *testing.T) {
	for _, r := range requestRoutes {
		t.Run(r.name, func(t *testing.T) {
			e := newEnv(t, asksAndDecides)
			e.call(r, notAUUID, http.StatusBadRequest)
			e.call(r, zeroUUID, http.StatusNotFound)
		})
	}
}

// TestRequestRoutes_RefuseADoula proves the approval screen's read and the
// decisions that follow it are Owner/Admin only.
func TestRequestRoutes_RefuseADoula(t *testing.T) {
	for _, r := range requestRoutes {
		if !r.staffOnly {
			continue
		}
		t.Run(r.name, func(t *testing.T) {
			e := newEnv(t, funded)
			e.call(r, e.pending(e.Staff), http.StatusForbidden)
		})
	}
}

// TestRequestRoutes_DecidingTwiceIsAConflict proves a decision cannot be
// made again: a second approval is not a second Engagement, and a decided
// Request cannot be refused or withdrawn.
func TestRequestRoutes_DecidingTwiceIsAConflict(t *testing.T) {
	for _, r := range requestRoutes {
		if !r.decides {
			continue
		}
		t.Run(r.name, func(t *testing.T) {
			e := newEnv(t, asksAndDecides, funded)
			requestID := e.pending(e.Staff)
			e.call(r, requestID, http.StatusOK)
			e.call(r, requestID, http.StatusConflict)
		})
	}
}

// TestDetailHandler_RefusesADecidedRequest proves a stale link to a
// Request somebody already decided says so, rather than rendering an
// approval screen for a decision that has been made.
func TestDetailHandler_RefusesADecidedRequest(t *testing.T) {
	e := newEnv(t, asksAndDecides, funded)
	requestID := e.pending(e.Staff)
	e.approve(requestID)
	e.call(requestRoutes[3], requestID, http.StatusConflict)
}
