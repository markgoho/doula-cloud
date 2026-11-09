package visit_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// assigneesURL is the one place the endpoint's path is spelled, so a case
// reads as "this Engagement" rather than as string concatenation.
func (e *env) assigneesURL(engagementID string) string {
	return e.PracticeURL("/engagements/" + engagementID + "/visit-assignees")
}

// assignees reads who may be named on this Engagement as the Staff member
// at the screen, keyed by staff id so a case can name the one row it is
// about rather than counting positions.
func (e *env) assignees() map[string]visit.Assignee {
	e.T.Helper()
	out := reqtest.Expect[visit.AssigneesResponse](e.T, http.MethodGet, e.assigneesURL(e.Engagement.ID), http.StatusOK, e.SignedIn)
	byID := map[string]visit.Assignee{}
	for _, row := range out.Items {
		byID[row.StaffID] = row
	}
	return byID
}

// TestVisitAssignees_MarksWhoMayBeNamedOnThisEngagement is #911's whole
// point in one case: the same Practice roster answers differently per
// Engagement, and nobody is dropped from it. The employee is nameable
// because an Admin may attach her directly; the unattached contractor is
// not, and says why; the contractor holding an open granted attachment
// is; and the contractor whose attachment was *ended* -- what Engagement
// completion does -- is not, which is exactly the row a screen deriving
// eligibility from the Offers list would have got wrong.
func TestVisitAssignees_MarksWhoMayBeNamedOnThisEngagement(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	employee := e.Colleague()
	unattached := e.Colleague(fixture.Contractor())
	attached := e.Colleague(fixture.Contractor())
	ended := e.Colleague(fixture.Contractor())
	accrued := e.Colleague(fixture.Contractor())
	bookkeeper := e.Colleague(fixture.Roles(adminRole))
	testdb.SeedGrantedAttachment(t, e.DB, e.Engagement.ID, attached.ID)
	testdb.SeedAttachment(t, e.DB, e.Engagement.ID, ended.ID, grantedOrigin, true)
	testdb.SeedAttachment(t, e.DB, e.Engagement.ID, accrued.ID, "accrued", false)

	rows := e.assignees()

	for _, want := range []struct {
		label    string
		staffID  string
		nameable bool
		reason   string
	}{
		{"an employee Doula", employee.ID, true, ""},
		{"a contractor with no attachment", unattached.ID, false, visit.ReasonContractorWithoutAcceptedOffer},
		{"a contractor with an open granted attachment", attached.ID, true, ""},
		{"a contractor whose granted attachment was ended", ended.ID, false, visit.ReasonContractorWithoutAcceptedOffer},
		{"a contractor with only an accrued attachment", accrued.ID, false, visit.ReasonContractorWithoutAcceptedOffer},
	} {
		row, listed := rows[want.staffID]
		if !listed {
			t.Fatalf("%s: not listed at all -- #911 marks her, it does not drop her", want.label)
		}
		if row.Nameable != want.nameable || row.Reason != want.reason {
			t.Errorf("%s: nameable = %v, reason = %q; want %v, %q",
				want.label, row.Nameable, row.Reason, want.nameable, want.reason)
		}
		if row.Name == "" {
			t.Errorf("%s: name is empty -- the picker offers a person by name", want.label)
		}
	}

	// The Owner here holds no Doula role, so she is not somebody a Visit
	// can be put on, and neither is the bookkeeper.
	for label, staffID := range map[string]string{"the Owner, who is not a Doula": e.Staff.ID, "the bookkeeper": bookkeeper.ID} {
		if _, listed := rows[staffID]; listed {
			t.Errorf("%s is in the picker; only Doula-role Memberships belong there", label)
		}
	}
}

