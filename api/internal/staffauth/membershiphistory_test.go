package staffauth_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// history reads staffID's Membership history as the Staff member at the
// screen.
func (e *env) history(staffID, query string) staffauth.MembershipHistory {
	e.T.Helper()
	return reqtest.Expect[staffauth.MembershipHistory](e.T, http.MethodGet, e.staffRoute(staffID, "membership-history"+query), http.StatusOK, e.SignedIn)
}

// seedMembershipEvent writes one row of the audit trail directly, in the
// exact shape staffauth.RecordMembershipEvent writes it, so a test can
// lay out years of a Membership's history without driving five different
// endpoints to produce it. Pass the empty string for a side that did not
// move, which is what a real event carries there.
func seedMembershipEvent(t *testing.T, db *testdb.DB, practiceID, staffID, action string,
	rolesFrom, rolesTo, employmentFrom, employmentTo, actorStaffID string, at time.Time) {
	t.Helper()
	diff, err := json.Marshal(map[string]map[string]string{
		"roles":          {"from": rolesFrom, "to": rolesTo},
		"employmentType": {"from": employmentFrom, "to": employmentTo},
	})
	if err != nil {
		// coverage:ignore reason: a map of plain strings always marshals cleanly
		t.Fatalf("marshal diff: %v", err)
	}
	seedMembershipActivityRow(t, db, practiceID, staffID, action, diff, "staff", actorStaffID, at)
}

