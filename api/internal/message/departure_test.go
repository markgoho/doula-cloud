package message_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
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
	const doulaName = "Priya Chandra"

	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Departure End To End Practice"))
	owner := fixture.Staff(t, db, p, fixture.Owner())
	doula := fixture.Staff(t, db, p, fixture.StaffName(doulaName))
	engagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Nadia Client"), fixture.ClientEmail("nadia-departure-e2e@example.com")))
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2)`, engagement.ID, doula.ID,
	); err != nil {
		t.Fatalf("seed visit: %v", err)
	}

	// The removal route exactly as staffauth/mount.go wires it, beside the
	// two read routes under test.
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	message.Mount(g, ir, db.App, objectstore.NewMemoryStore(), push.NewFakePusher())
	visit.Mount(g, ir, &tasknudge.FakeEnqueuer{})
	ir.ExemptGated("DELETE /api/practices/{practiceId}/staff/{staffId}/membership",
		"test mount of the production route", false, staffauth.OwnerOnly, staffauth.RemoveMembershipHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ownerSession := fixture.Session(t, db, owner)
	doulaSession := fixture.Session(t, db, doula)

	engagementURL := srv.URL + "/api/practices/" + p.ID + "/engagements/" + engagement.ID
	b, _ := json.Marshal(message.CreateRequest{Body: "See you Thursday."})
	created := authedPost(t, doulaSession, engagementURL+"/messages", b)
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", created.StatusCode, http.StatusCreated)
	}

	removed := reqtest.Delete(t, srv.URL+"/api/practices/"+p.ID+"/staff/"+doula.ID+"/membership", reqtest.Session(ownerSession), reqtest.Header("X-Confirmed", "true"))
	defer removed.Body.Close()
	if removed.StatusCode != http.StatusNoContent {
		t.Fatalf("remove status = %d, want %d", removed.StatusCode, http.StatusNoContent)
	}

	threadResp := authedGet(t, ownerSession, engagementURL+"/messages")
	defer threadResp.Body.Close()
	if threadResp.StatusCode != http.StatusOK {
		t.Fatalf("thread status = %d, want %d", threadResp.StatusCode, http.StatusOK)
	}
	thread := reqtest.Decode[message.ListResponse](t, threadResp)
	if len(thread.Items) != 1 || thread.Items[0].SenderName != doulaName {
		t.Fatalf("thread = %+v, want her one Message, sent by %q", thread.Items, doulaName)
	}

	visitsResp := authedGet(t, ownerSession, engagementURL+"/visits")
	defer visitsResp.Body.Close()
	if visitsResp.StatusCode != http.StatusOK {
		t.Fatalf("visits status = %d, want %d", visitsResp.StatusCode, http.StatusOK)
	}
	visits := reqtest.Decode[visit.ListResponse](t, visitsResp)
	if len(visits.Items) != 1 || visits.Items[0].StaffName != doulaName {
		t.Fatalf("visits = %+v, want her one Visit, assigned to %q", visits.Items, doulaName)
	}
}
