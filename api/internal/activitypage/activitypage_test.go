package activitypage_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activitypage"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/testdb"
)

// TestMain terminates the shared Postgres container testdb.New starts for
// this test process -- see testdb.Main's doc comment.
func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

const (
	ownerRole    = "owner"
	doulaRole    = "doula"
	employeeType = "employee"
)

// probe is the row type these tests project into: the shared facts, plus
// the one extra column a diff-aware caller asks for. It stands in for
// engagement.ActivityEntry and staffauth.MembershipChange without
// dragging either package's DTO into this one.
type probe struct {
	id        string
	action    string
	actorKind string
	actorName string
	diff      string
}

// probeProjection exercises both halves of a Projection: an extra column
// (the diff), and the builder's access to the row's own id.
var probeProjection = activitypage.Projection[probe]{
	Columns: []string{"a.diff"},
	Row: func() ([]any, func(activitypage.Row) probe) {
		var diff []byte
		return []any{&diff}, func(r activitypage.Row) probe {
			return probe{id: r.ID, action: r.Action, actorKind: r.ActorKind, actorName: r.ActorName, diff: string(diff)}
		}
	},
}

// bareProjection asks for nothing beyond the shared columns -- the shape
// activityfeed.ListForSubject uses to answer a Client, and the one that
// proves a caller can decline the diff entirely.
var bareProjection = activitypage.Projection[string]{
	Row: func() ([]any, func(activitypage.Row) string) {
		return nil, func(r activitypage.Row) string { return r.Action }
	},
}

// beginScopedTx opens a tx with app.current_practice_id already set, the
// same scoping staffauth.Middleware performs per request: List takes a
// bare *sql.Tx and reads under whatever RLS scope its caller established.
func beginScopedTx(t *testing.T, db *testdb.DB, practiceID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}
	return tx
}

// TestList_CarriesTheCallersOwnDiffAndRowID is the whole of what #1150
// opened for: a caller that needs a diff names it in its own projection
// and gets it, alongside the row id its DTO exposes as an event id.
func TestList_CarriesTheCallersOwnDiffAndRowID(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Diff Carrying Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "activitypage-diff-owner", []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "activitypage-diff@example.com", "active")

	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectEngagement, engagementID, "visit_logged",
		activity.StaffActor(ownerID), json.RawMessage(`{"kind": "postpartum"}`))

	tx := beginScopedTx(t, db, practiceID)
	page, err := activitypage.List(t.Context(), tx, activitypage.Query{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		PageSize:    30,
	}, probeProjection)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("Items = %+v, want 1", page.Items)
	}
	if page.Items[0].diff != `{"kind": "postpartum"}` {
		t.Errorf("diff = %q, want the row's own diff", page.Items[0].diff)
	}
	if page.Items[0].id == "" {
		t.Error("id is empty, want the activity row's own id")
	}
}

// TestStatement_OmitsTheDiffForAProjectionThatDoesNotAskForIt is AC2 read
// at the level it actually holds: withholding a diff from a Client is not
// a check this code runs, it is the absence of a column in the statement
// her request issues.
func TestStatement_OmitsTheDiffForAProjectionThatDoesNotAskForIt(t *testing.T) {
	q := activitypage.Query{PracticeID: "p", SubjectKind: activity.SubjectEngagement, SubjectID: "e", PageSize: 30}

	bare, _ := activitypage.Statement(q, bareProjection)
	if strings.Contains(bare, "a.diff") {
		t.Errorf("a projection that asked for no diff still selects one:\n%s", bare)
	}

	withDiff, _ := activitypage.Statement(q, probeProjection)
	if !strings.Contains(withDiff, "a.diff") {
		t.Errorf("a projection that asked for the diff did not get one:\n%s", withDiff)
	}
}

