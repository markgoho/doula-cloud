package message_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// TestRemoveMembershipHandler_LeavesHerNamedInThreadAndSchedule is
// #1455's end-to-end half: an Owner removes a Doula through the real
// DELETE route, not a fixture, and both Staff-side lists that fall back
// to activity.DepartedStaffName -- message.ListHandler's thread and
// visit.listVisits' Engagement Visits -- still name her. That is 00116's
// own claim: staff_visible_to_own_practice_membership_history is
// table-wide, so the 'removed' event RemoveMembershipHandler writes
// reaches her row for every Staff-side reader, not only the Activity
// feed it was written for.
func TestRemoveMembershipHandler_LeavesHerNamedInThreadAndSchedule(t *testing.T) {
	const ownerUID = "departure-e2e-owner"
	const doulaUID = "departure-e2e-doula"
	const doulaName = "Priya Chandra"

	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Departure End To End Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")
	doulaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, doulaUID, doulaName, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Nadia Client", "nadia-departure-e2e@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2)`, engagementID, doulaID,
	); err != nil {
		t.Fatalf("seed visit: %v", err)
	}

	// The removal route exactly as staffauth/mount.go wires it, beside the
	// two read routes under test.
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	message.Mount(g, ir, db.App, objectstore.NewMemoryStore(), push.NewFakePusher())
	visit.Mount(g, ir)
	ir.ExemptGated("DELETE /api/practices/{practiceId}/staff/{staffId}/membership",
		"test mount of the production route", false, staffauth.OwnerOnly, staffauth.RemoveMembershipHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ownerSession := authntest.SeedSession(t, db.App, ownerUID)
	doulaSession := authntest.SeedSession(t, db.App, doulaUID)

	engagementURL := srv.URL + "/api/practices/" + practiceID + "/engagements/" + engagementID
	b, _ := json.Marshal(message.CreateRequest{Body: "See you Thursday."})
	created := authedPost(t, doulaSession, engagementURL+"/messages", b)
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", created.StatusCode, http.StatusCreated)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete,
		srv.URL+"/api/practices/"+practiceID+"/staff/"+doulaID+"/membership", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, ownerSession)
	req.Header.Set("X-Confirmed", "true")
	removed, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("remove request: %v", err)
	}
	defer removed.Body.Close()
	if removed.StatusCode != http.StatusNoContent {
		t.Fatalf("remove status = %d, want %d", removed.StatusCode, http.StatusNoContent)
	}

	threadResp := authedGet(t, ownerSession, engagementURL+"/messages")
	defer threadResp.Body.Close()
	if threadResp.StatusCode != http.StatusOK {
		t.Fatalf("thread status = %d, want %d", threadResp.StatusCode, http.StatusOK)
	}
	var thread message.ListResponse
	if err := json.NewDecoder(threadResp.Body).Decode(&thread); err != nil {
		t.Fatalf("decode thread: %v", err)
	}
	if len(thread.Items) != 1 || thread.Items[0].SenderName != doulaName {
		t.Fatalf("thread = %+v, want her one Message, sent by %q", thread.Items, doulaName)
	}

	visitsResp := authedGet(t, ownerSession, engagementURL+"/visits")
	defer visitsResp.Body.Close()
	if visitsResp.StatusCode != http.StatusOK {
		t.Fatalf("visits status = %d, want %d", visitsResp.StatusCode, http.StatusOK)
	}
	var visits visit.ListResponse
	if err := json.NewDecoder(visitsResp.Body).Decode(&visits); err != nil {
		t.Fatalf("decode visits: %v", err)
	}
	if len(visits.Items) != 1 || visits.Items[0].StaffName != doulaName {
		t.Fatalf("visits = %+v, want her one Visit, assigned to %q", visits.Items, doulaName)
	}
}
