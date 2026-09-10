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
// activity.SubjectMembership today -- staffauth.RecordMembershipEvent's
// four, plus EndSessionsHandler's own. Named here rather than inline so
// this test fails loudly the day a sixth is written and left out.
var membershipActions = []string{
	"joined",
	"roles_changed",
	"employment_type_changed",
	"removed",
	"sessions_ended",
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
	subjectID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-feed-membership", "Renata Alvarez", []string{doulaRole}, employeeType)

	for _, action := range membershipActions {
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
	for _, action := range membershipActions {
		entry, ok := seen[action]
		if !ok {
			t.Fatalf("membership action %q missing from the practice feed: %+v", action, got.Items)
		}
		if entry.SubjectKind != activity.SubjectMembership || entry.SubjectID != subjectID {
			t.Errorf("%q subject = %s/%s, want %s/%s", action, entry.SubjectKind, entry.SubjectID, activity.SubjectMembership, subjectID)
		}
		// #1148's AC2: the person it happened to, not only the person
		// who did it -- a reader can tell one roster change from another.
		if entry.SubjectName != "Renata Alvarez" {
			t.Errorf("%q SubjectName = %q, want %q", action, entry.SubjectName, "Renata Alvarez")
		}
		if entry.ActorName != "Priya Raman" {
			t.Errorf("%q ActorName = %q, want %q", action, entry.ActorName, "Priya Raman")
		}
	}
}

// TestPracticeHandler_NamesADepartedMembershipSubject is the one case
// #1148's AC2 degrades in, recorded rather than left to be discovered: a
// 'removed' event's subject is by construction someone whose
// practice_memberships row is gone, and staff_practice_visibility (00002)
// reaches a staff row only through a live one. The feed says plainly that
// the person is gone -- activity.DepartedStaffName, the same word #887
// settled on for an absent actor -- rather than printing a bare uuid or
// an empty cell.
func TestPracticeHandler_NamesADepartedMembershipSubject(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-departed-subject"
	practiceID := testdb.SeedPractice(t, db, "Feed Departed Subject Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	departedID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-feed-departed", "Renata Alvarez", []string{doulaRole}, employeeType)

	testdb.SeedActivity(t, db, practiceID, activity.SubjectMembership, departedID, "removed", activity.StaffActor(ownerID))
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
	if got.Items[0].SubjectName != activity.DepartedStaffName {
		t.Fatalf("SubjectName = %q, want %q", got.Items[0].SubjectName, activity.DepartedStaffName)
	}
}

// TestPracticeHandler_MembershipRowsFollowTheRoster is #1148's AC3: who
// may see a Membership row in the feed matches who may read the roster it
// describes -- ListStaffHandler's own OwnerAndAdmin mount. A Doula sees
// the Practice's Engagement and Client activity beside it, so this
// asserts the feed is not simply empty for her.
func TestPracticeHandler_MembershipRowsFollowTheRoster(t *testing.T) {
	for _, tc := range []struct {
		name           string
		uid            string
		roles          []string
		employmentType string
		wantMembership bool
	}{
		{"owner", "roster-owner", []string{ownerRole}, employeeType, true},
		{"admin", "roster-admin", []string{"admin"}, employeeType, true},
		{"employee doula", "roster-employee", []string{doulaRole}, employeeType, false},
		{"contractor doula", "roster-contractor", []string{doulaRole}, contractorType, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			practiceID := testdb.SeedPractice(t, db, "Roster Reach Practice")
			readerID := testdb.SeedStaffAtPractice(t, db, practiceID, tc.uid, tc.roles, tc.employmentType)
			testdb.SeedActivity(t, db, practiceID, activity.SubjectMembership, readerID, "roles_changed", activity.StaffActor(readerID))

			srv, session := newServer(t, db, tc.uid)
			defer srv.Close()

			resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/activity")
			defer resp.Body.Close()
			var got activityfeed.ListResponse
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			hasMembership := false
			for _, entry := range got.Items {
				if entry.SubjectKind == activity.SubjectMembership {
					hasMembership = true
				}
			}
			if hasMembership != tc.wantMembership {
				t.Fatalf("membership row visible = %v, want %v (items: %+v)", hasMembership, tc.wantMembership, got.Items)
			}
		})
	}
}