// TestVisitAssignees_CallerIsNameableAsHerself covers the row the
// named-colleague rule must not be applied to. assignee (roles.go) takes
// the self path whenever the requested id equals the caller's own, so a
// contractor Doula who owns the Practice is accepted by the write when
// she names herself with no attachment at all -- and the read has to say
// so, or the screen would block a write the BFF allows.
func TestVisitAssignees_CallerIsNameableAsHerself(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula(), fixture.Contractor()))
	other := e.Colleague(fixture.Contractor())

	rows := e.assignees()

	if self := rows[e.Staff.ID]; !self.Nameable || self.Reason != "" {
		t.Errorf("the caller's own row: nameable = %v, reason = %q; want true, \"\" -- she needs no attachment to log her own Visit",
			self.Nameable, self.Reason)
	}
	// The same employment type, the same absent attachment, a different
	// person: the rule that applies to her is the stricter one.
	if row := rows[other.ID]; row.Nameable {
		t.Error("another unattached contractor is nameable; only the caller's own row takes the self rule")
	}
}

// TestVisitAssignees_AnOwnerOrAdminWhoIsAContractorDoulaIsNameable is
// #1625 on the pickers: a colleague who is a contractor Doula and holds
// Owner or Admin is nameable with no attachment, because the write
// accepts her name and attaches her. The contractor Doula who holds
// neither role is beside her and is still marked with the reason.
func TestVisitAssignees_AnOwnerOrAdminWhoIsAContractorDoulaIsNameable(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	owner := e.Colleague(fixture.Owner(), fixture.Doula(), fixture.Contractor())
	admin := e.Colleague(fixture.Admin(), fixture.Doula(), fixture.Contractor())
	plain := e.Colleague(fixture.Contractor())

	rows := e.assignees()

	for label, staffID := range map[string]string{"an Owner who is a contractor Doula": owner.ID, "an Admin who is a contractor Doula": admin.ID} {
		row, listed := rows[staffID]
		if !listed {
			t.Fatalf("%s: not listed", label)
		}
		if !row.Nameable || row.Reason != "" {
			t.Errorf("%s: nameable = %v, reason = %q; want true, \"\"", label, row.Nameable, row.Reason)
		}
		if row.EmploymentType != "contractor" {
			t.Errorf("%s: employmentType = %q, want contractor -- the row still says what she is to the business", label, row.EmploymentType)
		}
	}
	if row := rows[plain.ID]; row.Nameable || row.Reason != visit.ReasonContractorWithoutAcceptedOffer {
		t.Errorf("a contractor Doula who holds neither role: nameable = %v, reason = %q; want false, %q",
			row.Nameable, row.Reason, visit.ReasonContractorWithoutAcceptedOffer)
	}
}

// TestVisitAssignees_Refuses covers who and what the roster read turns
// away: an Engagement at another Practice, confirmed to be this
// Practice's before any roster is answered (otherwise a guessed id would
// answer 200 with a whole Practice's Doulas and their attachment state);
// an Engagement id that is not a uuid; and a plain Doula, to hold the
// endpoint to the audience of the roster read the pickers used before it
// -- naming a colleague is scheduling (ADR-0006, ADR-0008), and a plain
// Doula has no roster to pick one from.
func TestVisitAssignees_Refuses(t *testing.T) {
	cases := []struct {
		name  string
		staff []fixture.StaffOption
		// engagementID returns the Engagement the request names.
		engagementID func(e *env) string
		want         int
	}{
		{"an Engagement at another Practice", []fixture.StaffOption{fixture.Owner(), fixture.Doula()},
			func(e *env) string {
				return fixture.Engagement(e.T, e.DB, fixture.Practice(e.T, e.DB, fixture.PracticeName("Another Practice"))).ID
			}, http.StatusNotFound},
		{"an Engagement id that is not a uuid", []fixture.StaffOption{fixture.Owner(), fixture.Doula()},
			func(*env) string { return notAUUID }, http.StatusBadRequest},
		{"a plain Doula", []fixture.StaffOption{fixture.Doula()},
			func(e *env) string { return e.Engagement.ID }, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff...))

			reqtest.ExpectStatus(t, http.MethodGet, e.assigneesURL(tc.engagementID(e)), tc.want, e.SignedIn)
		})
	}
}
