package staffauth_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// storedNames reads the two columns, the printed name built from them,
// and the audit kinds, as Admin so the assertion is about what was
// written rather than what RLS lets a caller see.
func storedNames(t *testing.T, db *testdb.DB, identityUID string) (first, last, printed string, kinds []string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT first_name, last_name, name FROM staff WHERE identity_uid = $1`, identityUID,
	).Scan(&first, &last, &printed); err != nil {
		t.Fatalf("read names: %v", err)
	}
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT e.kind FROM staff_name_events e JOIN staff s ON s.id = e.staff_id
		  WHERE s.identity_uid = $1 ORDER BY e.created_at, e.id`, identityUID)
	if err != nil {
		t.Fatalf("read name events: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			t.Fatalf("scan name event: %v", err)
		}
		kinds = append(kinds, kind)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read name events: %v", err)
	}
	return first, last, printed, kinds
}

func TestAcceptInvite_RefusesEachEmptyNameOnItsOwn(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-for-names")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "lena@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))
	srv := newAcceptServer(t, db, "lena-uid", "lena@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{InviteToken: token, FirstName: "", LastName: "Vasquez", WorkState: "NY"})
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
	details := decodeDetails(t, resp)
	if details["firstName"] != staffauth.MsgFirstNameNeeded {
		t.Fatalf("details = %v, want a firstName entry", details)
	}
	if _, ok := details["lastName"]; ok {
		t.Fatalf("details = %v, want no lastName entry", details)
	}
}

func TestAcceptInvite_StoresBothNamesAndRecordsTheStatement(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-for-accept-names")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "lena2@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))
	srv := newAcceptServer(t, db, "lena2-uid", "lena2@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{InviteToken: token, FirstName: leaFirstName, LastName: "de la Cruz", WorkState: "NY"})
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)
	first, last, printed, kinds := storedNames(t, db, "lena2-uid")
	if first != "Lena" || last != "de la Cruz" || printed != "Lena de la Cruz" {
		t.Fatalf("names = %q, %q, %q", first, last, printed)
	}
	if len(kinds) != 1 || kinds[0] != "stated" {
		t.Fatalf("name events = %v, want one 'stated'", kinds)
	}
}

// --- Correcting a name (#1537) ------------------------------------------

func TestSignup_StoresBothNamesAndRecordsTheStatement(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "two-names-uid", Email: testStaffEmail})

	e.signUp(staffauth.SignupRequest{
		PracticeName: "Two Names Practice", FirstName: "  Mary Anne ", LastName: " Smith ", WorkState: "NY", Timezone: signupZone,
	})

	first, last, printed, kinds := storedNames(t, e.DB, "two-names-uid")
	if first != maryAnne || last != smith {
		t.Fatalf("names = %q, %q, want %q, %q -- no split on white space, trimmed", first, last, maryAnne, smith)
	}
	if printed != "Mary Anne Smith" {
		t.Fatalf("name = %q, want first then last", printed)
	}
	if len(kinds) != 1 || kinds[0] != "stated" {
		t.Fatalf("name events = %v, want one 'stated'", kinds)
	}
}

func TestSignup_RefusesEachEmptyNameOnItsOwn(t *testing.T) {
	e := newSignupEnv(t, authntest.Verifier{UID: "one-name-uid", Email: testStaffEmail})

	details := e.refused(staffauth.SignupRequest{
		PracticeName: "P", FirstName: "Mary", LastName: "  ", WorkState: "NY", Timezone: signupZone,
	}, http.StatusBadRequest).Details
	if details["lastName"] != staffauth.MsgLastNameNeeded {
		t.Fatalf("details = %v, want a lastName entry", details)
	}
	if _, ok := details["firstName"]; ok {
		t.Fatalf("details = %v, want no firstName entry", details)
	}
}

// TestUpdateName_RefusesEachEmptyNameAndChangesNothing is #1537's refusal:
// both names are named, and a refused correction leaves the row and its
// audit trail as they were.
func TestUpdateName_RefusesEachEmptyNameAndChangesNothing(t *testing.T) {
	e := newEnv(t)
	before, _, _, _ := storedNames(t, e.DB, e.Staff.UID)

	out := apierrtest.Expect(t, http.MethodPut, e.URL("/api/staff/name"), http.StatusBadRequest, e.SignedIn, reqtest.RawJSON([]byte(`{"firstName":" ","lastName":""}`)))
	if out.Details["firstName"] != staffauth.MsgFirstNameNeeded || out.Details["lastName"] != staffauth.MsgLastNameNeeded {
		t.Fatalf("details = %v, want both entries", out.Details)
	}
	first, _, _, kinds := storedNames(t, e.DB, e.Staff.UID)
	if first != before || len(kinds) != 0 {
		t.Fatalf("a refused correction changed something: first = %q, events = %v", first, kinds)
	}
}

func TestUpdateName_ChangesBothAndRecordsWhoAndWhen(t *testing.T) {
	e := newEnv(t)

	got := reqtest.Expect[staffauth.NameResponse](t, http.MethodPut, e.URL("/api/staff/name"), http.StatusOK, e.SignedIn,
		reqtest.RawJSON([]byte(`{"firstName":" Mary Anne ","lastName":"Smith"}`)))
	if got.FirstName != maryAnne || got.LastName != smith {
		t.Fatalf("response = %+v", got)
	}
	first, last, printed, kinds := storedNames(t, e.DB, e.Staff.UID)
	if first != maryAnne || last != smith || printed != "Mary Anne Smith" {
		t.Fatalf("stored = %q, %q, %q", first, last, printed)
	}
	if len(kinds) != 1 || kinds[0] != "changed" {
		t.Fatalf("name events = %v, want one 'changed'", kinds)
	}
	actor := testdb.Scalar[string](t, e.DB.Admin, `SELECT actor_staff_id FROM staff_name_events WHERE staff_id = $1`, e.Staff.ID)
	if actor != e.Staff.ID {
		t.Fatalf("actor_staff_id = %q, want her own id %q", actor, e.Staff.ID)
	}
}
