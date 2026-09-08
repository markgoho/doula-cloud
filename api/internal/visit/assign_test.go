package visit_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// createVisit POSTs a CreateRequest body, optionally under an
// Idempotency-Key so a replay can be asserted (#268's last acceptance
// criterion). key == "" sends no header at all, which is what every other
// test here does.
func createVisit(t *testing.T, session, url, key string, body visit.CreateRequest) *http.Response {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// decodeCreate reads a CreateResponse off a 201, failing the test on any
// other status -- the shape most of the success cases below want.
func decodeCreate(t *testing.T, resp *http.Response) visit.CreateResponse {
	t.Helper()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out visit.CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// countVisits is how the refusal cases prove no row was written.
func countVisits(t *testing.T, db *testdb.DB, engagementID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM visits WHERE engagement_id = $1`, engagementID,
	).Scan(&count); err != nil {
		t.Fatalf("count visits: %v", err)
	}
	return count
}

// assignedStaffID reads the assignee back off the newest activity row for
// action -- the audit-trail assertion, made through the stored diff rather
// than the handler's own response.
// key names which of the diff's own keys the caller is asking for: a
// creation carries one, `assignedStaffId`, and a reassignment carries
// both ends of the move under activity's before/after keys (#887).
func assignedStaffID(t *testing.T, db *testdb.DB, engagementID, action, key string) string {
	t.Helper()
	var diff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff FROM activity
		  WHERE subject_id = $1 AND action = $2 ORDER BY created_at DESC LIMIT 1`,
		engagementID, action,
	).Scan(&diff); err != nil {
		t.Fatalf("read activity entry: %v", err)
	}
	decoded := map[string]string{}
	if err := json.Unmarshal(diff, &decoded); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	return decoded[key]
}

// attachment reads the open attachment for a pair, or reports that there
// is none -- what the two grant assertions below turn on.
func attachment(t *testing.T, db *testdb.DB, engagementID, staffID string) (origin, attachedBy string, exists bool) {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT origin::text, attached_by::text FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NULL`,
		engagementID, staffID,
	)
	if err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return "", "", false
	}
	if err := rows.Scan(&origin, &attachedBy); err != nil {
		t.Fatalf("scan attachment: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	return origin, attachedBy, true
}

func visitsURL(srvURL, practiceID, engagementID string) string {
	return srvURL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/visits"
}

// The headline of #268: an Owner books a colleague onto a birth in one
// request, with no reassign step after it.
func TestCreateHandler_AssignsToTheNamedColleague(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-booking"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole, doulaRole}, "employee")
	targetStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-booked", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", visit.CreateRequest{StaffID: &targetStaffID})
	defer resp.Body.Close()
	if out := decodeCreate(t, resp); out.StaffID != targetStaffID {
		t.Fatalf("staffId = %q, want the named colleague %q", out.StaffID, targetStaffID)
	}
}

// The rule the Add-a-Visit picker leans on, guarded here so it cannot
// move under the screen (#909): a request that names nobody is a Visit
// for the caller. It is why a plain Doula is never asked the question at
// all, and therefore why a Doula who owns her Practice is offered her own
// name as a standing answer rather than an empty required field.
func TestCreateHandler_AnAbsentAssigneeIsTheCaller(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-logging-her-own"
	practiceID, callerStaffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole, doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", visit.CreateRequest{})
	defer resp.Body.Close()
	if out := decodeCreate(t, resp); out.StaffID != callerStaffID {
		t.Fatalf("staffId = %q, want the caller %q", out.StaffID, callerStaffID)
	}
}

// A plain Doula may log her own Visit and nobody else's: she cannot read
// the Staff roster at all (ADR-0008), so she has no colleague to pick.
func TestCreateHandler_RefusesAColleagueNamedByAPlainDoula(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-naming-someone-else"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	targetStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-named-by-peer", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", visit.CreateRequest{StaffID: &targetStaffID})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if got := countVisits(t, db, engagementID); got != 0 {
		t.Fatalf("visits = %d, want 0 -- a refused create must write no row", got)
	}
}

// Naming *herself* is the self path, so the same plain Doula still logs
// her own Visit through the new field without tripping the Owner/Admin
// rule.
func TestCreateHandler_APlainDoulaMayNameHerself(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-naming-herself"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", visit.CreateRequest{StaffID: &staffID})
	defer resp.Body.Close()
	if out := decodeCreate(t, resp); out.StaffID != staffID {
		t.Fatalf("staffId = %q, want her own %q", out.StaffID, staffID)
	}
}

// The Admin who is not a Doula, end to end: she names a colleague at
// create, she reassigns, and a create with no assignee refuses her rather
// than quietly putting her on a birth she is not attending.
func TestCreateHandler_AdminWhoIsNotADoulaNamesAColleague(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "admin-not-a-doula"
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	adminStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{adminRole}, "employee")
	firstStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-first", []string{doulaRole}, "employee")
	secondStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-second", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	url := visitsURL(srv.URL, practiceID, engagementID)

	created := createVisit(t, session, url, "", visit.CreateRequest{StaffID: &firstStaffID})
	defer created.Body.Close()
	out := decodeCreate(t, created)
	if out.StaffID != firstStaffID {
		t.Fatalf("staffId = %q, want %q", out.StaffID, firstStaffID)
	}

	body, err := json.Marshal(visit.ReassignRequest{StaffID: secondStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	reassigned := authedPatch(t, session, url+"/"+out.VisitID, body)
	defer reassigned.Body.Close()
	if reassigned.StatusCode != http.StatusOK {
		t.Fatalf("reassign status = %d, want %d", reassigned.StatusCode, http.StatusOK)
	}

	unnamed := createVisit(t, session, url, "", visit.CreateRequest{})
	defer unnamed.Body.Close()
	if unnamed.StatusCode != http.StatusForbidden {
		t.Fatalf("create with no assignee: status = %d, want %d", unnamed.StatusCode, http.StatusForbidden)
	}
	if _, _, exists := attachment(t, db, engagementID, adminStaffID); exists {
		t.Fatal("the Admin was attached to the Engagement; an Owner or Admin acting is never attached by it (ADR-0008)")
	}
}

// The three target rules, at create, with the messages the reassign path
// already returns. Table-driven because they are one rule read three
// ways, and because sharing the seed is what proves create is using the
// same helper rather than a looser copy.
func TestCreateHandler_RefusesAnIneligibleAssignee(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		target  func(t *testing.T, db *testdb.DB, practiceID, engagementID string) string
		wantMsg string
	}{
		{
			name: "not a staff member at this practice",
			target: func(t *testing.T, db *testdb.DB, _, _ string) string {
				t.Helper()
				otherPracticeID := testdb.SeedPractice(t, db, "Another Practice")
				return testdb.SeedStaffAtPractice(t, db, otherPracticeID, "doula-elsewhere-"+t.Name(), []string{doulaRole}, "employee")
			},
			wantMsg: "staff member not found at this practice",
		},
		{
			name: "membership carries no Doula role",
			target: func(t *testing.T, db *testdb.DB, practiceID, _ string) string {
				t.Helper()
				return testdb.SeedStaffAtPractice(t, db, practiceID, "admin-only-"+t.Name(), []string{adminRole}, "employee")
			},
			wantMsg: "staff member does not hold the Doula role at this practice",
		},
		{
			name: "contractor who has accepted no offer",
			target: func(t *testing.T, db *testdb.DB, practiceID, _ string) string {
				t.Helper()
				return testdb.SeedContractorAtPractice(t, db, practiceID, "contractor-unattached-"+t.Name())
			},
			wantMsg: "that contractor has not accepted an offer on this engagement",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			db := testdb.New(t)
			identityUID := "owner-naming-" + testCase.name
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole}, "employee")
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			targetStaffID := testCase.target(t, db, practiceID, engagementID)

			srv, session := newServer(t, db, identityUID)
			defer srv.Close()

			resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", visit.CreateRequest{StaffID: &targetStaffID})
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
			var out struct {
				Message string `json:"message"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if out.Message != testCase.wantMsg {
				t.Fatalf("message = %q, want %q", out.Message, testCase.wantMsg)
			}
			if got := countVisits(t, db, engagementID); got != 0 {
				t.Fatalf("visits = %d, want 0 -- a refused create must write no row", got)
			}
		})
	}
}

