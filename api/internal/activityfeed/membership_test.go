package activityfeed_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/testdb"
)

// membershipActions is every action a write site records against
// activity.SubjectMembership, read from the write side's own vocabulary
// rather than hand-copied here -- so a sixth action added to that
// vocabulary is one this test demands of the feed the moment it exists,
// which a literal list of five could never do. activity's own
// TestMembershipActions_HoldsEveryConstant is what keeps the vocabulary
// itself honest.
func membershipActions() []string {
	actions := activity.MembershipActions()
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = string(a)
	}
	return out
}

// TestPracticeHandler_SurfacesEveryMembershipEvent is #1148's own AC1 and
// AC4: a Practice's feed of "what has happened here" says the roster
// changed, for every Membership action a write site records -- including
// sessions_ended, which names an act performed against a Membership
// rather than a change to what the Membership is, and which belongs on
// the same feed for the same reason (see activitygate's membership Rule).
func TestPracticeHandler_SurfacesEveryMembershipEvent(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-membership"
	practiceID := testdb.SeedPractice(t, db, "Feed Membership Practice")
	ownerID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, identityUID, "Priya Raman", []string{ownerRole}, employeeType)
	subjectID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-feed-membership", renataAlvarez, []string{doulaRole}, employeeType)

	for _, action := range membershipActions() {
		testdb.SeedActivity(t, db, practiceID, activity.SubjectMembership, subjectID, action, activity.StaffActor(ownerID))
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	seen := map[string]activityfeed.Entry{}
	for _, entry := range got.Items {
		seen[entry.Action] = entry
	}
	for _, action := range membershipActions() {
		entry, ok := seen[action]
		if !ok {
			t.Fatalf("membership action %q missing from the practice feed: %+v", action, got.Items)
		}
		if entry.SubjectKind != activity.SubjectMembership || entry.SubjectID != subjectID {
			t.Errorf("%q subject = %s/%s, want %s/%s", action, entry.SubjectKind, entry.SubjectID, activity.SubjectMembership, subjectID)
		}
		// #1148's AC2: the person it happened to, not only the person
		// who did it -- a reader can tell one roster change from another.
		if entry.SubjectName != renataAlvarez {
			t.Errorf("%q SubjectName = %q, want %q", action, entry.SubjectName, renataAlvarez)
		}
		if entry.ActorName != "Priya Raman" {
			t.Errorf("%q ActorName = %q, want %q", action, entry.ActorName, "Priya Raman")
		}
	}
}

// TestPracticeHandler_NamesADepartedMembershipSubject is #1256's own AC:
// a 'removed' event's subject is by construction someone whose
// practice_memberships row is gone, and staff_practice_visibility (00002)
// reaches a staff row only through a live one. 00116's
// staff_visible_to_own_practice_membership_history policy is the second
// door -- a plain departure (Membership removed, login intact) still
// resolves to her real name, because the activity table itself proves
// she was once a subject of a Membership event at this Practice, and her
// staff row is not redacted. A staff row that IS redacted (ADR-0033) is
// TestPracticeHandler_RedactedDepartedSubjectReadsAsAFormerColleague's
// case, not this one -- this test would have passed before 00116 too,
// which is why that second test exists.
func TestPracticeHandler_NamesADepartedMembershipSubject(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-departed-subject"
	practiceID := testdb.SeedPractice(t, db, "Feed Departed Subject Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	departedID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-feed-departed", renataAlvarez, []string{doulaRole}, employeeType)

	// The actor is the departed person herself, which is the shape
	// logindeletion's endEveryMembership writes: somebody deleting her own
	// login ends every Membership she holds, so both halves of the
	// sentence -- who it happened to, and who did it -- name the same
	// departed person.
	testdb.SeedActivity(t, db, practiceID, activity.SubjectMembership, departedID, string(activity.ActionMembershipRemoved), activity.StaffActor(departedID))
	testdb.RemoveMembership(t, db, departedID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("Items = %+v, want 1", got.Items)
	}
	if got.Items[0].SubjectName != renataAlvarez {
		t.Errorf("SubjectName = %q, want %q", got.Items[0].SubjectName, renataAlvarez)
	}
	// Both halves, not only the subject: the reader who removed her is
	// also the person who just left, and both columns now name her.
	if got.Items[0].ActorName != renataAlvarez {
		t.Errorf("ActorName = %q, want %q", got.Items[0].ActorName, renataAlvarez)
	}
}

