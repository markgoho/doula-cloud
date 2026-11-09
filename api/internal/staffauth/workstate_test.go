package staffauth_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
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

// TestSignup_RefusesAWorkStateThatIsNotOne covers a missing state and a
// two-letter string that is not a state: both are refused by the handler
// rather than by the CHECK constraint, so the caller gets a 400 naming the
// field, not the 500 an unhandled constraint violation would produce.
func TestSignup_RefusesAWorkStateThatIsNotOne(t *testing.T) {
	cases := []struct {
		name      string
		workState string
	}{
		{"missing", ""},
		{"not a state", "ZZ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newSignupEnv(t, authntest.Verifier{UID: workStateUID, Email: testStaffEmail})

			e.refused(staffauth.SignupRequest{
				PracticeName: "Bad State Practice", FirstName: workStateName, LastName: placeholderLastName,
				WorkState: tc.workState,
			}, http.StatusBadRequest)
		})
	}
}

// The form offers a fixed <select>, but a form is not an enforcement
// boundary: whatever arrives is trimmed and upper-cased before it is
// stored, so the apportionment query never has to match case.
func TestSignup_NormalizesWorkStateAndRecordsFirstEvent(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: workStateUID, Email: testStaffEmail})

	e.signUp(staffauth.SignupRequest{
		PracticeName: "Lower Case Practice", FirstName: workStateName, LastName: placeholderLastName,
		WorkState: "  nj ",
	})

	state, events := readWorkState(t, e.DB, workStateUID)
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
		InviteToken: token, FirstName: "Rosa", LastName: "Mendel", WorkState: "pa",
	})
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusOK)
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
		InviteToken: token, FirstName: "Zed", LastName: "Nowhere", WorkState: "Ontario",
	})
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
}

