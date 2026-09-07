package activityfeed_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	ownerRole      = "owner"
	doulaRole      = "doula"
	employeeType   = "employee"
	contractorType = "contractor"
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

func authedGet(t *testing.T, session, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}
