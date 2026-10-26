package activityfeed_test

import (
	"encoding/json"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

// TestPracticeHandler_NamesTheClientEveryEngagementAndClientRowIsAbout is
// #2006: the practice-wide feed spans every Client the Practice holds, so
// "Contract sent" alone does not say whose. An Engagement's row and a
// Client record's own row both carry the Client's name. Since #2095 that
// is the full name (personname.Full): the conversation name -- the
// preferred name when there is one, the given name otherwise -- followed
// by the family name when there is one, so two Clients called Sarah read
// apart.
func TestPracticeHandler_NamesTheClientEveryEngagementAndClientRowIsAbout(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-client-subjects"
	practiceID := testdb.SeedPractice(t, db, "Feed Client Subjects Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	amaraID, amaraEngagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Amarachi", "feed-client-subjects-a@example.com", "active")
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE clients SET preferred_name = 'Amara', family_name = 'Okafor' WHERE id = $1`, amaraID); err != nil {
		t.Fatalf("set preferred name: %v", err)
	}
	// Simone has no family name: the given name alone, nothing trailing.
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
		activity.SubjectEngagement: "Amara Okafor",
		activity.SubjectClient:     "Simone",
		activity.SubjectPractice:   "",
	}
	for kind, name := range want {
		if bySubjectKind[kind] != name {
			t.Errorf("%s row SubjectName = %q, want %q (items %+v)", kind, bySubjectKind[kind], name, got.Items)
		}
	}
}

// TestPracticeHandler_TellsTwoClientsWithOneFirstNameApart is #2095: in a
// fourteen-doula agency two Clients can both be Sarah, and "Visit added
// for Sarah" twice does not say which. Each row carries the family name
// after the first name, and an Erasure (ADR-0027) still reaches every one
// of the erased Client's older rows, because the name is joined from the
// Client's row as it stands, never copied into the row at write time.
func TestPracticeHandler_TellsTwoClientsWithOneFirstNameApart(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-two-sarahs"
	practiceID := testdb.SeedPractice(t, db, "Feed Two Sarahs Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	leeID := testdb.SeedNamedClient(t, db, practiceID, "Sarah", "feed-two-sarahs-lee@example.com")
	okaforID := testdb.SeedNamedClient(t, db, practiceID, "Sarah", "feed-two-sarahs-okafor@example.com")
	erasedID := testdb.SeedNamedClient(t, db, practiceID, "Sarah", "feed-two-sarahs-erased@example.com")
	for id, family := range map[string]string{leeID: "Lee", okaforID: "Okafor", erasedID: "Whitfield"} {
		if _, err := db.Admin.ExecContext(t.Context(), `UPDATE clients SET family_name = $2 WHERE id = $1`, id, family); err != nil {
			t.Fatalf("set family name: %v", err)
		}
	}
	for _, id := range []string{leeID, okaforID, erasedID} {
		testdb.SeedActivity(t, db, practiceID, activity.SubjectClient, id, "created", activity.StaffActor(ownerID))
	}
	// The columns redactRecord empties, as it empties them.
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE clients SET given_name = $2, family_name = NULL, preferred_name = NULL, erased_at = now() WHERE id = $1`,
		erasedID, client.ErasedGivenName); err != nil {
		t.Fatalf("redact: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	bySubject := map[string]string{}
	for _, entry := range got.Items {
		bySubject[entry.SubjectID] = entry.SubjectName
	}
	want := map[string]string{leeID: "Sarah Lee", okaforID: "Sarah Okafor", erasedID: client.ErasedGivenName}
	for id, name := range want {
		if bySubject[id] != name {
			t.Errorf("row about %s SubjectName = %q, want %q (items %+v)", id, bySubject[id], name, got.Items)
		}
	}
}
