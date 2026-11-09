package staffauth_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// patch edits staffID's Membership and requires want.
func (e *env) patch(staffID string, body staffauth.UpdateMembershipRequest, want int) staffauth.UpdateMembershipResponse {
	e.T.Helper()
	return reqtest.Expect[staffauth.UpdateMembershipResponse](e.T, http.MethodPatch, e.membershipURL(staffID), want, e.SignedIn, reqtest.JSON(body))
}

// remove takes staffID's Membership off the Practice and requires want.
func (e *env) remove(staffID string, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodDelete, e.membershipURL(staffID), want, e.SignedIn, confirmed)
}

// membershipEvents reads a Membership's recorded history, newest first.
func membershipEvents(t *testing.T, db *testdb.DB, practiceID, staffID string) []string {
	t.Helper()
	out := testdb.Column[string](t, db.Admin, `SELECT action || ':' || COALESCE(diff->'roles'->>'from', '') ||
		        '->' || COALESCE(diff->'roles'->>'to', '') ||
		        '/' || COALESCE(diff->'employmentType'->>'from', '') ||
		        '->' || COALESCE(diff->'employmentType'->>'to', '')
		 FROM activity
		 WHERE practice_id = $1 AND subject_kind = 'membership' AND subject_id = $2
		 ORDER BY created_at, action`,
		practiceID, staffID)
	return out
}

func asDoula() staffauth.UpdateMembershipRequest {
	return staffauth.UpdateMembershipRequest{Roles: []string{doulaRole}, EmploymentType: employeeType}
}

