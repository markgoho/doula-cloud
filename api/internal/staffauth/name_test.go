package staffauth_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
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

func putJSON(t *testing.T, url, session, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, url, strings.NewReader(body))
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

func TestSignup_StoresBothNamesAndRecordsTheStatement(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "two-names-uid", Email: testStaffEmail}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		PracticeName: "Two Names Practice", FirstName: "  Mary Anne ", LastName: " Smith ", WorkState: "NY", Timezone: signupZone,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	first, last, printed, kinds := storedNames(t, db, "two-names-uid")
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
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{UID: "one-name-uid", Email: testStaffEmail}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		PracticeName: "P", FirstName: "Mary", LastName: "  ", WorkState: "NY", Timezone: signupZone,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	details := decodeDetails(t, resp)
	if details["lastName"] != staffauth.MsgLastNameNeeded {
		t.Fatalf("details = %v, want a lastName entry", details)
	}
	if _, ok := details["firstName"]; ok {
		t.Fatalf("details = %v, want no firstName entry", details)
	}
}

func TestAcceptInvite_RefusesEachEmptyNameOnItsOwn(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-for-names")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "lena@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))
	srv := newAcceptServer(t, db, "lena-uid", "lena@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{InviteToken: token, FirstName: "", LastName: "Vasquez", WorkState: "NY"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
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
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	first, last, printed, kinds := storedNames(t, db, "lena2-uid")
	if first != "Lena" || last != "de la Cruz" || printed != "Lena de la Cruz" {
		t.Fatalf("names = %q, %q, %q", first, last, printed)
	}
	if len(kinds) != 1 || kinds[0] != "stated" {
		t.Fatalf("name events = %v, want one 'stated'", kinds)
	}
}

// --- Correcting a name (#1537) ------------------------------------------

func TestUpdateName_MissingCookie(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newWorkStateServer(t, db, "no-cookie-naming")
	defer srv.Close()

	resp := putJSON(t, srv.URL+"/api/staff/name", "", `{"firstName":"A","lastName":"B"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestUpdateName_UnknownStaff(t *testing.T) {
	db := testdb.New(t)
	srv, session := newWorkStateServer(t, db, "session-without-a-staff-row-for-name")
	defer srv.Close()

	resp := putJSON(t, srv.URL+"/api/staff/name", session, `{"firstName":"A","lastName":"B"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestUpdateName_RejectsMalformedBody(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "malformed-name-uid")
	srv, session := newWorkStateServer(t, db, "malformed-name-uid")
	defer srv.Close()

	resp := putJSON(t, srv.URL+"/api/staff/name", session, `not json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestUpdateName_RefusesEachEmptyNameAndChangesNothing(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "empty-name-uid")
	srv, session := newWorkStateServer(t, db, "empty-name-uid")
	defer srv.Close()

	resp := putJSON(t, srv.URL+"/api/staff/name", session, `{"firstName":" ","lastName":""}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	details := decodeDetails(t, resp)
	if details["firstName"] != staffauth.MsgFirstNameNeeded || details["lastName"] != staffauth.MsgLastNameNeeded {
		t.Fatalf("details = %v, want both entries", details)
	}
	first, _, _, kinds := storedNames(t, db, "empty-name-uid")
	if first != "Test" || len(kinds) != 0 {
		t.Fatalf("a refused correction changed something: first = %q, events = %v", first, kinds)
	}
}

func TestUpdateName_ChangesBothAndRecordsWhoAndWhen(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "renamed-uid")
	srv, session := newWorkStateServer(t, db, "renamed-uid")
	defer srv.Close()

	resp := putJSON(t, srv.URL+"/api/staff/name", session, `{"firstName":" Mary Anne ","lastName":"Smith"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got staffauth.NameResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.FirstName != maryAnne || got.LastName != smith {
		t.Fatalf("response = %+v", got)
	}
	first, last, printed, kinds := storedNames(t, db, "renamed-uid")
	if first != maryAnne || last != smith || printed != "Mary Anne Smith" {
		t.Fatalf("stored = %q, %q, %q", first, last, printed)
	}
	if len(kinds) != 1 || kinds[0] != "changed" {
		t.Fatalf("name events = %v, want one 'changed'", kinds)
	}
	var actor string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id FROM staff_name_events WHERE staff_id = $1`, staffID,
	).Scan(&actor); err != nil {
		t.Fatalf("read actor: %v", err)
	}
	if actor != staffID {
		t.Fatalf("actor_staff_id = %q, want her own id %q", actor, staffID)
	}
}

// The audit table's own boundary: a row claiming someone else made the
// change is refused by staff_name_events_self's WITH CHECK.
func TestRLS_StaffNameEventRefusesAForeignActor(t *testing.T) {
	db := testdb.New(t)
	subjectID := testdb.SeedStaff(t, db, "name-event-subject")
	foreignID := testdb.SeedStaff(t, db, "name-event-foreign")

	if _, err := db.App.ExecContext(t.Context(),
		`SELECT set_config('app.current_identity_uid', $1, false)`, "name-event-subject",
	); err != nil {
		t.Fatalf("set identity: %v", err)
	}
	_, err := db.App.ExecContext(t.Context(),
		`INSERT INTO staff_name_events (staff_id, kind, actor_staff_id) VALUES ($1, 'changed', $2)`,
		subjectID, foreignID,
	)
	if err == nil {
		t.Fatal("insert succeeded -- an event may not name an actor who is not the caller")
	}
}
