package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// #2597: the Note typed on the Request that started an Engagement is read
// back on the Engagement, with who wrote it, for an Owner or an Admin only.
func TestDetailHandler_StartNoteForOwnersAndAdminsOnly(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "start-note-owner"
	const adminUID = "start-note-admin"
	const employeeUID = "start-note-employee"
	practiceID := testdb.SeedPractice(t, db, "Start Note Practice")
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, ownerUID, "Renata Owner", []string{ownerRole}, employeeType)
	adminID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, adminUID, "Dee Admin", []string{adminRole}, employeeType)
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, employeeUID, "Priya Employee", []string{doulaRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Lucia", "start-note-client@example.com", engagement.StatusActive)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, note, state, requested_by, decided_by, decided_at, engagement_id)
		 SELECT e.practice_id, e.client_id, e.kind, $2, 'approved', $3, $3, now(), e.id FROM engagements e WHERE e.id = $1`,
		engagementID, "Heard about us from her midwife.", adminID); err != nil {
		t.Fatalf("seed request: %v", err)
	}

	srv, _ := newServer(t, db, ownerUID)
	defer srv.Close()

	for _, uid := range []string{ownerUID, adminUID} {
		note := readDetail(t, db, srv, uid, practiceID, engagementID).StartNote
		if note == nil || note.Text != "Heard about us from her midwife." || note.WrittenBy != "Dee Admin" || note.WrittenAt.IsZero() {
			t.Errorf("startNote for %s = %+v, want Dee Admin's note", uid, note)
		}
	}
	if note := readDetail(t, db, srv, employeeUID, practiceID, engagementID).StartNote; note != nil {
		t.Errorf("startNote for an employee Doula = %+v, want none", note)
	}
}

func TestDetailHandler_NoStartNoteWhenNoneWasWritten(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "no-start-note-owner"
	practiceID := testdb.SeedPractice(t, db, "No Start Note Practice")
	ownerID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, ownerUID, "Renata Owner", []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Lucia", "no-start-note-client@example.com", engagement.StatusActive)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, state, requested_by, decided_by, decided_at, engagement_id)
		 SELECT e.practice_id, e.client_id, e.kind, 'approved', $2, $2, now(), e.id FROM engagements e WHERE e.id = $1`,
		engagementID, ownerID); err != nil {
		t.Fatalf("seed request: %v", err)
	}

	srv, _ := newServer(t, db, ownerUID)
	defer srv.Close()

	if note := readDetail(t, db, srv, ownerUID, practiceID, engagementID).StartNote; note != nil {
		t.Errorf("startNote = %+v, want none", note)
	}
}