// seedMembershipActivityRow is the raw form behind seedMembershipEvent,
// for the two rows that are not a roles/employment change: the
// sessions_ended row #473 writes with a bare "{}" diff, and the
// non-staff actor kind ADR-0022 allows on the table but no Membership
// write site produces yet.
func seedMembershipActivityRow(t *testing.T, db *testdb.DB, practiceID, staffID, action string,
	diff []byte, actorKind, actorStaffID string, at time.Time) {
	t.Helper()
	var actor any
	if actorStaffID != "" {
		actor = actorStaffID
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6::activity_actor_kind, $7, $8)`,
		practiceID, activity.SubjectMembership, staffID, action, diff, actorKind, actor, at,
	); err != nil {
		t.Fatalf("seed membership activity row: %v", err)
	}
}

// TestListMembershipHistory_ReadsTheWholeTrail is the ticket's core
// case, and the one every acceptance criterion about content rests on:
// the three kinds of event a person's standing at a Practice produces --
// she joined, her roles moved, her employment type moved -- come back
// newest first, each naming the person who did it and each carrying only
// the fact it actually changed.
func TestListMembershipHistory_ReadsTheWholeTrail(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	doula := e.Colleague()

	joined := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)
	promoted := time.Date(2026, 8, 14, 15, 30, 0, 0, time.UTC)
	reclassified := time.Date(2027, 1, 6, 11, 0, 0, 0, time.UTC)
	seedMembershipEvent(t, e.DB, e.Practice.ID, doula.ID, "joined", "", "doula", "", "employee", doula.ID, joined)
	seedMembershipEvent(t, e.DB, e.Practice.ID, doula.ID, "roles_changed", "doula", "admin,doula", "", "", e.Staff.ID, promoted)
	seedMembershipEvent(t, e.DB, e.Practice.ID, doula.ID, "employment_type_changed", "", "", "employee", "contractor", e.Staff.ID, reclassified)

	history := e.history(doula.ID, "")
	if len(history.Items) != 3 {
		t.Fatalf("items = %+v, want 3", history.Items)
	}

	// Newest first: the employment-type change, carrying both employment
	// values and neither role list.
	newest := history.Items[0]
	if newest.Action != "employment_type_changed" {
		t.Errorf("newest action = %q, want employment_type_changed", newest.Action)
	}
	if newest.PreviousEmploymentType != employeeType || newest.EmploymentType != "contractor" {
		t.Errorf("newest employment = %q -> %q, want employee -> contractor",
			newest.PreviousEmploymentType, newest.EmploymentType)
	}
	if newest.PreviousRoles != nil || newest.Roles != nil {
		t.Errorf("newest roles = %v -> %v, want both absent on an employment-type change",
			newest.PreviousRoles, newest.Roles)
	}
	if newest.ActorName != e.Staff.Name {
		t.Errorf("newest actorName = %q, want the Owner who made the change", newest.ActorName)
	}

	// The promotion, carrying both role lists as arrays -- never the CSV
	// the diff column stores -- and neither employment value.
	middle := history.Items[1]
	if middle.Action != "roles_changed" {
		t.Errorf("middle action = %q, want roles_changed", middle.Action)
	}
	if len(middle.PreviousRoles) != 1 || middle.PreviousRoles[0] != doulaRole {
		t.Errorf("middle previousRoles = %v, want [doula]", middle.PreviousRoles)
	}
	if len(middle.Roles) != 2 || middle.Roles[0] != "admin" || middle.Roles[1] != doulaRole {
		t.Errorf("middle roles = %v, want [admin doula]", middle.Roles)
	}
	if middle.PreviousEmploymentType != "" || middle.EmploymentType != "" {
		t.Errorf("middle employment = %q -> %q, want both absent on a roles change",
			middle.PreviousEmploymentType, middle.EmploymentType)
	}

	// The joining event: what she arrived as, with no before.
	oldest := history.Items[2]
	if oldest.Action != "joined" {
		t.Errorf("oldest action = %q, want joined", oldest.Action)
	}
	if oldest.PreviousRoles != nil {
		t.Errorf("oldest previousRoles = %v, want absent -- a Membership has no before", oldest.PreviousRoles)
	}
	if len(oldest.Roles) != 1 || oldest.Roles[0] != doulaRole {
		t.Errorf("oldest roles = %v, want [doula]", oldest.Roles)
	}
	if oldest.EmploymentType != employeeType {
		t.Errorf("oldest employmentType = %q, want employee", oldest.EmploymentType)
	}
	if history.HasMore || history.NextCursor != nil {
		t.Errorf("hasMore = %v, nextCursor = %v, want a single complete page", history.HasMore, history.NextCursor)
	}
}

// TestListMembershipHistory_FoundingOwnerIsHerOwnActor is #872's
// third AC and #316's rule read back. The founding Owner's Membership
// gets the same 'joined' record everybody else's does, and she is named
// as her own actor -- so "how did this person come to hold these roles?"
// has an answer for the person who created the Practice too, and not
// only for the people she later invited.
func TestListMembershipHistory_FoundingOwnerIsHerOwnActor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	seedMembershipEvent(t, e.DB, e.Practice.ID, e.Staff.ID, "joined",
		"", "owner,admin,doula", "", employeeType, e.Staff.ID,
		time.Date(2026, 1, 20, 8, 0, 0, 0, time.UTC))

	history := e.history(e.Staff.ID, "")
	if len(history.Items) != 1 {
		t.Fatalf("items = %+v, want the founding Owner's own joined row", history.Items)
	}
	entry := history.Items[0]
	if entry.ActorName != e.Staff.Name {
		t.Errorf("actorName = %q, want the founding Owner herself", entry.ActorName)
	}
	if len(entry.Roles) != 3 {
		t.Errorf("roles = %v, want all three she founded the Practice with", entry.Roles)
	}
}

// TestListMembershipHistory_OneRowEntries covers the rows whose shape is
// the point, each the only row on a colleague's trail: sessions_ended
// (#473), whose diff is a bare "{}" and which has to come back naming who
// ended the sessions with all four before/after fields absent rather than
// blank; an actor who has since left, resolved through a LEFT JOIN
// because staff_practice_visibility (00002) admits a staff row only while
// that person holds a Membership here, and a bare uuid would tell the
// reader nothing; the actor kind the table allows but no Membership write
// produces yet, which ADR-0022 says reads "DoulaCloud", never "System"
// and never a blank; and a diff that is valid jsonb but not this type's
// shape, which no writer can produce but must not fail the page.
func TestListMembershipHistory_OneRowEntries(t *testing.T) {
	cases := []struct {
		name string
		seed func(e *env, doula fixture.StaffRecord)
		want func(t *testing.T, e *env, entry staffauth.MembershipChange)
	}{
		{"sessions ended carries no facts", func(e *env, doula fixture.StaffRecord) {
			seedMembershipActivityRow(e.T, e.DB, e.Practice.ID, doula.ID, "sessions_ended",
				[]byte("{}"), "staff", e.Staff.ID, time.Date(2027, 2, 2, 12, 0, 0, 0, time.UTC))
		}, func(t *testing.T, e *env, entry staffauth.MembershipChange) {
			if entry.Action != "sessions_ended" {
				t.Errorf("action = %q, want sessions_ended", entry.Action)
			}
			if entry.PreviousRoles != nil || entry.Roles != nil ||
				entry.PreviousEmploymentType != "" || entry.EmploymentType != "" {
				t.Errorf("entry = %+v, want no before/after fact at all", entry)
			}
			if entry.ActorName != e.Staff.Name {
				t.Errorf("actorName = %q, want the Owner who ended the sessions", entry.ActorName)
			}
		}},
		{"a departed actor is named as gone", func(e *env, doula fixture.StaffRecord) {
			// The departed Owner never gets a Membership here, which is
			// exactly the state she is in after RemoveMembershipHandler
			// deletes it.
			departed := testdb.SeedStaff(e.T, e.DB, "owner-who-has-since-left")
			seedMembershipEvent(e.T, e.DB, e.Practice.ID, doula.ID, "roles_changed",
				doulaRole, "admin,doula", "", "", departed,
				time.Date(2026, 9, 9, 9, 0, 0, 0, time.UTC))
		}, func(t *testing.T, _ *env, entry staffauth.MembershipChange) {
			if entry.ActorName != activity.DepartedStaffName {
				t.Errorf("actorName = %q, want %q", entry.ActorName, activity.DepartedStaffName)
			}
		}},
		{"a system actor is named DoulaCloud", func(e *env, doula fixture.StaffRecord) {
			seedMembershipActivityRow(e.T, e.DB, e.Practice.ID, doula.ID, "roles_changed",
				[]byte(`{"roles":{"from":"doula","to":"admin,doula"},"employmentType":{"from":"","to":""}}`),
				"system", "", time.Date(2027, 4, 1, 10, 0, 0, 0, time.UTC))
		}, func(t *testing.T, _ *env, entry staffauth.MembershipChange) {
			if entry.ActorName != activity.SystemActorName {
				t.Errorf("actorName = %q, want %q", entry.ActorName, activity.SystemActorName)
			}
		}},
		{"an unreadable diff still names the event", func(e *env, doula fixture.StaffRecord) {
			seedMembershipActivityRow(e.T, e.DB, e.Practice.ID, doula.ID, "roles_changed",
				[]byte(`{"roles": 5}`), "staff", e.Staff.ID, time.Date(2027, 5, 5, 10, 0, 0, 0, time.UTC))
		}, func(t *testing.T, e *env, entry staffauth.MembershipChange) {
			if entry.Action != "roles_changed" || entry.ActorName != e.Staff.Name {
				t.Errorf("entry = %+v, want the action and actor intact", entry)
			}
			if entry.PreviousRoles != nil || entry.Roles != nil {
				t.Errorf("entry roles = %v -> %v, want both absent", entry.PreviousRoles, entry.Roles)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()))
			doula := e.Colleague()
			tc.seed(e, doula)

			history := e.history(doula.ID, "")
			if len(history.Items) != 1 {
				t.Fatalf("items = %+v, want the one row", history.Items)
			}
			tc.want(t, e, history.Items[0])
		})
	}
}