// A contractor doula joining her second Practice keeps the work state she
// already asserted, and this acceptance records no new assertion: she did
// not make one. The second Practice inherits the value -- which the roster
// prints as self-reported, with its date, so nobody has to wonder where it
// came from.
func TestAcceptInvite_IgnoresWorkStateForAnExistingPerson(t *testing.T) {
	signup := newSignupEnv(t, authntest.Verifier{UID: "roaming-uid", Email: "nell@example.com"})
	signup.signUp(staffauth.SignupRequest{
		PracticeName: "Her First Practice", FirstName: "Nell", LastName: "Ward", WorkState: "NY",
	})
	db := signup.DB

	ownerID, practiceID := seedOwnerMembership(t, db, "owner-of-the-second-practice")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "nell@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))

	acceptSrv := newAcceptServer(t, db, "roaming-uid", "nell@example.com")
	defer acceptSrv.Close()

	resp := postAccept(t, acceptSrv, staffauth.AcceptInviteRequest{
		InviteToken: token, FirstName: "Nell", LastName: "Ward", WorkState: "TX",
	})
	defer resp.Body.Close()

	reqtest.RequireStatus(t, resp, http.StatusOK)
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
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.Exec(`UPDATE staff SET work_state = 'CT' WHERE id = $1`, e.Staff.ID)

	roster := reqtest.Expect[staffauth.Roster](t, http.MethodGet, e.PracticeURL("/staff"), http.StatusOK, e.SignedIn)
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

// putWorkState corrects the work state of the Staff member at the screen.
// The route carries no {practiceId} and no {staffId}: that absence is the
// whole authorization story.
func (e *env) putWorkState(body string, want int, opts ...reqtest.Option) []byte {
	e.T.Helper()
	return reqtest.ExpectBody(e.T, http.MethodPut, e.URL("/api/staff/work-state"), want, append([]reqtest.Option{reqtest.RawJSON([]byte(body))}, opts...)...)
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
	at := testdb.Scalar[time.Time](t, db.Admin, `SELECT work_state_reported_at FROM staff WHERE identity_uid = $1`, identityUID)
	return at
}

// Refused by the handler rather than by 00043's CHECK constraint, so the
// caller gets a 400 naming the field instead of the 500 an unhandled
// constraint violation would produce -- and in the same words the two
// onboarding paths use. A refused change leaves no audit row.
func TestUpdateWorkState_RejectsSomethingThatIsNotAState(t *testing.T) {
	e := newEnv(t)

	out := apierrtest.Expect(t, http.MethodPut, e.URL("/api/staff/work-state"), http.StatusBadRequest, e.SignedIn,
		reqtest.RawJSON([]byte(`{"workState":"Ontario"}`)))
	if out.Message != staffauth.MsgWorkStateRequired {
		t.Fatalf("message = %q, want %q", out.Message, staffauth.MsgWorkStateRequired)
	}
	state, events := readWorkState(t, e.DB, e.Staff.UID)
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
	e := newEnv(t)
	before := reportedAt(t, e.DB, e.Staff.UID)

	// Lower case and padded, because a form is not an enforcement boundary
	// on this path either.
	got := reqtest.Expect[staffauth.WorkStateResponse](t, http.MethodPut, e.URL("/api/staff/work-state"), http.StatusOK, e.SignedIn,
		reqtest.RawJSON([]byte(`{"workState":" nj "}`)))
	if got.WorkState != "NJ" {
		t.Fatalf("response workState = %q, want %q", got.WorkState, "NJ")
	}

	state, events := readWorkState(t, e.DB, e.Staff.UID)
	if state != "NJ" {
		t.Fatalf("work_state = %q, want %q", state, "NJ")
	}
	if events != 1 {
		t.Fatalf("work state events = %d, want 1", events)
	}
	previous, current, actor := latestWorkStateEvent(t, e.DB, e.Staff.UID)
	if previous != "NY" || current != "NJ" {
		t.Fatalf("event = %q -> %q, want %q -> %q", previous, current, "NY", "NJ")
	}
	if actor != e.Staff.ID {
		t.Fatalf("actor_staff_id = %q, want her own id %q", actor, e.Staff.ID)
	}

	after := reportedAt(t, e.DB, e.Staff.UID)
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
	e := newEnv(t)
	before := reportedAt(t, e.DB, e.Staff.UID)

	e.putWorkState(`{"workState":"NY"}`, http.StatusOK, e.SignedIn)

	_, events := readWorkState(t, e.DB, e.Staff.UID)
	if events != 1 {
		t.Fatalf("work state events = %d, want 1 -- an unchanged value is still an assertion", events)
	}
	previous, current, _ := latestWorkStateEvent(t, e.DB, e.Staff.UID)
	if previous != "NY" || current != "NY" {
		t.Fatalf("event = %q -> %q, want both sides %q", previous, current, "NY")
	}
	if !reportedAt(t, e.DB, e.Staff.UID).After(before) {
		t.Fatal("work_state_reported_at did not move -- a re-assertion that refreshes nothing is pointless")
	}
}

// Reach: an Owner reads the value on the roster and cannot write it. There
// is no staff id in the route to aim at someone else, so the proof is that
// an Owner's own correction moves her own row and nobody else's.
func TestUpdateWorkState_AnOwnerCorrectsOnlyHerself(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	doula := e.Colleague()

	e.putWorkState(`{"workState":"VT"}`, http.StatusOK, e.SignedIn)

	if state, _ := readWorkState(t, e.DB, e.Staff.UID); state != "VT" {
		t.Fatalf("owner work_state = %q, want %q", state, "VT")
	}
	if state, events := readWorkState(t, e.DB, doula.UID); state != "NY" || events != 0 {
		t.Fatalf("doula work_state = %q with %d events, want %q with 0 -- an Owner reached another person's row", state, events, "NY")
	}
}

// setIdentity makes uid the caller db.App's policies see.
func setIdentity(t *testing.T, db *testdb.DB, uid string) {
	t.Helper()
	if _, err := db.App.ExecContext(t.Context(),
		`SELECT set_config('app.current_identity_uid', $1, false)`, uid,
	); err != nil {
		t.Fatalf("set identity: %v", err)
	}
}

// The policy itself, not the handler agreeing with it: 00044's
// staff_self_update is what makes an Owner aiming straight at SQL miss.
// Exercised through db.App with the session variables set by hand, so a
// future route that forgets the shape rule still meets the boundary.
func TestRLS_StaffSelfUpdateReachesOnlyHerOwnRow(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	other := e.Colleague()

	setIdentity(t, e.DB, e.Staff.UID)
	result, err := e.DB.App.ExecContext(t.Context(),
		`UPDATE staff SET work_state = 'TX' WHERE id = $1`, other.ID,
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