// A malformed assignee is a 400 about the id, not a 403 about the role --
// the caller typo'd, she was not refused.
func TestCreateHandler_InvalidAssignee(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-bad-assignee"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	badID := notAUUID
	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", visit.CreateRequest{StaffID: &badID})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// A contractor who already holds the granted attachment her own
// acceptance opened may be named -- and gets no second attachment out of
// it, because the one she agreed to is already open.
func TestCreateHandler_AllowsAnAttachedContractorAndGrantsHerNothing(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-naming-attached-contractor"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole}, "employee")
	targetStaffID := testdb.SeedContractorAtPractice(t, db, practiceID, "contractor-attached")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	testdb.SeedGrantedAttachment(t, db, engagementID, targetStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", visit.CreateRequest{StaffID: &targetStaffID})
	defer resp.Body.Close()
	if out := decodeCreate(t, resp); out.StaffID != targetStaffID {
		t.Fatalf("staffId = %q, want %q", out.StaffID, targetStaffID)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		engagementID, targetStaffID,
	).Scan(&count); err != nil {
		t.Fatalf("count attachments: %v", err)
	}
	if count != 1 {
		t.Fatalf("attachments = %d, want the one her acceptance opened and no more", count)
	}
}

// ADR-0008's "an Admin scheduling her onto a Visit ... granted, written
// explicitly": the named employee is attached, and attached_by is the
// person who did the naming, not the person named.
func TestCreateHandler_GrantsTheNamedEmployee(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "admin-granting-at-create"
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	adminStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{adminRole}, "employee")
	targetStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-named-employee", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", visit.CreateRequest{StaffID: &targetStaffID})
	defer resp.Body.Close()
	decodeCreate(t, resp)

	origin, attachedBy, exists := attachment(t, db, engagementID, targetStaffID)
	if !exists {
		t.Fatal("no attachment for the named employee")
	}
	if origin != grantedOrigin || attachedBy != adminStaffID {
		t.Fatalf("attachment = %s by %s, want granted by the person who named her", origin, attachedBy)
	}
}