// TestPracticeHandler_RedactedDepartedSubjectReadsAsAFormerColleague is
// #1256's AC4: 00116's policy excludes a redacted staff row
// (staff.deleted_at IS NOT NULL, ADR-0033) on purpose, so a Doula who
// deleted her own login still reads as activity.DepartedStaffName rather
// than handing the feed the "Deleted Staff Member" sentinel
// redactStaffRow writes -- an internal string that was never meant to
// reach a reader.
func TestPracticeHandler_RedactedDepartedSubjectReadsAsAFormerColleague(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-redacted-subject"
	practiceID := testdb.SeedPractice(t, db, "Feed Redacted Subject Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	departedID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-feed-redacted", renataAlvarez, []string{doulaRole}, employeeType)

	testdb.SeedActivity(t, db, practiceID, activity.SubjectMembership, departedID, string(activity.ActionMembershipRemoved), activity.StaffActor(departedID))
	testdb.RemoveMembership(t, db, departedID)
	testdb.RedactDeletedLogin(t, db, departedID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("Items = %+v, want 1", got.Items)
	}
	if got.Items[0].SubjectName != activity.DepartedStaffName {
		t.Errorf("SubjectName = %q, want %q", got.Items[0].SubjectName, activity.DepartedStaffName)
	}
	if got.Items[0].ActorName != activity.DepartedStaffName {
		t.Errorf("ActorName = %q, want %q", got.Items[0].ActorName, activity.DepartedStaffName)
	}
}

// TestPracticeHandler_MembershipRowsFollowTheRoster is #1148's AC3: who
// may see a Membership row in the feed matches who may read the roster it
// describes -- ListStaffHandler's own OwnerAndAdmin mount. Each case
// seeds an Engagement row beside the Membership one and asserts that it
// arrives, so a refusal reads as "she sees the Practice's activity and
// not the roster's" rather than as an empty feed, which would also pass
// if the gate refused every kind.
func TestPracticeHandler_MembershipRowsFollowTheRoster(t *testing.T) {
	for _, tc := range []struct {
		name           string
		uid            string
		roles          []string
		employmentType string
		wantMembership bool
	}{
		{"owner", "roster-owner", []string{ownerRole}, employeeType, true},
		{"admin", "roster-admin", []string{adminRole}, employeeType, true},
		{"employee doula", "roster-employee", []string{doulaRole}, employeeType, false},
		{"contractor doula", "roster-contractor", []string{doulaRole}, contractorType, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			practiceID := testdb.SeedPractice(t, db, "Roster Reach Practice")
			readerID := testdb.SeedStaffAtPractice(t, db, practiceID, tc.uid, tc.roles, tc.employmentType)
			testdb.SeedActivity(t, db, practiceID, activity.SubjectMembership, readerID, string(activity.ActionRolesChanged), activity.StaffActor(readerID))

			// An Engagement row beside it, so a refusal reads as "this
			// reader sees the Practice's activity and not the roster's"
			// rather than as an empty feed that would also pass if the
			// gate refused everything. A contractor reaches an Engagement
			// only through a granted attachment (ADR-0008), so she gets
			// one -- otherwise her feed really would be empty for a
			// reason that has nothing to do with this ticket.
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			if tc.employmentType == contractorType {
				testdb.SeedAttachment(t, db, engagementID, readerID, "granted", false)
			}
			testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID,
				string(activity.ActionVisitLogged), activity.StaffActor(readerID))

			srv, session := newServer(t, db, tc.uid)
			defer srv.Close()

			resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/activity")
			defer resp.Body.Close()
			var got activityfeed.ListResponse
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			hasMembership, hasEngagement := false, false
			for _, entry := range got.Items {
				switch entry.SubjectKind {
				case activity.SubjectMembership:
					hasMembership = true
				case activity.SubjectEngagement:
					hasEngagement = true
				}
			}
			if !hasEngagement {
				t.Fatalf("the Engagement row is missing too, so this case proves nothing about the roster (items: %+v)", got.Items)
			}
			if hasMembership != tc.wantMembership {
				t.Fatalf("membership row visible = %v, want %v (items: %+v)", hasMembership, tc.wantMembership, got.Items)
			}
		})
	}
}
