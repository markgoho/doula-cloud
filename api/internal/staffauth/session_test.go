package staffauth_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
)

// sessionAs reads GET /api/staff/session with credential and requires
// the 200.
func (e *env) sessionAs(credential reqtest.Option) staffauth.SessionResponse {
	e.T.Helper()
	return reqtest.Expect[staffauth.SessionResponse](e.T, http.MethodGet, e.URL("/api/staff/session"), http.StatusOK, credential)
}

func TestSessionHandler_Refuses(t *testing.T) {
	cases := []struct {
		name    string
		want    int
		session func(e *env) reqtest.Option
	}{
		{"no cookie", http.StatusUnauthorized, func(*env) reqtest.Option { return reqtest.Session("") }},
		// A cookie that names no live session -- the shape a stale or
		// forged cookie arrives in.
		{"a session that was never issued", http.StatusUnauthorized, func(*env) reqtest.Option { return reqtest.Session("never-issued") }},
		{"a session with no staff row", http.StatusNotFound, func(e *env) reqtest.Option {
			return reqtest.Session(authntest.SeedSession(e.T, e.DB.App, "no-such-staff"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)

			reqtest.ExpectStatus(t, http.MethodGet, e.URL("/api/staff/session"), tc.want, tc.session(e))
		})
	}
}

func TestSessionHandler_SingleMembership(t *testing.T) {
	e := newEnv(t)

	out := e.sessionAs(e.SignedIn)

	if out.StaffID != e.Staff.ID {
		t.Fatalf("staffId = %q, want %q", out.StaffID, e.Staff.ID)
	}
	// The shell's avatar menu shows which account a person is signed in as
	// (#452), which matters because one account holds Memberships at
	// several Practices.
	if wantEmail := e.Staff.UID + "@example.com"; out.Email != wantEmail {
		t.Fatalf("email = %q, want %q", out.Email, wantEmail)
	}
	if len(out.Memberships) != 1 || out.Memberships[0].PracticeID != e.Practice.ID {
		t.Fatalf("memberships = %+v, want single membership at %q", out.Memberships, e.Practice.ID)
	}
	if len(out.Memberships[0].Roles) != 1 || out.Memberships[0].Roles[0] != doulaRole {
		t.Fatalf("roles = %v, want [doula]", out.Memberships[0].Roles)
	}
	if out.LastPracticeID != nil {
		t.Fatalf("lastPracticeId = %v, want nil (never set)", *out.LastPracticeID)
	}
}

// TestSessionHandler_SecondFactor proves the response carries this
// session's own second-factor fact (#606) -- the account screen's
// "Enroll" vs "Remove" branch reads it straight from here, not from a
// fresh Identity Platform lookup.
func TestSessionHandler_SecondFactor(t *testing.T) {
	e := newEnv(t)

	unenrolled := reqtest.Session(authntest.SeedSessionWithSecondFactor(t, e.DB.App, e.Staff.UID, false))
	if e.sessionAs(unenrolled).SecondFactor {
		t.Fatalf("secondFactor = true, want false")
	}

	enrolled := reqtest.Session(authntest.SeedSessionWithSecondFactor(t, e.DB.App, e.Staff.UID, true))
	if !e.sessionAs(enrolled).SecondFactor {
		t.Fatalf("secondFactor = false, want true")
	}
}

func TestSessionHandler_MultiplePracticesWithLastUsed(t *testing.T) {
	e := newEnv(t)
	practiceB := fixture.Practice(t, e.DB, fixture.PracticeName("Practice B"))
	seedMembership(t, e.DB, practiceB.ID, e.Staff.ID)
	e.Exec(`UPDATE staff SET last_practice_id = $1 WHERE id = $2`, practiceB.ID, e.Staff.ID)
	e.Exec(`UPDATE practice_memberships SET employment_type = 'contractor' WHERE practice_id = $1 AND staff_id = $2`, practiceB.ID, e.Staff.ID)

	out := e.sessionAs(e.SignedIn)

	if len(out.Memberships) != 2 {
		t.Fatalf("expected 2 memberships, got %d", len(out.Memberships))
	}
	if out.LastPracticeID == nil || *out.LastPracticeID != practiceB.ID {
		t.Fatalf("lastPracticeId = %v, want %q", out.LastPracticeID, practiceB.ID)
	}
	// #2266: the switcher says what she is at each Practice, so each
	// Membership carries its own employment type.
	want := map[string]string{e.Practice.ID: employeeType, practiceB.ID: contractorType}
	for _, m := range out.Memberships {
		if m.EmploymentType != want[m.PracticeID] {
			t.Fatalf("employmentType at %q = %q, want %q", m.PracticeID, m.EmploymentType, want[m.PracticeID])
		}
	}
}

// TestSessionHandler_SoleOwner is the two-Owner fixture #694's account
// screen turns on: a sole Owner is offered saved recovery codes, a
// co-Owner is offered none.
//
// It is here rather than beside the rotate endpoint because of what it
// is really guarding. The "is there another Owner?" half of the question
// reads Membership rows belonging to *other people*, and this endpoint's
// transaction sets only app.current_identity_uid -- so under
// practice_memberships_self_visibility that subquery sees nothing, finds
// no other Owner, and answers "sole" for everyone. The failure is
// silent: no error, no 403, just a co-Owner shown a section whose button
// then 403s. The second half of this test, with a second Owner seeded at
// the same Practice, is the assertion that catches it.
func TestSessionHandler_SoleOwner(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	if got := e.sessionAs(e.SignedIn); !got.SoleOwner {
		t.Fatalf("soleOwner = false for the only Owner of a practice, want true")
	}

	// A second Owner at the same Practice. Neither of them is sole now,
	// and neither can see the other's Membership row through RLS -- which
	// is the whole point of the assertions below.
	second := e.Colleague(fixture.Owner())

	if got := e.sessionAs(e.SignedIn); got.SoleOwner {
		t.Fatalf("soleOwner = true once a second Owner holds a membership at the same practice, want false")
	}
	if got := e.sessionAs(e.As(second)); got.SoleOwner {
		t.Fatalf("soleOwner = true for the second Owner, want false")
	}
}

// TestSessionHandler_SoleOwnerFalseForNonOwner covers the population
// #615's AC names last: somebody who is not an Owner anywhere holds no
// saved codes, and the account screen must not imply she does.
func TestSessionHandler_SoleOwnerFalseForNonOwner(t *testing.T) {
	e := newEnv(t)

	if got := e.sessionAs(e.SignedIn); got.SoleOwner {
		t.Fatalf("soleOwner = true for a Doula, want false")
	}
}