// Both halves of the audit trail: who acted, who it went to, and when.
func TestVisitWrites_RecordTheAssignee(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-audited"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole}, "employee")
	firstStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-audited-first", []string{doulaRole}, "employee")
	secondStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-audited-second", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	url := visitsURL(srv.URL, practiceID, engagementID)

	created := createVisit(t, session, url, "", visit.CreateRequest{StaffID: &firstStaffID})
	defer created.Body.Close()
	out := decodeCreate(t, created)
	if got := assignedStaffID(t, db, engagementID, "visit_logged", "assignedStaffId"); got != firstStaffID {
		t.Fatalf("visit_logged assignee = %q, want %q", got, firstStaffID)
	}

	body, err := json.Marshal(visit.ReassignRequest{StaffID: secondStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	reassigned := authedPatch(t, session, url+"/"+out.VisitID, body)
	defer reassigned.Body.Close()
	if reassigned.StatusCode != http.StatusOK {
		t.Fatalf("reassign status = %d, want %d", reassigned.StatusCode, http.StatusOK)
	}
	if got := assignedStaffID(t, db, engagementID, "visit_reassigned", activity.DiffKeyAssignedStaffIDAfter); got != secondStaffID {
		t.Fatalf("visit_reassigned assignee = %q, want %q", got, secondStaffID)
	}
	if got := assignedStaffID(t, db, engagementID, "visit_reassigned", activity.DiffKeyAssignedStaffIDBefore); got != firstStaffID {
		t.Fatalf("visit_reassigned previous assignee = %q, want %q", got, firstStaffID)
	}
}

// The create route is Replayable (mount.go), and now carries a body: a
// retry under the same Idempotency-Key must answer with the same Visit
// and the same assignee, and must not book a second one.
func TestCreateHandler_ReplayReturnsTheSameAssignee(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-replaying"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole}, "employee")
	targetStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-replayed", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	url := visitsURL(srv.URL, practiceID, engagementID)

	first := createVisit(t, session, url, "book-once", visit.CreateRequest{StaffID: &targetStaffID})
	defer first.Body.Close()
	firstOut := decodeCreate(t, first)

	second := createVisit(t, session, url, "book-once", visit.CreateRequest{StaffID: &targetStaffID})
	defer second.Body.Close()
	secondOut := decodeCreate(t, second)

	if secondOut != firstOut {
		t.Fatalf("replay = %+v, want the stored %+v", secondOut, firstOut)
	}
	if got := countVisits(t, db, engagementID); got != 1 {
		t.Fatalf("visits = %d, want 1 -- a replay must not book a second", got)
	}
}

// The reassign half of the same rule, asserted by role alone: whether the
// screen drew the control or not, a plain Doula cannot hand a Visit to a
// colleague.
func TestReassignHandler_RefusesAColleagueNamedByAPlainDoula(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassigning-to-peer"
	practiceID, ownStaffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	targetStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-peer", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, ownStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, visitsURL(srv.URL, practiceID, engagementID)+"/"+visitID, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	var assignedTo string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT staff_id::text FROM visits WHERE id = $1`, visitID,
	).Scan(&assignedTo); err != nil {
		t.Fatalf("read visit: %v", err)
	}
	if assignedTo != ownStaffID {
		t.Fatalf("visit is assigned to %q, want the refusal to have left it on %q", assignedTo, ownStaffID)
	}
}

// She may still hand it back to herself -- the self path, the same one
// create takes with no assignee.
func TestReassignHandler_APlainDoulaMayNameHerself(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-reassigning-to-self"
	practiceID, ownStaffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	peerStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-holding-it", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, peerStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: ownStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, visitsURL(srv.URL, practiceID, engagementID)+"/"+visitID, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// The mirror of TestCreateHandler_GrantsTheNamedEmployee on the reassign
// side: the grant decision is one seam now (#914), and a seam is only
// proved shared by exercising it from both callers.
func TestReassignHandler_GrantsTheNamedEmployee(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "admin-granting-at-reassign"
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	adminStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{adminRole}, "employee")
	holderStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-holding-the-visit", []string{doulaRole}, "employee")
	targetStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-handed-the-visit", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, holderStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(visit.ReassignRequest{StaffID: targetStaffID})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPatch(t, session, visitsURL(srv.URL, practiceID, engagementID)+"/"+visitID, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	origin, attachedBy, exists := attachment(t, db, engagementID, targetStaffID)
	if !exists {
		t.Fatal("no attachment for the employee the Visit was handed to")
	}
	if origin != grantedOrigin || attachedBy != adminStaffID {
		t.Fatalf("attachment = %s by %s, want granted by the person who handed it over", origin, attachedBy)
	}
}
