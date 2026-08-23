package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newStaffListServer mounts ListStaffHandler the way main.go really does
// -- through GatedRouter with the "owner","admin" declaration -- since
// the Owner/Admin-vs-Doula boundary lives at that mount, not inside the
// handler (#315).
func newStaffListServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	g.Get("/practices/{practiceId}/staff", []string{ownerRole, adminRole}, staffauth.ListStaffHandler())
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getStaffList(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+"/staff", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestListStaffHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-listing-staff"
	_, practiceID := seedStaffWithMembership(t, db, identityUID) // '{doula}', not owner

	srv, session := newStaffListServer(t, db, identityUID)
	defer srv.Close()

	resp := getStaffList(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestListStaffHandler_AdminSuccess proves ADR-0008's read table widened
// this route past Owner-only: an Admin membership, which the pre-#315
// handler's internal RequireOwner check would have 403'd, now reaches it
// because the mount declares both roles.
func TestListStaffHandler_AdminSuccess(t *testing.T) {
	db := testdb.New(t)
	const adminUID = "admin-lists-staff"
	practiceID := seedPractice(t, db, "Admin Roster Practice")
	adminID := seedStaff(t, db, adminUID)
	seedMembershipWithRoles(t, db, practiceID, adminID, "{admin}")

	srv, session := newStaffListServer(t, db, adminUID)
	defer srv.Close()

	resp := getStaffList(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestListStaffHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-lists-staff"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := seedStaff(t, db, "doula-on-roster")
	seedMembership(t, db, practiceID, doulaID) // seeds '{doula}'

	zeroRoleID := seedStaff(t, db, "zero-role-on-roster")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{}', 'employee')`,
		practiceID, zeroRoleID,
	); err != nil {
		t.Fatalf("seed zero-role membership: %v", err)
	}

	srv, session := newStaffListServer(t, db, ownerUID)
	defer srv.Close()

	resp := getStaffList(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var list []staffauth.StaffSummary
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("staff list = %+v, want 3 entries", list)
	}

	byID := map[string]staffauth.StaffSummary{}
	for _, s := range list {
		byID[s.StaffID] = s
	}
	owner, ok := byID[ownerID]
	if !ok || len(owner.Roles) != 1 || owner.Roles[0] != ownerRole {
		t.Fatalf("owner entry = %+v, want roles [owner]", owner)
	}
	doula, ok := byID[doulaID]
	if !ok || len(doula.Roles) != 1 || doula.Roles[0] != doulaRole {
		t.Fatalf("doula entry = %+v, want roles [doula]", doula)
	}
	zeroRole, ok := byID[zeroRoleID]
	if !ok || len(zeroRole.Roles) != 0 {
		t.Fatalf("zero-role entry = %+v, want no roles", zeroRole)
	}
}
