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

func newMembershipServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("PATCH /practices/{practiceId}/staff/{staffId}/membership",
		staffauth.Middleware(db.App)(staffauth.UpdateMembershipHandler()))
	mux.Handle("DELETE /practices/{practiceId}/staff/{staffId}/membership",
		staffauth.Middleware(db.App)(staffauth.RemoveMembershipHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func deleteMembership(t *testing.T, srv *httptest.Server, session, practiceID, staffID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete,
		srv.URL+"/practices/"+practiceID+"/staff/"+staffID+"/membership", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("X-Confirmed", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func patchMembership(t *testing.T, srv *httptest.Server, session, practiceID, staffID string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return patchMembershipRaw(t, srv, session, practiceID, staffID, payload)
}

func patchMembershipRaw(t *testing.T, srv *httptest.Server, session, practiceID, staffID string, payload []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch,
		srv.URL+"/practices/"+practiceID+"/staff/"+staffID+"/membership", bytes.NewReader(payload))
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

// membershipEvents reads a Membership's recorded history, newest first.
func membershipEvents(t *testing.T, db *testdb.DB, practiceID, staffID string) []string {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT action || ':' || COALESCE(diff->'roles'->>'from', '') ||
		        '->' || COALESCE(diff->'roles'->>'to', '') ||
		        '/' || COALESCE(diff->'employmentType'->>'from', '') ||
		        '->' || COALESCE(diff->'employmentType'->>'to', '')
		 FROM activity
		 WHERE practice_id = $1 AND subject_kind = 'membership' AND subject_id = $2
		 ORDER BY created_at, action`,
		practiceID, staffID,
	)
	if err != nil {
		t.Fatalf("read membership events: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan membership event: %v", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate membership events: %v", err)
	}
	return out
}

// TestUpdateMembershipHandler_EditsBothHalvesAtOnce is RA-G2 (#261): the
// roles and the employment type move on one request, because they are
// edited on one form.
func TestUpdateMembershipHandler_EditsBothHalvesAtOnce(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-edits-membership"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)
	targetID := seedStaff(t, db, "target-membership")
	seedMembership(t, db, practiceID, targetID) // '{doula}', employee

	srv, session := newMembershipServer(t, db, ownerUID)
	defer srv.Close()

	resp := patchMembership(t, srv, session, practiceID, targetID, staffauth.UpdateMembershipRequest{
		Roles: []string{adminRole, doulaRole}, EmploymentType: contractorType,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var updated staffauth.UpdateMembershipResponse
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.EmploymentType != contractorType || len(updated.Roles) != 2 {
		t.Fatalf("response = %+v", updated)
	}

	var roles, employmentType string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT array_to_string(roles, ','), employment_type::text FROM practice_memberships
		 WHERE practice_id = $1 AND staff_id = $2`, practiceID, targetID,
	).Scan(&roles, &employmentType); err != nil {
		t.Fatalf("read membership: %v", err)
	}
	// Immediately, with no grandfathering: ADR-0008 gates ambient reach
	// on employment type, and a gate honouring the old answer is not one.
	if roles != "admin,doula" || employmentType != contractorType {
		t.Fatalf("membership = %q/%q", roles, employmentType)
	}

	events := membershipEvents(t, db, practiceID, targetID)
	// Both events share one created_at (one transaction), so the tie
	// breaks on action, alphabetically.
	want := []string{"employment_type_changed:->/employee->contractor", "roles_changed:doula->admin,doula/->"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}

	var actor string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT DISTINCT actor_staff_id FROM activity WHERE subject_kind = 'membership' AND subject_id = $1`, targetID,
	).Scan(&actor); err != nil {
		t.Fatalf("read actor: %v", err)
	}
	if actor != ownerID {
		t.Fatalf("actor = %q, want the Owner %q", actor, ownerID)
	}
}

// TestUpdateMembershipHandler_NoOpRecordsNothing keeps the history a list
// of changes rather than a list of times someone opened the form.
func TestUpdateMembershipHandler_NoOpRecordsNothing(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-edits-nothing"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	targetID := seedStaff(t, db, "unchanged-membership")
	seedMembership(t, db, practiceID, targetID) // '{doula}', employee

	srv, session := newMembershipServer(t, db, ownerUID)
	defer srv.Close()

	resp := patchMembership(t, srv, session, practiceID, targetID, staffauth.UpdateMembershipRequest{
		Roles: []string{doulaRole}, EmploymentType: employeeType,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if events := membershipEvents(t, db, practiceID, targetID); len(events) != 0 {
		t.Fatalf("events = %v, want none", events)
	}
}

// TestUpdateMembershipHandler_KeepsTheLastOwner covers the one edit an
// Owner may not make: nothing else in the API grants the role back, so a
// Practice with no Owner can never be invited to or edited again.
func TestUpdateMembershipHandler_KeepsTheLastOwner(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "sole-owner-demotes-herself"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session := newMembershipServer(t, db, ownerUID)
	defer srv.Close()

	resp := patchMembership(t, srv, session, practiceID, ownerID, staffauth.UpdateMembershipRequest{
		Roles: []string{doulaRole}, EmploymentType: employeeType,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	// With a second Owner in place, the same demotion is allowed.
	secondOwner := seedStaff(t, db, "second-owner")
	seedMembershipWithRoles(t, db, practiceID, secondOwner, "{owner}")

	allowed := patchMembership(t, srv, session, practiceID, ownerID, staffauth.UpdateMembershipRequest{
		Roles: []string{doulaRole}, EmploymentType: employeeType,
	})
	defer allowed.Body.Close()
	if allowed.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", allowed.StatusCode, http.StatusOK)
	}
}

func TestUpdateMembershipHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-edits-membership"
	staffID, practiceID := seedStaffWithMembership(t, db, doulaUID) // '{doula}'

	srv, session := newMembershipServer(t, db, doulaUID)
	defer srv.Close()

	resp := patchMembership(t, srv, session, practiceID, staffID, staffauth.UpdateMembershipRequest{
		Roles: []string{ownerRole}, EmploymentType: employeeType,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestUpdateMembershipHandler_NoSuchMembership(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-edits-a-stranger"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session := newMembershipServer(t, db, ownerUID)
	defer srv.Close()

	resp := patchMembership(t, srv, session, practiceID, emptyUUID, staffauth.UpdateMembershipRequest{
		Roles: []string{doulaRole}, EmploymentType: employeeType,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestUpdateMembershipHandler_Rejects(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-membership-validation"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session := newMembershipServer(t, db, ownerUID)
	defer srv.Close()

	cases := []struct {
		name string
		body staffauth.UpdateMembershipRequest
	}{
		{"no roles", staffauth.UpdateMembershipRequest{EmploymentType: employeeType}},
		{"unknown role", staffauth.UpdateMembershipRequest{Roles: []string{"superuser"}, EmploymentType: employeeType}},
		{"unknown employment type", staffauth.UpdateMembershipRequest{Roles: []string{doulaRole}, EmploymentType: "volunteer"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := patchMembership(t, srv, session, practiceID, ownerID, tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}

	t.Run("invalid body", func(t *testing.T) {
		resp := patchMembershipRaw(t, srv, session, practiceID, ownerID, []byte("not json"))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("malformed staff id", func(t *testing.T) {
		resp := patchMembership(t, srv, session, practiceID, "not-a-uuid", staffauth.UpdateMembershipRequest{
			Roles: []string{doulaRole}, EmploymentType: employeeType,
		})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})
}

// TestRemoveMembershipHandler_Success is #291's second criterion: a
// membership can be taken off a Practice by a route in the product, and
// who took it off is recorded.
func TestRemoveMembershipHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-removes-a-membership"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)
	targetID := seedStaff(t, db, "removable-membership")
	seedMembership(t, db, practiceID, targetID) // '{doula}', employee

	srv, session := newMembershipServer(t, db, ownerUID)
	defer srv.Close()

	resp := deleteMembership(t, srv, session, practiceID, targetID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var memberships int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, targetID,
	).Scan(&memberships); err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	if memberships != 0 {
		t.Fatalf("memberships = %d, want none", memberships)
	}

	// The staff row survives: a person is not owned by one Practice.
	var staffRows int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM staff WHERE id = $1`, targetID).Scan(&staffRows); err != nil {
		t.Fatalf("count staff: %v", err)
	}
	if staffRows != 1 {
		t.Fatalf("staff rows = %d, want the person to survive her membership", staffRows)
	}

	events := membershipEvents(t, db, practiceID, targetID)
	if len(events) != 1 || events[0] != "removed:doula->/employee->" {
		t.Fatalf("events = %v, want one removed event naming what she held", events)
	}
	var actor string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id FROM activity WHERE subject_kind = 'membership' AND subject_id = $1`, targetID,
	).Scan(&actor); err != nil {
		t.Fatalf("read actor: %v", err)
	}
	if actor != ownerID {
		t.Fatalf("actor = %q, want the Owner %q", actor, ownerID)
	}
}

func TestRemoveMembershipHandler_KeepsTheLastOwner(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "sole-owner-removes-herself"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session := newMembershipServer(t, db, ownerUID)
	defer srv.Close()

	resp := deleteMembership(t, srv, session, practiceID, ownerID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

func TestRemoveMembershipHandler_Refuses(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-removes-a-stranger"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session := newMembershipServer(t, db, ownerUID)
	defer srv.Close()

	t.Run("no such membership", func(t *testing.T) {
		resp := deleteMembership(t, srv, session, practiceID, emptyUUID)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("malformed staff id", func(t *testing.T) {
		resp := deleteMembership(t, srv, session, practiceID, "not-a-uuid")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})
}

func TestRemoveMembershipHandler_RequiresConfirmation(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-forgets-to-confirm"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	targetID := seedStaff(t, db, "unconfirmed-removal")
	seedMembership(t, db, practiceID, targetID)

	srv, session := newMembershipServer(t, db, ownerUID)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete,
		srv.URL+"/practices/"+practiceID+"/staff/"+targetID+"/membership", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var memberships int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, targetID,
	).Scan(&memberships); err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	if memberships != 1 {
		t.Fatalf("memberships = %d, want the unconfirmed request to have removed nothing", memberships)
	}
}

func TestRemoveMembershipHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-removes-a-membership"
	staffID, practiceID := seedStaffWithMembership(t, db, doulaUID) // '{doula}'

	srv, session := newMembershipServer(t, db, doulaUID)
	defer srv.Close()

	resp := deleteMembership(t, srv, session, practiceID, staffID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestUpdateMembershipHandler_ReorderedRolesAreNoChange keeps the history
// a record of changes: the order roles arrive in is the caller's, not a
// fact about the Membership.
func TestUpdateMembershipHandler_ReorderedRolesAreNoChange(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reorders-roles"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	targetID := seedStaff(t, db, "reordered-membership")
	seedMembershipWithRoles(t, db, practiceID, targetID, "{admin,doula}")

	srv, session := newMembershipServer(t, db, ownerUID)
	defer srv.Close()

	resp := patchMembership(t, srv, session, practiceID, targetID, staffauth.UpdateMembershipRequest{
		Roles: []string{doulaRole, adminRole}, EmploymentType: employeeType,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if events := membershipEvents(t, db, practiceID, targetID); len(events) != 0 {
		t.Fatalf("events = %v, want none", events)
	}
}
