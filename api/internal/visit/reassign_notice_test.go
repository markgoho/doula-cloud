package visit_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

type queuedNotice struct{ recipient, kind, triggeredBy string }

func queuedNotices(t *testing.T, db *testdb.DB, visitID string) []queuedNotice {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT recipient_staff_id, kind::text, triggered_by_staff_id FROM staff_notice_outbox
		  WHERE subject_id = $1 AND status = 'pending' ORDER BY kind`, visitID)
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

func reassign(t *testing.T, db *testdb.DB, uid, practiceID, engagementID, visitID, to string) visit.ReassignResponse {
	t.Helper()
	srv, session := newServer(t, db, uid)
	defer srv.Close()
	body, err := json.Marshal(visit.ReassignRequest{StaffID: to})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out visit.ReassignResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// #2436: an Admin moving a Visit from one Doula to another queues an email
// to each, in the reassignment's own transaction, naming the Admin as the
// cause, and the response says who was emailed.
func TestReassignHandler_QueuesAnEmailToBothDoulas(t *testing.T) {
	db := testdb.New(t)
	const adminUID = "admin-reassigning"
	practiceID, adminID := testdb.SeedStaffAtNewPractice(t, db, adminUID, []string{"admin"}, "employee")
	fromID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-from", []string{doulaRole}, "employee")
	toID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-to", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, fromID)

	out := reassign(t, db, adminUID, practiceID, engagementID, visitID, toID)

	if want := []string{toID, fromID}; !slices.Equal(out.NotifiedStaffIDs, want) {
		t.Fatalf("notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
	}
	got := queuedNotices(t, db, visitID)
	want := []queuedNotice{
		{fromID, "visit_reassigned_from", adminID},
		{toID, "visit_reassigned_to", adminID},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("queued = %+v, want %+v", got, want)
	}
}

// Nobody is told about a move they made themselves, and a move onto the
// person already holding the Visit tells nobody.
func TestReassignHandler_TellsNobodyAboutTheirOwnMove(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reassigning"
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, ownerUID, []string{ownerRole, doulaRole}, "employee")
	otherID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-other", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, ownerID)

	out := reassign(t, db, ownerUID, practiceID, engagementID, visitID, otherID)
	if want := []string{otherID}; !slices.Equal(out.NotifiedStaffIDs, want) {
		t.Fatalf("off herself: notifiedStaffIds = %v, want %v", out.NotifiedStaffIDs, want)
	}
	if got := queuedNotices(t, db, visitID); len(got) != 1 || got[0].kind != "visit_reassigned_to" {
		t.Fatalf("off herself: queued = %+v, want only the move onto the other Doula", got)
	}

	out = reassign(t, db, ownerUID, practiceID, engagementID, visitID, otherID)
	if len(out.NotifiedStaffIDs) != 0 {
		t.Fatalf("onto the holder: notifiedStaffIds = %v, want none", out.NotifiedStaffIDs)
	}
}
