package visit_test

import (
	"net/http"
	"slices"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

const (
	kindVisitTo = "visit_reassigned_to"
	kindPutOn   = "put_on_engagement"
)

type queuedNotice struct{ recipient, kind, triggeredBy string }

func queuedNotices(t *testing.T, db *testdb.DB) []queuedNotice {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT recipient_staff_id, kind::text, triggered_by_staff_id FROM staff_notice_outbox
		  WHERE status = 'pending' ORDER BY kind`)
	if err != nil {
		t.Fatalf("read staff_notice_outbox: %v", err)
	}
	defer rows.Close()
	var out []queuedNotice
	for rows.Next() {
		var n queuedNotice
		if err := rows.Scan(&n.recipient, &n.kind, &n.triggeredBy); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read rows: %v", err)
	}
	return out
}

// reassignOK hands visitID to staffID as the Staff member at the screen and
// requires the 200.
func (e *env) reassignOK(visitID, staffID string) visit.ReassignResponse {
	e.T.Helper()
	return reqtest.Expect[visit.ReassignResponse](e.T, http.MethodPatch, e.visitsURL("/"+visitID), http.StatusOK,
		e.SignedIn, reqtest.JSON(visit.ReassignRequest{StaffID: staffID}))
}

// #2436: an Admin moving a Visit from one Doula to another queues an email
// to each, in the reassignment's own transaction, naming the Admin as the
// cause, and the response says who was emailed.
func TestReassignHandler_QueuesAnEmailToBothDoulas(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	from, to := e.Colleague(), e.attachedDoula()
	visitID := seedVisit(t, e.DB, e.Engagement.ID, from.ID)

	out := e.reassignOK(visitID, to.ID)

	if want := []string{to.ID, from.ID}; !slices.Equal(out.NotifiedStaffIDs, want) {
		t.Fatalf("notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
	}
	got := queuedNotices(t, e.DB)
	want := []queuedNotice{
		{from.ID, "visit_reassigned_from", e.Staff.ID},
		{to.ID, kindVisitTo, e.Staff.ID},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("queued = %+v, want %+v", got, want)
	}
}

// Nobody is told about a move they made themselves, and a move onto the
// person already holding the Visit tells nobody.
func TestReassignHandler_TellsNobodyAboutTheirOwnMove(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula()))
	other := e.attachedDoula()
	visitID := seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)

	out := e.reassignOK(visitID, other.ID)
	if want := []string{other.ID}; !slices.Equal(out.NotifiedStaffIDs, want) {
		t.Fatalf("off herself: notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
	}
	if got := queuedNotices(t, e.DB); len(got) != 1 || got[0].kind != kindVisitTo {
		t.Fatalf("off herself: queued = %+v, want only the move onto the other Doula", got)
	}

	out = e.reassignOK(visitID, other.ID)
	if len(out.NotifiedStaffIDs) != 0 {
		t.Fatalf("onto the holder: notifiedStaffIds = %v, want none", out.NotifiedStaffIDs)
	}
}