// TestUpdateMembershipHandler_EditsBothHalvesAtOnce is RA-G2 (#261): the
// roles and the employment type move on one request, because they are
// edited on one form.
func TestUpdateMembershipHandler_EditsBothHalvesAtOnce(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	target := e.Colleague()

	updated := e.patch(target.ID, staffauth.UpdateMembershipRequest{
		Roles: []string{adminRole, doulaRole}, EmploymentType: contractorType,
	}, http.StatusOK)
	if updated.EmploymentType != contractorType || len(updated.Roles) != 2 {
		t.Fatalf("response = %+v", updated)
	}

	var roles, employmentType string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT array_to_string(roles, ','), employment_type::text FROM practice_memberships
		 WHERE practice_id = $1 AND staff_id = $2`, e.Practice.ID, target.ID,
	).Scan(&roles, &employmentType); err != nil {
		t.Fatalf("read membership: %v", err)
	}
	// Immediately, with no grandfathering: ADR-0008 gates ambient reach
	// on employment type, and a gate honoring the old answer is not one.
	if roles != "admin,doula" || employmentType != contractorType {
		t.Fatalf("membership = %q/%q", roles, employmentType)
	}

	events := membershipEvents(t, e.DB, e.Practice.ID, target.ID)
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

	actor := testdb.Scalar[string](t, e.DB.Admin, `SELECT DISTINCT actor_staff_id FROM activity WHERE subject_kind = 'membership' AND subject_id = $1`, target.ID)
	if actor != e.Staff.ID {
		t.Fatalf("actor = %q, want the Owner %q", actor, e.Staff.ID)
	}
}

// TestUpdateMembershipHandler_NoChangeRecordsNothing keeps the history a
// list of changes rather than a list of times someone opened the form --
// and a record of changes: the order roles arrive in is the caller's, not
// a fact about the Membership.
func TestUpdateMembershipHandler_NoChangeRecordsNothing(t *testing.T) {
	cases := []struct {
		name   string
		target fixture.StaffOption
		body   staffauth.UpdateMembershipRequest
	}{
		{"the same answer", fixture.Doula(), asDoula()},
		{"the same roles in another order", fixture.Roles(adminRole, doulaRole), staffauth.UpdateMembershipRequest{
			Roles: []string{doulaRole, adminRole}, EmploymentType: employeeType,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()))
			target := e.Colleague(tc.target)

			e.patch(target.ID, tc.body, http.StatusOK)

			if events := membershipEvents(t, e.DB, e.Practice.ID, target.ID); len(events) != 0 {
				t.Fatalf("events = %v, want none", events)
			}
		})
	}
}

// TestUpdateMembershipHandler_KeepsTheLastOwner covers the one edit an
// Owner may not make: nothing else in the API grants the role back, so a
// Practice with no Owner can never be invited to or edited again.
func TestUpdateMembershipHandler_KeepsTheLastOwner(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	e.patch(e.Staff.ID, asDoula(), http.StatusConflict)

	// With a second Owner in place, the same demotion is allowed.
	e.Colleague(fixture.Owner())
	e.patch(e.Staff.ID, asDoula(), http.StatusOK)
}

func TestUpdateMembershipHandler_NonOwnerForbidden(t *testing.T) {
	e := newEnv(t)

	e.patch(e.Staff.ID, staffauth.UpdateMembershipRequest{Roles: []string{ownerRole}, EmploymentType: employeeType}, http.StatusForbidden)
}

func TestUpdateMembershipHandler_Rejects(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	cases := []struct {
		name    string
		staffID string
		body    staffauth.UpdateMembershipRequest
	}{
		{"no roles", e.Staff.ID, staffauth.UpdateMembershipRequest{EmploymentType: employeeType}},
		{"unknown role", e.Staff.ID, staffauth.UpdateMembershipRequest{Roles: []string{"superuser"}, EmploymentType: employeeType}},
		{"unknown employment type", e.Staff.ID, staffauth.UpdateMembershipRequest{Roles: []string{doulaRole}, EmploymentType: "volunteer"}},
		{"malformed staff id", notAUUID, asDoula()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(*testing.T) {
			e.patch(tc.staffID, tc.body, http.StatusBadRequest)
		})
	}

	t.Run("invalid body", func(t *testing.T) {
		reqtest.ExpectStatus(t, http.MethodPatch, e.membershipURL(e.Staff.ID), http.StatusBadRequest, e.SignedIn, reqtest.RawJSON([]byte("not json")))
	})
}

func TestUpdateMembershipHandler_NoSuchMembership(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	e.patch(emptyUUID, asDoula(), http.StatusNotFound)
}

// TestRemoveMembershipHandler_Success is #291's second criterion: a
// membership can be taken off a Practice by a route in the product, and
// who took it off is recorded.
func TestRemoveMembershipHandler_Success(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	target := e.Colleague()

	e.remove(target.ID, http.StatusNoContent)

	memberships := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`, e.Practice.ID, target.ID)
	if memberships != 0 {
		t.Fatalf("memberships = %d, want none", memberships)
	}

	// The staff row survives: a person is not owned by one Practice.
	staffRows := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM staff WHERE id = $1`, target.ID)
	if staffRows != 1 {
		t.Fatalf("staff rows = %d, want the person to survive her membership", staffRows)
	}

	events := membershipEvents(t, e.DB, e.Practice.ID, target.ID)
	if len(events) != 1 || events[0] != "removed:doula->/employee->" {
		t.Fatalf("events = %v, want one removed event naming what she held", events)
	}
	actor := testdb.Scalar[string](t, e.DB.Admin, `SELECT actor_staff_id FROM activity WHERE subject_kind = 'membership' AND subject_id = $1`, target.ID)
	if actor != e.Staff.ID {
		t.Fatalf("actor = %q, want the Owner %q", actor, e.Staff.ID)
	}
}

func TestRemoveMembershipHandler_KeepsTheLastOwner(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	e.remove(e.Staff.ID, http.StatusConflict)
}

func TestRemoveMembershipHandler_Refuses(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	t.Run("no such membership", func(*testing.T) {
		e.remove(emptyUUID, http.StatusNotFound)
	})

	t.Run("malformed staff id", func(*testing.T) {
		e.remove(notAUUID, http.StatusBadRequest)
	})
}

func TestRemoveMembershipHandler_RequiresConfirmation(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	target := e.Colleague()

	reqtest.ExpectStatus(t, http.MethodDelete, e.membershipURL(target.ID), http.StatusBadRequest, e.SignedIn)

	memberships := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`, e.Practice.ID, target.ID)
	if memberships != 1 {
		t.Fatalf("memberships = %d, want the unconfirmed request to have removed nothing", memberships)
	}
}

func TestRemoveMembershipHandler_NonOwnerForbidden(t *testing.T) {
	e := newEnv(t)

	e.remove(e.Staff.ID, http.StatusForbidden)
}
