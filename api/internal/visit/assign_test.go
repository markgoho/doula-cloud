package visit_test

import (
	"encoding/json/v2"
	"net/http"
	"reflect"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// createVisit POSTs a CreateRequest body, optionally under an
// Idempotency-Key so a replay can be asserted (#268's last acceptance
// criterion). key == "" sends no header at all, which is what every other
// test here does.
func createVisit(t *testing.T, session, url, key string, body visit.CreateRequest) *http.Response {
	t.Helper()
	resp := reqtest.Post(t, url, reqtest.Session(session), reqtest.HeaderIf(key != "", "Idempotency-Key", key), reqtest.JSON(body))
	return resp
}

// decodeCreate reads a CreateResponse off a 201, failing the test on any
// other status -- the shape most of the success cases below want.
func decodeCreate(t *testing.T, resp *http.Response) visit.CreateResponse {
	t.Helper()
	reqtest.RequireStatus(t, resp, http.StatusCreated)
	out := reqtest.Decode[visit.CreateResponse](t, resp)
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
	decoded := map[string]any{}
	if err := json.Unmarshal(diff, &decoded); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	s, _ := decoded[key].(string)
	return s
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

// create books a Visit on the Engagement as the Staff member at the
// screen.
func (e *env) create(request visit.CreateRequest, opts ...reqtest.Option) visit.CreateResponse {
	e.T.Helper()
	return reqtest.Expect[visit.CreateResponse](e.T, http.MethodPost, e.visitsURL(""), http.StatusCreated, append([]reqtest.Option{e.SignedIn, reqtest.JSON(request)}, opts...)...)
}

// refuseCreate proves a create is refused with want and writes no row.
func (e *env) refuseCreate(request visit.CreateRequest, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodPost, e.visitsURL(""), want, e.SignedIn, reqtest.JSON(request))
	if got := countVisits(e.T, e.DB, e.Engagement.ID); got != 0 {
		e.T.Fatalf("visits = %d, want 0 -- a refused create must write no row", got)
	}
}

// reassign hands a Visit to staffID as the Staff member at the screen and
// requires want.
func (e *env) reassign(visitID, staffID string, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodPatch, e.visitsURL("/"+visitID), want, e.SignedIn, reqtest.JSON(visit.ReassignRequest{StaffID: staffID}))
}

// requireAttachment proves staffID holds an open granted Attachment on the
// Engagement, attached by attachedBy.
func (e *env) requireAttachment(staffID, attachedBy, who string) {
	e.T.Helper()
	origin, got, exists := attachment(e.T, e.DB, e.Engagement.ID, staffID)
	if !exists {
		e.T.Fatalf("no attachment for %s", who)
	}
	if origin != grantedOrigin || got != attachedBy {
		e.T.Fatalf("attachment = %s by %s, want granted by %s", origin, got, attachedBy)
	}
}

// The headline of #268: an Owner books a colleague onto a birth in one
// request, with no reassign step after it. The rule the Add-a-Visit picker
// leans on is guarded here too so it cannot move under the screen (#909):
// a request that names nobody is a Visit for the caller, which is why a
// plain Doula is never asked the question at all and a Doula who owns her
// Practice is offered her own name as a standing answer rather than an
// empty required field. Naming *herself* is the self path, so a plain
// Doula still logs her own Visit through the field without tripping the
// Owner/Admin rule.
func TestCreateHandler_AssignsTheVisit(t *testing.T) {
	cases := []struct {
		name   string
		caller []fixture.StaffOption
		// request names who the Visit is for; it returns the request and
		// the Staff member the Visit should land on.
		request func(e *env) (visit.CreateRequest, string)
	}{
		{"the named colleague", []fixture.StaffOption{fixture.Owner(), fixture.Doula()}, func(e *env) (visit.CreateRequest, string) {
			colleague := e.Colleague().ID
			return visit.CreateRequest{StaffID: &colleague}, colleague
		}},
		{"the caller when nobody is named", []fixture.StaffOption{fixture.Owner(), fixture.Doula()}, func(e *env) (visit.CreateRequest, string) {
			return visit.CreateRequest{}, e.Staff.ID
		}},
		{"a plain Doula who names herself", []fixture.StaffOption{fixture.Doula()}, func(e *env) (visit.CreateRequest, string) {
			return visit.CreateRequest{StaffID: &e.Staff.ID}, e.Staff.ID
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.caller...))
			request, want := tc.request(e)

			if out := e.create(request); out.StaffID != want {
				t.Fatalf("staffId = %q, want %q", out.StaffID, want)
			}
		})
	}
}

// A plain Doula may log her own Visit and nobody else's: she cannot read
// the Staff roster at all (ADR-0008), so she has no colleague to pick.
func TestCreateHandler_RefusesAColleagueNamedByAPlainDoula(t *testing.T) {
	e := newEnv(t)
	colleague := e.Colleague().ID

	e.refuseCreate(visit.CreateRequest{StaffID: &colleague}, http.StatusForbidden)
}

