package staffauth_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	workStateUID  = "work-state-uid"
	workStateName = "Nadia"
)

// readWorkState returns the work state and the number of audit events for
// the staff row with the given identity, read as the Admin role so the
// assertion is about what was written rather than about what RLS lets a
// particular caller see.
func readWorkState(t *testing.T, db *testdb.DB, identityUID string) (string, int) {
	t.Helper()
	var state string
	var events int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT s.work_state, (SELECT count(*) FROM staff_work_state_events e WHERE e.staff_id = s.id)
		   FROM staff s WHERE s.identity_uid = $1`,
		identityUID,
	).Scan(&state, &events); err != nil {
		t.Fatalf("read work state: %v", err)
	}
	return state, events
}

func TestSignup_RejectsMissingWorkState(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: workStateUID}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		PracticeName: "No State Practice", StaffName: workStateName, StaffEmail: testStaffEmail,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// A two-letter string that is not a state is refused by the handler rather
// than by the CHECK constraint: the caller gets a 400 naming the field, not
// the 500 an unhandled constraint violation would produce.
func TestSignup_RejectsUnknownWorkState(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: workStateUID}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		PracticeName: "Bad State Practice", StaffName: workStateName, StaffEmail: testStaffEmail,
		WorkState: "ZZ",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// The form offers a fixed <select>, but a form is not an enforcement
// boundary: whatever arrives is trimmed and upper-cased before it is
// stored, so the apportionment query never has to match case.
func TestSignup_NormalizesWorkStateAndRecordsFirstEvent(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: workStateUID}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		PracticeName: "Lower Case Practice", StaffName: workStateName, StaffEmail: testStaffEmail,
		WorkState: "  nj ",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	state, events := readWorkState(t, db, workStateUID)
	if state != "NJ" {
		t.Fatalf("work_state = %q, want %q", state, "NJ")
	}
	// The founding Owner gets the same first assertion anyone else does.
	if events != 1 {
		t.Fatalf("work state events = %d, want 1", events)
	}
}

// The Offer path a contractor arrives by is the Invitation path: under
// ADR-0008 an Offer rides on an Invitation, so accepting one is what
// creates her staff row -- and her work state with it.
func TestAcceptInvite_RecordsWorkStateForANewPerson(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-inviting-a-contractor")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "rosa@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))

	srv := newAcceptServer(t, db, "rosa-uid", "rosa@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{
		InviteToken: token, Name: "Rosa Mendel", WorkState: "pa",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	state, events := readWorkState(t, db, "rosa-uid")
	if state != "PA" {
		t.Fatalf("work_state = %q, want %q", state, "PA")
	}
	if events != 1 {
		t.Fatalf("work state events = %d, want 1", events)
	}
}

func TestAcceptInvite_RejectsUnknownWorkStateForANewPerson(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-inviting-a-bad-state")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "zed@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))

	srv := newAcceptServer(t, db, "zed-uid", "zed@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{
		InviteToken: token, Name: "Zed Nowhere", WorkState: "Ontario",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// A contractor doula joining her second Practice keeps the work state she
// already asserted, and this acceptance records no new assertion: she did
// not make one. The second Practice inherits the value -- which the roster
// prints as self-reported, with its date, so nobody has to wonder where it
// came from.
func TestAcceptInvite_IgnoresWorkStateForAnExistingPerson(t *testing.T) {
	db := testdb.New(t)

	signupSrv := newSignupServer(authntest.Verifier{UID: "roaming-uid"}, db)
	defer signupSrv.Close()
	first := postSignup(t, signupSrv, "tok", staffauth.SignupRequest{
		PracticeName: "Her First Practice", StaffName: "Nell Ward",
		StaffEmail: "nell@example.com", WorkState: "NY",
	})
	_ = first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("signup status = %d, want %d", first.StatusCode, http.StatusCreated)
	}

	ownerID, practiceID := seedOwnerMembership(t, db, "owner-of-the-second-practice")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "nell@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))

	acceptSrv := newAcceptServer(t, db, "roaming-uid", "nell@example.com")
	defer acceptSrv.Close()

	resp := postAccept(t, acceptSrv, staffauth.AcceptInviteRequest{
		InviteToken: token, Name: "Nell Ward", WorkState: "TX",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	state, events := readWorkState(t, db, "roaming-uid")
	if state != "NY" {
		t.Fatalf("work_state = %q, want %q -- the second Practice must not overwrite it", state, "NY")
	}
	if events != 1 {
		t.Fatalf("work state events = %d, want 1 -- acceptance recorded an assertion she did not make", events)
	}
}

// The roster is where "how did this get set?" is answered: the value and
// the date she asserted it, for an Owner who never entered either.
func TestRoster_CarriesWorkStateAndItsDate(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "roster-work-state-uid"}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		PracticeName: "Roster Practice", StaffName: "Ada Frost",
		StaffEmail: "ada@example.com", WorkState: "CT",
	})
	defer resp.Body.Close()
	var created staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode signup: %v", err)
	}

	listSrv, sess := newStaffListServer(t, db, "roster-work-state-uid")
	defer listSrv.Close()
	listResp := getStaffList(t, listSrv, sess, created.PracticeID)
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("roster status = %d, want %d", listResp.StatusCode, http.StatusOK)
	}
	var roster staffauth.Roster
	if err := json.NewDecoder(listResp.Body).Decode(&roster); err != nil {
		t.Fatalf("decode roster: %v", err)
	}
	if len(roster.Members) != 1 {
		t.Fatalf("members = %d, want 1", len(roster.Members))
	}
	if got := roster.Members[0].WorkState; got != "CT" {
		t.Fatalf("workState = %q, want %q", got, "CT")
	}
	if roster.Members[0].WorkStateReportedAt.IsZero() {
		t.Fatal("workStateReportedAt is zero -- the roster cannot say how old the value is")
	}
}

// --- Correcting a work state (#437) -------------------------------------

// newWorkStateServer mounts the correction route and seeds a live session
// for uid, returning the token its __session cookie carries. The route
// carries no {practiceId} and no {staffId}: that absence is the whole
// authorization story, so the mount here mirrors main.go exactly.
func newWorkStateServer(t *testing.T, db *testdb.DB, uid string) (*httptest.Server, string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("PUT /staff/work-state", staffauth.UpdateWorkStateHandler(db.App))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func putWorkState(t *testing.T, srv *httptest.Server, session, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/staff/work-state", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if session != "" {
		authntest.AddSessionCookie(req, session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// latestWorkStateEvent returns both sides of the newest audit row for a
// person, plus who was recorded as having made the change.
func latestWorkStateEvent(t *testing.T, db *testdb.DB, identityUID string) (previous, current, actor string) {
	t.Helper()
	var prev sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT e.previous_work_state, e.work_state, e.actor_staff_id
		   FROM staff_work_state_events e
		   JOIN staff s ON s.id = e.staff_id
		  WHERE s.identity_uid = $1
		  ORDER BY e.created_at DESC, e.id DESC
		  LIMIT 1`,
		identityUID,
	).Scan(&prev, &current, &actor); err != nil {
		t.Fatalf("read latest work state event: %v", err)
	}
	return prev.String, current, actor
}

