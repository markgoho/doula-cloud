package client_test

import (
	"encoding/json/v2"
	"maps"
	"net/http"
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func getDoulaCounts(t *testing.T, session, url string) client.DoulaCountsResponse {
	t.Helper()
	resp := authedGet(t, session, url)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)
	var got client.DoulaCountsResponse
	if err := json.UnmarshalRead(resp.Body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return got
}

// doulaCountsFixture seeds the Practice both tests read. Ana holds two Clients
// (one of them through two Engagements, still one Client); Bo's attachment on
// a Client has ended, another is not granted, and a third sits on a completed
// Engagement; the Owner, who is also a Doula, holds one.
type doulaCountsFixture struct {
	practiceID, ownerID, anaID, boID string
}

func seedDoulaCountsFixture(t *testing.T, db *testdb.DB) doulaCountsFixture {
	t.Helper()
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "owner-doula-counts", []string{ownerRole, "doula"}, "employee")
	anaID := testdb.SeedContractorAtPractice(t, db, practiceID, "ana-doula-counts")
	boID := testdb.SeedContractorAtPractice(t, db, practiceID, "bo-doula-counts")

	twiceID, twiceEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Dana", "dc-twice@example.com", "active")
	var second string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, 'active', 'postpartum') RETURNING id`,
		twiceID, practiceID).Scan(&second); err != nil {
		t.Fatalf("seed second engagement: %v", err)
	}
	testdb.SeedGrantedAttachment(t, db, twiceEngagement, anaID)
	testdb.SeedGrantedAttachment(t, db, second, anaID)
	testdb.SeedGrantedAttachment(t, db, twiceEngagement, ownerID)

	_, amara := testdb.SeedEngagementInStatus(t, db, practiceID, "Amara", "dc-amara@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, amara, anaID)
	testdb.SeedAttachment(t, db, amara, boID, "granted", true)

	_, bex := testdb.SeedEngagementInStatus(t, db, practiceID, "Bex", "dc-bex@example.com", "active")
	testdb.SeedAttachment(t, db, bex, boID, "accrued", false)

	_, cleo := testdb.SeedEngagementInStatus(t, db, practiceID, "Cleo", "dc-cleo@example.com", "completed")
	testdb.SeedGrantedAttachment(t, db, cleo, boID)

	return doulaCountsFixture{practiceID: practiceID, ownerID: ownerID, anaID: anaID, boID: boID}
}

// An open versus an ended attachment, a completed Engagement and an origin
// that is not 'granted' each leave a Doula uncounted, and the reader's own
// count rides along.
func TestDoulaCountsHandler_CountsLiveAttachmentsPerDoula(t *testing.T) {
	db := testdb.New(t)
	f := seedDoulaCountsFixture(t, db)
	srv, session := newServer(t, db, "owner-doula-counts")
	defer srv.Close()

	got := getDoulaCounts(t, session, srv.URL+"/api/practices/"+f.practiceID+"/clients/doula-counts")

	want := client.DoulaCountsResponse{Mine: 1, Doulas: map[string]int{f.anaID: 2, f.ownerID: 1}}
	if got.Mine != want.Mine || !maps.Equal(got.Doulas, want.Doulas) {
		t.Errorf("counts = %+v, want %+v", got, want)
	}
}

// A contractor is told her own count and no other Doula's.
func TestDoulaCountsHandler_ContractorSeesOnlyHerOwn(t *testing.T) {
	db := testdb.New(t)
	f := seedDoulaCountsFixture(t, db)
	srv, session := newServer(t, db, "ana-doula-counts")
	defer srv.Close()

	got := getDoulaCounts(t, session, srv.URL+"/api/practices/"+f.practiceID+"/clients/doula-counts")

	want := client.DoulaCountsResponse{Mine: 2, Doulas: map[string]int{f.anaID: 2}}
	if got.Mine != want.Mine || !maps.Equal(got.Doulas, want.Doulas) {
		t.Errorf("counts = %+v, want %+v", got, want)
	}
}

// A Doula holding nothing live gets zero and an empty map, never null.
func TestDoulaCountsHandler_NoLiveClientsIsEmpty(t *testing.T) {
	db := testdb.New(t)
	f := seedDoulaCountsFixture(t, db)
	srv, session := newServer(t, db, "bo-doula-counts")
	defer srv.Close()

	got := getDoulaCounts(t, session, srv.URL+"/api/practices/"+f.practiceID+"/clients/doula-counts")

	if got.Mine != 0 || got.Doulas == nil || len(got.Doulas) != 0 {
		t.Errorf("counts = %+v, want zero mine and an empty map", got)
	}
}