// The Admin who is not a Doula, end to end: she names a colleague at
// create, she reassigns, and a create with no assignee refuses her rather
// than quietly putting her on a birth she is not attending.
func TestCreateHandler_AdminWhoIsNotADoulaNamesAColleague(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Roles(fixture.RoleAdmin)))
	first, second := e.Colleague().ID, e.Colleague().ID

	out := e.create(visit.CreateRequest{StaffID: &first})
	if out.StaffID != first {
		t.Fatalf("staffId = %q, want %q", out.StaffID, first)
	}

	e.reassign(out.VisitID, second, http.StatusOK)

	reqtest.ExpectStatus(t, http.MethodPost, e.visitsURL(""), http.StatusForbidden, e.SignedIn, reqtest.JSON(visit.CreateRequest{}))
	if _, _, exists := attachment(t, e.DB, e.Engagement.ID, e.Staff.ID); exists {
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
		target  func(e *env) string
		wantMsg string
	}{
		{
			name: "not a staff member at this practice",
			target: func(e *env) string {
				return fixture.Staff(e.T, e.DB, fixture.Practice(e.T, e.DB, fixture.PracticeName("Another Practice"))).ID
			},
			wantMsg: "staff member not found at this practice",
		},
		{
			name:    "membership carries no Doula role",
			target:  func(e *env) string { return e.Colleague(fixture.Roles(fixture.RoleAdmin)).ID },
			wantMsg: "staff member does not hold the Doula role at this practice",
		},
		{
			name:    "contractor who has accepted no offer",
			target:  func(e *env) string { return e.Colleague(fixture.Contractor()).ID },
			wantMsg: "that contractor has not accepted an offer on this work",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, signedInAs(fixture.Owner()))
			target := testCase.target(e)

			got := apierrtest.Expect(t, http.MethodPost, e.visitsURL(""), http.StatusBadRequest, e.SignedIn, reqtest.JSON(visit.CreateRequest{StaffID: &target}))

			if got.Message != testCase.wantMsg {
				t.Fatalf("message = %q, want %q", got.Message, testCase.wantMsg)
			}
			if got := countVisits(t, e.DB, e.Engagement.ID); got != 0 {
				t.Fatalf("visits = %d, want 0 -- a refused create must write no row", got)
			}
		})
	}
}

// A malformed assignee is a 400 about the id, not a 403 about the role --
// the caller typo'd, she was not refused.
func TestCreateHandler_InvalidAssignee(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	badID := notAUUID

	e.refuseCreate(visit.CreateRequest{StaffID: &badID}, http.StatusBadRequest)
}

