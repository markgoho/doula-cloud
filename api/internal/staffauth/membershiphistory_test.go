package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// newMembershipHistoryServer mounts this package's whole surface through
// staffauth.Mount, the same call main.go makes on the real GatedRouter --
// because the Owner/Admin-vs-Doula boundary lives at that mount and not
// inside the handler (#315).
func newMembershipHistoryServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getMembershipHistory(t *testing.T, srv *httptest.Server, session, practiceID, staffID, query string) *http.Response {
	t.Helper()
	url := srv.URL + "/api/practices/" + practiceID + "/staff/" + staffID + "/membership-history" + query
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
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

func decodeMembershipHistory(t *testing.T, resp *http.Response) staffauth.MembershipHistory {
	t.Helper()
	var out staffauth.MembershipHistory
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
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

// TestListMembershipHistory_DoulaForbidden holds #872's reach criterion
// at the boundary that enforces it: the history reads through the same
// gate as the roster it hangs off, and a Doula has neither.
func TestListMembershipHistory_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reading-membership-history"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

	srv, session := newMembershipHistoryServer(t, db, identityUID)
	defer srv.Close()

	resp := getMembershipHistory(t, srv, session, practiceID, staffID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestListMembershipHistory_ReadsTheWholeTrail is the ticket's core
// case, and the one every acceptance criterion about content rests on:
// the three kinds of event a person's standing at a Practice produces --
// she joined, her roles moved, her employment type moved -- come back
// newest first, each naming the person who did it and each carrying only
// the fact it actually changed.
func TestListMembershipHistory_ReadsTheWholeTrail(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reads-membership-history"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := testdb.SeedStaff(t, db, "doula-who-was-promoted")
	seedMembership(t, db, practiceID, doulaID)

	joined := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)
	promoted := time.Date(2026, 8, 14, 15, 30, 0, 0, time.UTC)
	reclassified := time.Date(2027, 1, 6, 11, 0, 0, 0, time.UTC)
	seedMembershipEvent(t, db, practiceID, doulaID, "joined", "", "doula", "", "employee", doulaID, joined)
	seedMembershipEvent(t, db, practiceID, doulaID, "roles_changed", "doula", "admin,doula", "", "", ownerID, promoted)
	seedMembershipEvent(t, db, practiceID, doulaID, "employment_type_changed", "", "", "employee", "contractor", ownerID, reclassified)

	srv, session := newMembershipHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getMembershipHistory(t, srv, session, practiceID, doulaID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	history := decodeMembershipHistory(t, resp)
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
	if newest.ActorName != "Test Staff "+ownerUID {
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
	db := testdb.New(t)
	const ownerUID = "founding-owner-reads-her-own-row"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	seedMembershipEvent(t, db, practiceID, ownerID, "joined",
		"", "owner,admin,doula", "", employeeType, ownerID,
		time.Date(2026, 1, 20, 8, 0, 0, 0, time.UTC))

	srv, session := newMembershipHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getMembershipHistory(t, srv, session, practiceID, ownerID, "")
	defer resp.Body.Close()

	history := decodeMembershipHistory(t, resp)
	if len(history.Items) != 1 {
		t.Fatalf("items = %+v, want the founding Owner's own joined row", history.Items)
	}
	entry := history.Items[0]
	if entry.ActorName != "Test Staff "+ownerUID {
		t.Errorf("actorName = %q, want the founding Owner herself", entry.ActorName)
	}
	if len(entry.Roles) != 3 {
		t.Errorf("roles = %v, want all three she founded the Practice with", entry.Roles)
	}
}

// TestListMembershipHistory_SessionsEndedCarriesNoFacts pins the one
// action on this subject kind that changes nothing about what a
// Membership is (#473): its diff is a bare "{}", and the entry has to
// come back naming who ended the sessions and when, with all four
// before/after fields absent rather than blank.
func TestListMembershipHistory_SessionsEndedCarriesNoFacts(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reads-an-ended-session"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := testdb.SeedStaff(t, db, "doula-signed-out-everywhere")
	seedMembership(t, db, practiceID, doulaID)
	seedMembershipActivityRow(t, db, practiceID, doulaID, "sessions_ended",
		[]byte("{}"), "staff", ownerID, time.Date(2027, 2, 2, 12, 0, 0, 0, time.UTC))

	srv, session := newMembershipHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getMembershipHistory(t, srv, session, practiceID, doulaID, "")
	defer resp.Body.Close()

	history := decodeMembershipHistory(t, resp)
	if len(history.Items) != 1 {
		t.Fatalf("items = %+v, want the sessions_ended row", history.Items)
	}
	entry := history.Items[0]
	if entry.Action != "sessions_ended" {
		t.Errorf("action = %q, want sessions_ended", entry.Action)
	}
	if entry.PreviousRoles != nil || entry.Roles != nil ||
		entry.PreviousEmploymentType != "" || entry.EmploymentType != "" {
		t.Errorf("entry = %+v, want no before/after fact at all", entry)
	}
	if entry.ActorName != "Test Staff "+ownerUID {
		t.Errorf("actorName = %q, want the Owner who ended the sessions", entry.ActorName)
	}
}

// TestListMembershipHistory_DepartedActorIsNamedAsGone covers the reason
// the actor is resolved through a LEFT JOIN: staff_practice_visibility
// (00002) admits a staff row only while that person holds a Membership
// here, so the Owner who made somebody an Admin and has since left is
// unreachable by name. The entry still has to render, and a bare uuid
// would tell the reader nothing.
func TestListMembershipHistory_DepartedActorIsNamedAsGone(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reads-a-departed-colleagues-act"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	departedID := testdb.SeedStaff(t, db, "owner-who-has-since-left")
	doulaID := testdb.SeedStaff(t, db, "doula-promoted-by-a-leaver")
	seedMembership(t, db, practiceID, doulaID)

	// The departed Owner never gets a Membership here, which is exactly
	// the state she is in after RemoveMembershipHandler deletes it.
	seedMembershipEvent(t, db, practiceID, doulaID, "roles_changed",
		doulaRole, "admin,doula", "", "", departedID,
		time.Date(2026, 9, 9, 9, 0, 0, 0, time.UTC))

	srv, session := newMembershipHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getMembershipHistory(t, srv, session, practiceID, doulaID, "")
	defer resp.Body.Close()

	history := decodeMembershipHistory(t, resp)
	if len(history.Items) != 1 {
		t.Fatalf("items = %+v, want the promotion row", history.Items)
	}
	if history.Items[0].ActorName != activity.DepartedStaffName {
		t.Errorf("actorName = %q, want %q", history.Items[0].ActorName, activity.DepartedStaffName)
	}
}

// TestListMembershipHistory_SystemActorIsNamedDoulaCloud holds
// ADR-0022's naming rule for the actor kind this subject kind has no
// writer for yet: if one is ever added, the row reads "Doula Cloud",
// never "System" and never a blank name.
func TestListMembershipHistory_SystemActorIsNamedDoulaCloud(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reads-a-system-written-row"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := testdb.SeedStaff(t, db, "doula-changed-by-the-product")
	seedMembership(t, db, practiceID, doulaID)
	seedMembershipActivityRow(t, db, practiceID, doulaID, "roles_changed",
		[]byte(`{"roles":{"from":"doula","to":"admin,doula"},"employmentType":{"from":"","to":""}}`),
		"system", "", time.Date(2027, 4, 1, 10, 0, 0, 0, time.UTC))

	srv, session := newMembershipHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getMembershipHistory(t, srv, session, practiceID, doulaID, "")
	defer resp.Body.Close()

	history := decodeMembershipHistory(t, resp)
	if len(history.Items) != 1 {
		t.Fatalf("items = %+v, want the system-written row", history.Items)
	}
	if history.Items[0].ActorName != activity.SystemActorName {
		t.Errorf("actorName = %q, want %q", history.Items[0].ActorName, activity.SystemActorName)
	}
}

// TestListMembershipHistory_UnreadableDiffStillNamesTheEvent pins what
// happens to a row whose diff is valid jsonb but not this type's shape.
// No writer can produce one, but the whole page must not fail over it:
// the entry still carries what happened, who did it and when, which is
// most of what a reader came for, and only the before/after facts go
// missing.
func TestListMembershipHistory_UnreadableDiffStillNamesTheEvent(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reads-an-unreadable-diff"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := testdb.SeedStaff(t, db, "doula-with-a-strange-row")
	seedMembership(t, db, practiceID, doulaID)
	seedMembershipActivityRow(t, db, practiceID, doulaID, "roles_changed",
		[]byte(`{"roles": 5}`), "staff", ownerID, time.Date(2027, 5, 5, 10, 0, 0, 0, time.UTC))

	srv, session := newMembershipHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getMembershipHistory(t, srv, session, practiceID, doulaID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d -- one unreadable diff must not fail the page", resp.StatusCode, http.StatusOK)
	}

	history := decodeMembershipHistory(t, resp)
	if len(history.Items) != 1 {
		t.Fatalf("items = %+v, want the row to survive", history.Items)
	}
	entry := history.Items[0]
	if entry.Action != "roles_changed" || entry.ActorName != "Test Staff "+ownerUID {
		t.Errorf("entry = %+v, want the action and actor intact", entry)
	}
	if entry.PreviousRoles != nil || entry.Roles != nil {
		t.Errorf("entry roles = %v -> %v, want both absent", entry.PreviousRoles, entry.Roles)
	}
}

// TestListMembershipHistory_OtherPracticesStaffIsNotFound is the AC
// about who may read this, at the row rather than the role: a staff id
// that names a real person at somebody else's Practice must be a 404,
// not an empty history, which would confirm the person exists.
func TestListMembershipHistory_OtherPracticesStaffIsNotFound(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-probing-another-practice"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	otherPracticeID, elsewhereID := testdb.SeedStaffAtNewPractice(t, db,
		"doula-at-another-practice", []string{doulaRole}, employeeType)
	if otherPracticeID == practiceID {
		t.Fatal("fixture seeded both staff at the same Practice")
	}
	seedMembershipEvent(t, db, otherPracticeID, elsewhereID, "joined",
		"", doulaRole, "", employeeType, elsewhereID,
		time.Date(2026, 5, 5, 5, 0, 0, 0, time.UTC))

	srv, session := newMembershipHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getMembershipHistory(t, srv, session, practiceID, elsewhereID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestListMembershipHistory_PaginatesOldestPagesLast is the AC about a
// long-lived Membership: the screen asks for one page and the rest stays
// reachable through the cursor rather than being cut off.
func TestListMembershipHistory_PaginatesOldestPagesLast(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-pages-membership-history"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := testdb.SeedStaff(t, db, "doula-reclassified-often")
	seedMembership(t, db, practiceID, doulaID)

	// 25 alternating employment-type changes, so the page boundary falls
	// inside them.
	const total = 25
	types := [2]string{employeeType, "contractor"}
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range total {
		seedMembershipEvent(t, db, practiceID, doulaID, "employment_type_changed",
			"", "", types[i%2], types[(i+1)%2], ownerID, start.AddDate(0, i, 0))
	}

	srv, session := newMembershipHistoryServer(t, db, ownerUID)
	defer srv.Close()

	first := getMembershipHistory(t, srv, session, practiceID, doulaID, "")
	defer first.Body.Close()
	page1 := decodeMembershipHistory(t, first)
	if len(page1.Items) != 20 {
		t.Fatalf("first page items = %d, want 20", len(page1.Items))
	}
	if !page1.HasMore || page1.NextCursor == nil {
		t.Fatalf("first page hasMore = %v, nextCursor = %v, want more", page1.HasMore, page1.NextCursor)
	}

	second := getMembershipHistory(t, srv, session, practiceID, doulaID, "?cursor="+*page1.NextCursor)
	defer second.Body.Close()
	page2 := decodeMembershipHistory(t, second)
	if len(page2.Items) != total-20 {
		t.Fatalf("second page items = %d, want %d", len(page2.Items), total-20)
	}
	if page2.HasMore {
		t.Error("second page hasMore = true, want false")
	}
	if page2.Items[0].CreatedAt.After(page1.Items[19].CreatedAt) {
		t.Error("second page starts newer than the first page ended")
	}
}

func TestListMembershipHistory_RejectsBadCursor(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-sends-a-bad-membership-cursor"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session := newMembershipHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getMembershipHistory(t, srv, session, practiceID, ownerID, "?cursor=not-a-cursor")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestListMembershipHistory_RejectsBadStaffID(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-sends-a-bad-membership-staff-id"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session := newMembershipHistoryServer(t, db, ownerUID)
	defer srv.Close()

	resp := getMembershipHistory(t, srv, session, practiceID, "not-a-uuid", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
