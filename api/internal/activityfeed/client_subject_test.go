package activityfeed_test

import (
	"encoding/json"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/testdb"
)

// TestPracticeHandler_NamesTheClientEveryEngagementAndClientRowIsAbout is
// #2006: the practice-wide feed spans every Client the Practice holds, so
// "Contract sent" alone does not say whose. An Engagement's row and a
// Client record's own row both carry the Client's name, the conversation
// name every screen reads (personname.Preferred): the preferred name when
// there is one, the given name otherwise.
func TestPracticeHandler_NamesTheClientEveryEngagementAndClientRowIsAbout(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-client-subjects"
	practiceID := testdb.SeedPractice(t, db, "Feed Client Subjects Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	amaraID, amaraEngagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Amarachi", "feed-client-subjects-a@example.com", "active")
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE clients SET preferred_name = 'Amara' WHERE id = $1`, amaraID); err != nil {
		t.Fatalf("set preferred name: %v", err)
	}
	simoneID := testdb.SeedNamedClient(t, db, practiceID, "Simone", "feed-client-subjects-s@example.com")

	testdb.SeedActivity(t, db, practiceID, activity.SubjectClient, simoneID, "created", activity.StaffActor(ownerID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, amaraEngagementID, string(activity.ActionContractSent), activity.StaffActor(ownerID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectPractice, practiceID, string(activity.ActionPracticeNameChanged), activity.StaffActor(ownerID))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	bySubjectKind := map[string]string{}
	for _, entry := range got.Items {
		bySubjectKind[entry.SubjectKind] = entry.SubjectName
	}
	want := map[string]string{
		activity.SubjectEngagement: "Amara",
		activity.SubjectClient:     "Simone",
		activity.SubjectPractice:   "",
	}
	for kind, name := range want {
		if bySubjectKind[kind] != name {
			t.Errorf("%s row SubjectName = %q, want %q (items %+v)", kind, bySubjectKind[kind], name, got.Items)
		}
	}
}