// TestList_NamesEveryKindOfActorOnce is the bug #1150 found: one absence
// used to have three words and a blank across four hand-written readers.
// All four cases are answered here, in the one place that answers them.
func TestList_NamesEveryKindOfActorOnce(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Actor Naming Practice")
	ownerID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "activitypage-actor-owner", "Renata Alvarez", []string{ownerRole}, employeeType)
	departedID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "activitypage-actor-departed", "Marguerite Vandenberg", []string{doulaRole}, employeeType)
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Margaretha", "activitypage-actor-client@example.com")

	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "staff_acted", activity.StaffActor(ownerID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "departed_acted", activity.StaffActor(departedID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "client_acted", activity.ClientActor(clientID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "system_acted", activity.SystemActor())
	testdb.RemoveMembership(t, db, departedID)

	tx := beginScopedTx(t, db, practiceID)
	page, err := activitypage.List(t.Context(), tx, activitypage.Query{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		PageSize:    30,
	}, probeProjection)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := map[string]string{
		"staff_acted":    "Renata Alvarez",
		"departed_acted": activity.DepartedStaffName,
		"client_acted":   "Margaretha",
		"system_acted":   activity.SystemActorName,
	}
	got := map[string]string{}
	for _, item := range page.Items {
		got[item.action] = item.actorName
	}
	for action, name := range want {
		if got[action] != name {
			t.Errorf("%s actor = %q, want %q", action, got[action], name)
		}
	}
}

// TestList_PageSizeZeroReadsEveryRow is the Client history's own read:
// one screen of a woman's whole record, with no cursor to resume from.
func TestList_PageSizeZeroReadsEveryRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Unpaginated Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "activitypage-all-owner", []string{ownerRole}, employeeType)
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Persephone", "activitypage-all@example.com")

	const total = 35
	for i := range total {
		testdb.SeedActivity(t, db, practiceID, activity.SubjectClient, clientID, fmt.Sprintf("edited_%d", i), activity.StaffActor(ownerID))
	}

	tx := beginScopedTx(t, db, practiceID)
	page, err := activitypage.List(t.Context(), tx, activitypage.Query{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectClient,
		SubjectID:   clientID,
	}, bareProjection)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != total {
		t.Errorf("Items = %d, want every one of the %d rows", len(page.Items), total)
	}
	if page.HasMore || page.NextCursor != nil {
		t.Errorf("hasMore=%v cursor=%v, want an unpaginated read to claim neither", page.HasMore, page.NextCursor)
	}
}

// TestList_PagesNewestFirstPastAnExclusionAndAnExtraJoin walks the four
// parts of the shape a caller never writes again: an extra join, an
// action exclusion built from the caller's own constants, the sentinel
// row that reports more without a second query, and the cursor the next
// page resumes from.
func TestList_PagesNewestFirstPastAnExclusionAndAnExtraJoin(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Paging Practice")
	ownerID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "activitypage-paging-owner", "Renata Alvarez", []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Client", "activitypage-paging@example.com", "active")

	// One row the exclusion drops, then five it keeps.
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "contract_priced", activity.StaffActor(ownerID))
	const kept = 5
	for i := range kept {
		testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, fmt.Sprintf("visit_%d", i), activity.StaffActor(ownerID))
	}

	// An extra join on staff keyed by the actor, standing in for the two
	// engagement's own projection makes off the diff.
	joined := activitypage.Projection[string]{
		Joins:   []string{"LEFT JOIN staff actor_again ON actor_again.id = a.actor_staff_id"},
		Columns: []string{"actor_again.name"},
		Row: func() ([]any, func(activitypage.Row) string) {
			var name sql.NullString
			return []any{&name}, func(r activitypage.Row) string { return r.Action + "/" + name.String }
		},
	}

	q := activitypage.Query{
		PracticeID:      practiceID,
		SubjectKind:     activity.SubjectEngagement,
		SubjectID:       engagementID,
		ExcludedActions: "'contract_priced'",
		PageSize:        3,
	}

	tx := beginScopedTx(t, db, practiceID)
	first, err := activitypage.List(t.Context(), tx, q, joined)
	if err != nil {
		t.Fatalf("List first page: %v", err)
	}
	if len(first.Items) != 3 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 3/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}
	if first.Items[0] != "visit_4/Renata Alvarez" {
		t.Errorf("newest row = %q, want the last-written row with its joined name", first.Items[0])
	}

	cursor, err := pagecursor.Decode(*first.NextCursor)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	q.After = &cursor
	second, err := activitypage.List(t.Context(), tx, q, joined)
	if err != nil {
		t.Fatalf("List second page: %v", err)
	}
	if len(second.Items) != 2 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 2/false/nil -- the excluded row must not reappear",
			len(second.Items), second.HasMore, second.NextCursor)
	}
}

// TestList_RefusesAnotherPracticesRows is where visibility is enforced
// after #1150: the app-layer practice_id predicate this package always
// writes, underneath the RLS policy the caller's own transaction is
// scoped by. Asking for a subject in another Practice returns nothing
// even though the subject id is real.
func TestList_RefusesAnotherPracticesRows(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Reading Practice")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	otherOwnerID := testdb.SeedStaffAtPractice(t, db, otherPracticeID, "activitypage-other-owner", []string{ownerRole}, employeeType)
	_, otherEngagementID := testdb.SeedEngagementInStatus(t, db, otherPracticeID, "Client", "activitypage-other@example.com", "active")

	testdb.SeedActivity(t, db, otherPracticeID, activity.SubjectEngagement, otherEngagementID, "visit_logged", activity.StaffActor(otherOwnerID))

	tx := beginScopedTx(t, db, practiceID)
	page, err := activitypage.List(t.Context(), tx, activitypage.Query{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   otherEngagementID,
		PageSize:    30,
	}, bareProjection)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("Items = %+v, want none of another Practice's rows", page.Items)
	}
}
