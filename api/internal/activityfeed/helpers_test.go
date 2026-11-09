package activityfeed_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	ownerRole      = "owner"
	adminRole      = "admin"
	doulaRole      = "doula"
	employeeType   = "employee"
	contractorType = "contractor"
	// renataAlvarez is membership_test.go's own Membership subject name,
	// shared across three tests there so goconst has one place to point
	// at rather than three literals free to drift.
	renataAlvarez = "Renata Alvarez"
	priyaRaman    = "Priya Raman"
)

// newServer mounts this package's route through activityfeed.Mount, the
// same call main.go makes on the real GatedRouter, and seeds a live
// session for uid.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	activityfeed.Mount(g)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// readFeed signs in as uid and reads the first page of the Practice's
// activity feed.
func readFeed(t *testing.T, db *testdb.DB, uid, practiceID string) activityfeed.ListResponse {
	t.Helper()
	srv, session := newServer(t, db, uid)
	defer srv.Close()
	return reqtest.Expect[activityfeed.ListResponse](t, http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/activity", http.StatusOK, reqtest.Session(session))
}

func authedGet(t *testing.T, session, url string) *http.Response {
	t.Helper()
	return reqtest.Get(t, url, reqtest.Session(session))
}
