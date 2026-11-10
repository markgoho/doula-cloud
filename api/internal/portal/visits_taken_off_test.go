package portal_test

import (
	"testing"
	"time"
)

// #2596: a Client is not told a Doula is coming to a booked Visit once
// that Doula has been taken off the Engagement. A past Visit keeps the name
// of the person who worked it.
func TestVisitsHandler_LeavesTheNameOffABookedVisitWhoseDoulaWasTakenOff(t *testing.T) {
	e := newEnv(t)
	e.Exec(`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, ended_at, ended_by, ended_reason)
		 VALUES ($1, $2, 'granted', $2, now(), $2, 'taken_off')`, e.Engagement.ID, e.Staff.ID)
	past := e.visit(time.Now().Add(-30 * 24 * time.Hour))
	upcoming := e.visit(time.Now().Add(7 * 24 * time.Hour))

	names := map[string]string{}
	for _, item := range e.visitsPage("").Items {
		names[item.VisitID] = item.DoulaName
	}
	if names[upcoming] != "" {
		t.Fatalf("booked Visit names %q, want no name", names[upcoming])
	}
	if names[past] != doulaName {
		t.Fatalf("past Visit names %q, want the Doula who worked it", names[past])
	}
}
