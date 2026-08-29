package staffauth_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

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
