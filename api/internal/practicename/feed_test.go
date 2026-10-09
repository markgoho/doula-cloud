package practicename_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/practicename"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestPutHandler_TheRenameShowsInThePracticesActivity is the AC's read
// side: the row the write records is the row the Practice-wide feed
// returns to an Owner.
func TestPutHandler_TheRenameShowsInThePracticesActivity(t *testing.T) {
	db := testdb.New(t)
	const uid = "name-feed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	practicename.Mount(idempotency.NewRouter(g, db.App))
	activityfeed.Mount(g)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	session := authntest.SeedSession(t, db.App, uid)

	put := putName(t, srv, session, practiceID, newName)
	_ = put.Body.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/activity", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	var feed activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
		t.Fatalf("decode feed: %v", err)
	}
	for _, item := range feed.Items {
		if item.Action == "practice_name_changed" {
			return
		}
	}
	t.Fatalf("feed = %+v, want a practice_name_changed row", feed.Items)
}
