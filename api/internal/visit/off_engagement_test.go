package visit_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// #2596: a booked Visit held by a Doula who was taken off the Engagement is
// flagged on both Staff reads. A past Visit, a Visit of a Doula still on it,
// and a Visit on a Doula who never had an Attachment are not.
func TestVisitReads_FlagABookedVisitWhoseDoulaWasTakenOff(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "off-engagement-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	takenOff := testdb.SeedNamedStaffAtPractice(t, db, practiceID, identityUID+"-off", "Lena Vasquez", []string{doulaRole}, employeeType)
	staysOn := testdb.SeedNamedStaffAtPractice(t, db, practiceID, identityUID+"-on", "Priya Raman", []string{doulaRole}, employeeType)
	never := testdb.SeedNamedStaffAtPractice(t, db, practiceID, identityUID+"-never", "Joss Adeyemi", []string{doulaRole}, employeeType)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, ended_at, ended_by)
		 VALUES ($1, $2, 'granted', $2, now(), $2)`, engagementID, takenOff); err != nil {
		t.Fatalf("seed ended attachment: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by)
		 VALUES ($1, $2, 'granted', $2)`, engagementID, staysOn); err != nil {
		t.Fatalf("seed open attachment: %v", err)
	}
	soon := time.Now().Add(3 * 24 * time.Hour)
	offBooked := seedScheduledVisit(t, db, engagementID, takenOff, soon)
	offPast := seedScheduledVisit(t, db, engagementID, takenOff, time.Now().Add(-3*24*time.Hour))
	onBooked := seedScheduledVisit(t, db, engagementID, staysOn, soon)
	neverBooked := seedScheduledVisit(t, db, engagementID, never, soon)
	unscheduled := seedVisit(t, db, engagementID, takenOff)
	want := map[string]bool{offBooked: true, offPast: false, onBooked: false, neverBooked: false, unscheduled: false}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	var list visit.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, err = %v", resp.StatusCode, err)
	}
	for _, v := range list.Items {
		if v.IsOffEngagement != want[v.VisitID] {
			t.Errorf("engagement list: visit %s isOffEngagement = %v, want %v", v.VisitID, v.IsOffEngagement, want[v.VisitID])
		}
	}

	page := getSchedule(t, session, schedulePath(srv.URL, practiceID, nil))
	if len(page.Items) != 3 {
		t.Fatalf("schedule items = %d, want the 3 booked Visits", len(page.Items))
	}
	for _, v := range page.Items {
		if v.IsOffEngagement != want[v.VisitID] {
			t.Errorf("schedule: visit %s isOffEngagement = %v, want %v", v.VisitID, v.IsOffEngagement, want[v.VisitID])
		}
	}
}
