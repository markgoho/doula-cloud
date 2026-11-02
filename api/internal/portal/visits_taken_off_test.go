package portal_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

const takenOffDoulaName = "Priya Raman"

// #2596: a Client is not told a Doula is coming to a booked Visit once
// that Doula has been taken off the Engagement. A past Visit keeps the name
// of the person who worked it.
func TestVisitsHandler_LeavesTheNameOffABookedVisitWhoseDoulaWasTakenOff(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-visits-taken-off"
	engagementID, doulaID := seedEngagementForVisits(t, db, identityUID, "Taken Off Practice", takenOffDoulaName)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, ended_at, ended_by)
		 VALUES ($1, $2, 'granted', $2, now(), $2)`, engagementID, doulaID); err != nil {
		t.Fatalf("seed ended attachment: %v", err)
	}
	past := seedPortalVisit(t, db, engagementID, doulaID, time.Now().Add(-30*24*time.Hour))
	upcoming := seedPortalVisit(t, db, engagementID, doulaID, time.Now().Add(7*24*time.Hour))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	names := map[string]string{}
	for _, item := range decodeVisits(t, resp).Items {
		names[item.VisitID] = item.DoulaName
	}
	if names[upcoming] != "" {
		t.Fatalf("booked Visit names %q, want no name", names[upcoming])
	}
	if names[past] != takenOffDoulaName {
		t.Fatalf("past Visit names %q, want the Doula who worked it", names[past])
	}
}