// A contractor who already holds the granted attachment her own
// acceptance opened may be named -- and gets no second attachment out of
// it, because the one she agreed to is already open.
func TestCreateHandler_AllowsAnAttachedContractorAndGrantsHerNothing(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	target := e.Colleague(fixture.Contractor()).ID
	testdb.SeedGrantedAttachment(t, e.DB, e.Engagement.ID, target)

	if out := e.create(visit.CreateRequest{StaffID: &target}); out.StaffID != target {
		t.Fatalf("staffId = %q, want %q", out.StaffID, target)
	}

	count := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`, e.Engagement.ID, target)
	if count != 1 {
		t.Fatalf("attachments = %d, want the one her acceptance opened and no more", count)
	}
}

// ADR-0008's "an Admin scheduling her onto a Visit ... granted, written
// explicitly": the named employee is attached, and attached_by is the
// person who did the naming, not the person named. The grant decision is
// one seam (#914), and a seam is only proved shared by exercising it from
// both callers, create and reassign.
func TestVisitWrites_GrantTheNamedEmployee(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Roles(fixture.RoleAdmin)))
		target := e.Colleague().ID

		e.create(visit.CreateRequest{StaffID: &target})

		e.requireAttachment(target, e.Staff.ID, "the person who named her")
	})
	t.Run("reassign", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Roles(fixture.RoleAdmin)))
		visitID := seedVisit(t, e.DB, e.Engagement.ID, e.Colleague().ID)
		target := e.Colleague().ID

		e.reassign(visitID, target, http.StatusOK)

		e.requireAttachment(target, e.Staff.ID, "the person who handed it over")
	})
}

// #1625: the Visit is one of the writers of a granted Attachment that
// needs no Offer, and it reads the one rule the others read. A Doula who
// holds Owner or Admin is attached by a Visit under each Employment
// type: where she logs it for herself she is attached_by herself, and
// where an Owner or an Admin names her the person who named her is.
// "Only her own acceptance of an Offer" is the rule for a contractor who
// holds neither role, and TestCreateHandler_RefusesAnIneligibleAssignee
// and TestCreateHandler_GrantsNothingToAContractorWhoLoggedAVisit still
// hold her to it.
func TestCreateHandler_GrantsAnOwnerOrAdminWhoIsAContractorDoula(t *testing.T) {
	roles := []struct {
		name string
		role fixture.StaffOption
	}{{ownerRole, fixture.Owner()}, {adminRole, fixture.Admin()}}
	for _, r := range roles {
		roleName, role := r.name, r.role
		t.Run(roleName+" logs a Visit for herself", func(t *testing.T) {
			e := newEnv(t, signedInAs(role, fixture.Doula(), fixture.Contractor()))

			if out := e.create(visit.CreateRequest{}); out.StaffID != e.Staff.ID {
				t.Fatalf("staffId = %q, want her own %q", out.StaffID, e.Staff.ID)
			}

			e.requireAttachment(e.Staff.ID, e.Staff.ID, "herself")
		})

		t.Run(roleName+" is named by an Owner", func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()))
			target := e.Colleague(role, fixture.Doula(), fixture.Contractor()).ID

			if out := e.create(visit.CreateRequest{StaffID: &target}); out.StaffID != target {
				t.Fatalf("staffId = %q, want %q", out.StaffID, target)
			}

			e.requireAttachment(target, e.Staff.ID, "the person who named her")
		})
	}
}

// The reassign path is the same act at a second moment, so it gives the
// same answer: an Admin who is a contractor Doula is handed a Visit with
// no Offer, and is attached by the person who handed it to her.
func TestReassignHandler_GrantsAnAdminWhoIsAContractorDoula(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula()))
	target := e.Colleague(fixture.Admin(), fixture.Doula(), fixture.Contractor()).ID
	out := e.create(visit.CreateRequest{})

	e.reassign(out.VisitID, target, http.StatusOK)

	e.requireAttachment(target, e.Staff.ID, "the person who handed her the Visit")
}

// Both halves of the audit trail: who acted, who it went to, and when.
func TestVisitWrites_RecordTheAssignee(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	first, second := e.Colleague().ID, e.Colleague().ID

	out := e.create(visit.CreateRequest{StaffID: &first})
	if got := assignedStaffID(t, e.DB, e.Engagement.ID, "visit_logged", "assignedStaffId"); got != first {
		t.Fatalf("visit_logged assignee = %q, want %q", got, first)
	}

	e.reassign(out.VisitID, second, http.StatusOK)
	if got := assignedStaffID(t, e.DB, e.Engagement.ID, "visit_reassigned", activity.DiffKeyAssignedStaffIDAfter); got != second {
		t.Fatalf("visit_reassigned assignee = %q, want %q", got, second)
	}
	if got := assignedStaffID(t, e.DB, e.Engagement.ID, "visit_reassigned", activity.DiffKeyAssignedStaffIDBefore); got != first {
		t.Fatalf("visit_reassigned previous assignee = %q, want %q", got, first)
	}
}

// The create route is Replayable (mount.go), and now carries a body: a
// retry under the same Idempotency-Key must answer with the same Visit
// and the same assignee, and must not book a second one.
func TestCreateHandler_ReplayReturnsTheSameAssignee(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	target := e.Colleague().ID
	key := reqtest.Header("Idempotency-Key", "book-once")

	firstOut := e.create(visit.CreateRequest{StaffID: &target}, key)
	secondOut := e.create(visit.CreateRequest{StaffID: &target}, key)

	if !reflect.DeepEqual(secondOut, firstOut) {
		t.Fatalf("replay = %+v, want the stored %+v", secondOut, firstOut)
	}
	if got := countVisits(t, e.DB, e.Engagement.ID); got != 1 {
		t.Fatalf("visits = %d, want 1 -- a replay must not book a second", got)
	}
}

// The reassign half of the rule a plain Doula may name only herself,
// asserted by role alone: whether the screen drew the control or not, a
// plain Doula cannot hand a Visit to a colleague, and the refusal leaves
// it where it was. She may still hand one back to herself -- the self
// path, the same one create takes with no assignee.
func TestReassignHandler_APlainDoulaMayNameOnlyHerself(t *testing.T) {
	e := newEnv(t)
	peer := e.Colleague().ID
	visitID := seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)

	e.reassign(visitID, peer, http.StatusForbidden)
	assignedTo := testdb.Scalar[string](t, e.DB.Admin, `SELECT staff_id::text FROM visits WHERE id = $1`, visitID)
	if assignedTo != e.Staff.ID {
		t.Fatalf("visit is assigned to %q, want the refusal to have left it on %q", assignedTo, e.Staff.ID)
	}

	peerVisit := seedVisit(t, e.DB, e.Engagement.ID, peer)
	e.reassign(peerVisit, e.Staff.ID, http.StatusOK)
}