// reportedAt reads the stored assertion date, so a test can prove it moved
// rather than trusting the response body to have told the truth.
func reportedAt(t *testing.T, db *testdb.DB, identityUID string) time.Time {
	t.Helper()
	var at time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT work_state_reported_at FROM staff WHERE identity_uid = $1`, identityUID,
	).Scan(&at); err != nil {
		t.Fatalf("read work_state_reported_at: %v", err)
	}
	return at
}

func TestUpdateWorkState_MissingCookie(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newWorkStateServer(t, db, "no-cookie-correcting")
	defer srv.Close()

	resp := putWorkState(t, srv, "", `{"workState":"NJ"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// A live session whose identity resolves to no staff row: the shape a
// person who abandoned an invitation halfway arrives in.
func TestUpdateWorkState_UnknownStaff(t *testing.T) {
	db := testdb.New(t)
	srv, session := newWorkStateServer(t, db, "session-without-a-staff-row")
	defer srv.Close()

	resp := putWorkState(t, srv, session, `{"workState":"NJ"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestUpdateWorkState_RejectsMalformedBody(t *testing.T) {
	db := testdb.New(t)
	seedStaff(t, db, "malformed-body-uid")
	srv, session := newWorkStateServer(t, db, "malformed-body-uid")
	defer srv.Close()

	resp := putWorkState(t, srv, session, `not json`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// Refused by the handler rather than by 00043's CHECK constraint, so the
// caller gets a 400 naming the field instead of the 500 an unhandled
// constraint violation would produce -- and in the same words the two
// onboarding paths use.
func TestUpdateWorkState_RejectsSomethingThatIsNotAState(t *testing.T) {
	db := testdb.New(t)
	seedStaff(t, db, "not-a-state-uid")
	srv, session := newWorkStateServer(t, db, "not-a-state-uid")
	defer srv.Close()

	resp := putWorkState(t, srv, session, `{"workState":"Ontario"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Message != staffauth.MsgWorkStateRequired {
		t.Fatalf("message = %q, want %q", out.Message, staffauth.MsgWorkStateRequired)
	}
	state, events := readWorkState(t, db, "not-a-state-uid")
	if state != "NY" {
		t.Fatalf("work_state = %q, want it untouched at %q", state, "NY")
	}
	if events != 0 {
		t.Fatalf("work state events = %d, want 0 -- a refused change must leave no audit row", events)
	}
}

// The whole point of the ticket: a doula who moves says so, the value
// moves, and the audit row carries both sides rather than only the new one.
func TestUpdateWorkState_MovesTheValueAndRecordsBothSides(t *testing.T) {
	db := testdb.New(t)
	seedStaff(t, db, "she-moved-uid")
	before := reportedAt(t, db, "she-moved-uid")
	srv, session := newWorkStateServer(t, db, "she-moved-uid")
	defer srv.Close()

	// Lower case and padded, because a form is not an enforcement boundary
	// on this path either.
	resp := putWorkState(t, srv, session, `{"workState":" nj "}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got staffauth.WorkStateResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.WorkState != "NJ" {
		t.Fatalf("response workState = %q, want %q", got.WorkState, "NJ")
	}

	state, events := readWorkState(t, db, "she-moved-uid")
	if state != "NJ" {
		t.Fatalf("work_state = %q, want %q", state, "NJ")
	}
	if events != 1 {
		t.Fatalf("work state events = %d, want 1", events)
	}
	previous, current, actor := latestWorkStateEvent(t, db, "she-moved-uid")
	if previous != "NY" || current != "NJ" {
		t.Fatalf("event = %q -> %q, want %q -> %q", previous, current, "NY", "NJ")
	}
	var staffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT id FROM staff WHERE identity_uid = $1`, "she-moved-uid",
	).Scan(&staffID); err != nil {
		t.Fatalf("read staff id: %v", err)
	}
	if actor != staffID {
		t.Fatalf("actor_staff_id = %q, want her own id %q", actor, staffID)
	}

	after := reportedAt(t, db, "she-moved-uid")
	if !after.After(before) {
		t.Fatalf("work_state_reported_at = %v, want it after %v", after, before)
	}
	// The roster prints the date the row carries, so the response has to
	// echo that same instant rather than a second clock read.
	if !got.WorkStateReportedAt.Equal(after) {
		t.Fatalf("response workStateReportedAt = %v, want the stored %v", got.WorkStateReportedAt, after)
	}
}

// Saving the same state again is a re-assertion, not a no-op: the date is
// the only staleness signal the design has, so "yes, still New York, as of
// today" has to be sayable.
func TestUpdateWorkState_SameStateIsAReAssertion(t *testing.T) {
	db := testdb.New(t)
	seedStaff(t, db, "still-here-uid")
	before := reportedAt(t, db, "still-here-uid")
	srv, session := newWorkStateServer(t, db, "still-here-uid")
	defer srv.Close()

	resp := putWorkState(t, srv, session, `{"workState":"NY"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_, events := readWorkState(t, db, "still-here-uid")
	if events != 1 {
		t.Fatalf("work state events = %d, want 1 -- an unchanged value is still an assertion", events)
	}
	previous, current, _ := latestWorkStateEvent(t, db, "still-here-uid")
	if previous != "NY" || current != "NY" {
		t.Fatalf("event = %q -> %q, want both sides %q", previous, current, "NY")
	}
	if !reportedAt(t, db, "still-here-uid").After(before) {
		t.Fatal("work_state_reported_at did not move -- a re-assertion that refreshes nothing is pointless")
	}
}

// Reach: an Owner reads the value on the roster and cannot write it. There
// is no staff id in the route to aim at someone else, so the proof is that
// an Owner's own correction moves her own row and nobody else's.
func TestUpdateWorkState_AnOwnerCorrectsOnlyHerself(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Reach Practice")
	ownerID := seedStaff(t, db, "owner-correcting-uid")
	seedMembership(t, db, practiceID, ownerID)
	doulaID := seedStaff(t, db, "her-doula-uid")
	seedMembership(t, db, practiceID, doulaID)

	srv, session := newWorkStateServer(t, db, "owner-correcting-uid")
	defer srv.Close()
	resp := putWorkState(t, srv, session, `{"workState":"VT"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if state, _ := readWorkState(t, db, "owner-correcting-uid"); state != "VT" {
		t.Fatalf("owner work_state = %q, want %q", state, "VT")
	}
	if state, events := readWorkState(t, db, "her-doula-uid"); state != "NY" || events != 0 {
		t.Fatalf("doula work_state = %q with %d events, want %q with 0 -- an Owner reached another person's row", state, events, "NY")
	}
}

// The policy itself, not the handler agreeing with it: 00044's
// staff_self_update is what makes an Owner aiming straight at SQL miss.
// Exercised through db.App with the session variables set by hand, so a
// future route that forgets the shape rule still meets the boundary.
func TestRLS_StaffSelfUpdateReachesOnlyHerOwnRow(t *testing.T) {
	db := testdb.New(t)
	seedStaff(t, db, "rls-owner-uid")
	otherID := seedStaff(t, db, "rls-doula-uid")

	if _, err := db.App.ExecContext(t.Context(),
		`SELECT set_config('app.current_identity_uid', $1, false)`, "rls-owner-uid",
	); err != nil {
		t.Fatalf("set identity: %v", err)
	}
	result, err := db.App.ExecContext(t.Context(),
		`UPDATE staff SET work_state = 'TX' WHERE id = $1`, otherID,
	)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if affected != 0 {
		t.Fatalf("rows affected = %d, want 0 -- RLS let one person rewrite another's work state", affected)
	}
}

// The audit table's own boundary: a row claiming someone else made the
// change is refused by staff_work_state_events_self's WITH CHECK, so
// actor_staff_id stays a signal rather than a field anyone may fill in.
func TestRLS_WorkStateEventRefusesAForeignActor(t *testing.T) {
	db := testdb.New(t)
	subjectID := seedStaff(t, db, "event-subject-uid")
	foreignID := seedStaff(t, db, "event-foreign-uid")

	if _, err := db.App.ExecContext(t.Context(),
		`SELECT set_config('app.current_identity_uid', $1, false)`, "event-subject-uid",
	); err != nil {
		t.Fatalf("set identity: %v", err)
	}
	_, err := db.App.ExecContext(t.Context(),
		`INSERT INTO staff_work_state_events (staff_id, previous_work_state, work_state, actor_staff_id)
		 VALUES ($1, 'NY', 'TX', $2)`,
		subjectID, foreignID,
	)
	if err == nil {
		t.Fatal("insert succeeded -- an event may not name an actor who is not the caller")
	}
}
