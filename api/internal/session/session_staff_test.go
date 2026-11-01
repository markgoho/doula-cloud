package session_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/testdb"
)

func decodeCreate(t *testing.T, resp *http.Response) session.CreateResponse {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body session.CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	return body
}

// #2434: signing in answers with the Staff session itself, so /login
// lands without a second GET /api/staff/session.
func TestCreateHandler_AnswersWithTheStaffSession(t *testing.T) {
	const uid = "uid-staff-session"
	srv, db := newServer(t, authntest.Verifier{UID: uid, SecondFactor: true})
	defer srv.Close()
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{"owner"}, "employee")

	resp := postCreate(t, srv, "good-token")
	defer resp.Body.Close()
	body := decodeCreate(t, resp)

	if !body.OK {
		t.Error("ok = false, want true")
	}
	if body.Staff == nil {
		t.Fatal("staff is absent, want the caller's Staff session")
	}
	if body.Staff.StaffID != staffID {
		t.Errorf("staffId = %q, want %q", body.Staff.StaffID, staffID)
	}
	if len(body.Staff.Memberships) != 1 || body.Staff.Memberships[0].PracticeID != practiceID {
		t.Errorf("memberships = %+v, want one at %s", body.Staff.Memberships, practiceID)
	}
	if !body.Staff.SecondFactor {
		t.Error("secondFactor = false, want the minted session's own second factor")
	}
	if !body.Staff.SoleOwner {
		t.Error("soleOwner = false, want true for the Practice's only Owner")
	}
}

// An identity with no Staff row still gets its session, with no staff in
// the body: the app reads GET /api/staff/session for that 404's answer.
func TestCreateHandler_LeavesStaffOutWhenThereIsNoStaffRow(t *testing.T) {
	srv, _ := newServer(t, authntest.Verifier{UID: "uid-no-staff"})
	defer srv.Close()

	resp := postCreate(t, srv, "good-token")
	defer resp.Body.Close()
	body := decodeCreate(t, resp)

	if body.Staff != nil {
		t.Errorf("staff = %+v, want absent for an identity with no Staff row", body.Staff)
	}
	if sessionCookie(resp) == nil {
		t.Error("no __session cookie set")
	}
}
