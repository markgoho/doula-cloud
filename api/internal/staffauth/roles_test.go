package staffauth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func newRolesServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("PATCH /practices/{practiceId}/staff/{staffId}/roles",
		staffauth.Middleware(db.App)(staffauth.AssignRolesHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func patchRoles(t *testing.T, srv *httptest.Server, session string, practiceID, staffID string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch,
		srv.URL+"/practices/"+practiceID+"/staff/"+staffID+"/roles", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestAssignRolesHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-assigning-roles"
	staffID, practiceID := seedStaffWithMembership(t, db, identityUID) // '{doula}', not owner

	srv, session := newRolesServer(t, db, identityUID)
	defer srv.Close()

	resp := patchRoles(t, srv, session, practiceID, staffID, staffauth.AssignRolesRequest{Roles: []string{ownerRole}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestAssignRolesHandler_UnknownRole(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-bad-role"
	ownerID, practiceID := seedOwnerMembership(t, db, identityUID)

	srv, session := newRolesServer(t, db, identityUID)
	defer srv.Close()

	resp := patchRoles(t, srv, session, practiceID, ownerID, staffauth.AssignRolesRequest{Roles: []string{"admin"}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAssignRolesHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-invalid-roles-body"
	ownerID, practiceID := seedOwnerMembership(t, db, identityUID)

	srv, session := newRolesServer(t, db, identityUID)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch,
		srv.URL+"/practices/"+practiceID+"/staff/"+ownerID+"/roles", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAssignRolesHandler_NoSuchMembership(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-no-such-target"
	_, practiceID := seedOwnerMembership(t, db, identityUID)

	srv, session := newRolesServer(t, db, identityUID)
	defer srv.Close()

	resp := patchRoles(t, srv, session, practiceID, "00000000-0000-0000-0000-000000000000", staffauth.AssignRolesRequest{Roles: []string{doulaRole}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAssignRolesHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-assigns-roles"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	targetID := seedStaff(t, db, "target-staff")
	seedMembership(t, db, practiceID, targetID) // starts with '{doula}'

	srv, session := newRolesServer(t, db, ownerUID)
	defer srv.Close()

	resp := patchRoles(t, srv, session, practiceID, targetID, staffauth.AssignRolesRequest{Roles: []string{ownerRole, doulaRole}})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var roles string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT array_to_string(roles, ',') FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, targetID,
	).Scan(&roles); err != nil {
		t.Fatalf("query updated roles: %v", err)
	}
	if roles != "owner,doula" {
		t.Fatalf("roles = %q, want %q", roles, "owner,doula")
	}
}

// TestRoles exercises staffauth.Roles directly -- the function main.go's
// practiceSessionHandler calls so the frontend can gate Owner-only UI
// (like the invite link) on the caller's actual roles.
func TestRoles(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Roles Fn Test")
	doulaID := seedStaff(t, db, "roles-fn-doula")
	seedMembership(t, db, practiceID, doulaID) // seeds '{doula}'

	zeroRoleID := seedStaff(t, db, "roles-fn-zero")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{}', 'employee')`,
		practiceID, zeroRoleID,
	); err != nil {
		t.Fatalf("seed zero-role membership: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	roles, err := staffauth.Roles(t.Context(), tx, practiceID, doulaID)
	if err != nil {
		t.Fatalf("Roles(doula): %v", err)
	}
	if len(roles) != 1 || roles[0] != doulaRole {
		t.Fatalf("Roles(doula) = %v, want [doula]", roles)
	}

	zeroRoles, err := staffauth.Roles(t.Context(), tx, practiceID, zeroRoleID)
	if err != nil {
		t.Fatalf("Roles(zero-role): %v", err)
	}
	if len(zeroRoles) != 0 {
		t.Fatalf("Roles(zero-role) = %v, want empty", zeroRoles)
	}
}
