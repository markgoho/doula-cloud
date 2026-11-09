package engagementrequest_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
)

// readDoulas reads the Start work form's list as who.
func (e *env) readDoulas(who fixture.StaffRecord) engagementrequest.DoulasResponse {
	e.T.Helper()
	return reqtest.Expect[engagementrequest.DoulasResponse](e.T, http.MethodGet, e.PracticeURL("/engagement-request-doulas"), http.StatusOK, e.As(who))
}

func names(list []engagementrequest.RequestDoula) []string {
	out := make([]string, 0, len(list))
	for _, d := range list {
		out = append(out, d.Name)
	}
	return out
}

// TestDoulasHandler_ListsWhatTheCallerMayRead is the Start work form's list
// at Practices of each shape, read by the person at the form. Nothing is
// selected for her unless she is the only Doula, and a contractor Doula is
// never listed although her being at the Practice stops the pre-selection.
func TestDoulasHandler_ListsWhatTheCallerMayRead(t *testing.T) {
	const caller = "Renata Alvarez"
	for _, tc := range []struct {
		name      string
		signedIn  []fixture.StaffOption
		seed      func(e *env)
		want      []string
		onlyDoula bool
	}{
		{
			// A solo Owner reads herself and the fact that selects her answer
			// when the form opens.
			name:     "the only Doula is listed and marked",
			signedIn: []fixture.StaffOption{fixture.Founder()},
			want:     []string{caller}, onlyDoula: true,
		},
		{
			// A solo Owner whose only Owner is a contractor Doula (#1625) is
			// listed, and marked.
			name:     "a solo Owner who is a contractor is selected",
			signedIn: []fixture.StaffOption{fixture.Founder(), fixture.Contractor()},
			want:     []string{caller}, onlyDoula: true,
		},
		{
			// Herself first, then each employee Doula by last name and then
			// first (#1537); never a contractor, never a person with no Doula
			// role, never a person whose Invitation is pending.
			name:     "an Owner reads every employee Doula",
			signedIn: []fixture.StaffOption{fixture.Founder()},
			seed: func(e *env) {
				e.Colleague(fixture.StaffName("Zora Bell"))
				e.Colleague(fixture.StaffName("Amara Okafor"))
				e.Colleague(fixture.StaffName("Lena Vasquez"), fixture.Contractor())
				e.Colleague(fixture.StaffName("Priya Nair"), fixture.Admin())
				e.Exec(`INSERT INTO practice_invitations
				            (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
				        VALUES ($1, 'invited@example.com', '{doula}'::practice_role[], 'employee', $2, $3, $4)`,
					e.Practice.ID, staffauth.TokenDigest("invited-token"), e.Staff.ID, time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC))
			},
			want: []string{caller, "Zora Bell", "Amara Okafor"},
		},
		{
			// The list holds no caller who cannot be named: an Admin who is no
			// Doula reads her colleagues and not herself.
			name:     "an Admin with no Doula role is not listed",
			signedIn: []fixture.StaffOption{fixture.Admin()},
			seed:     func(e *env) { e.Colleague(fixture.StaffName("Hana Kim")) },
			want:     []string{"Hana Kim"},
		},
		{
			// ADR-0008 gives a plain Doula no read of the roster, so she reads
			// herself at a Practice of any size. She is still not marked the
			// only Doula, which the server knows although her own list cannot
			// show it.
			name:     "a plain Doula reads herself only",
			signedIn: []fixture.StaffOption{},
			seed: func(e *env) {
				e.Colleague(fixture.Owner())
				e.Colleague()
			},
			want: []string{caller},
		},
		{
			// The mark does not depend on approval authority: the one employee
			// Doula of an Owner who is no Doula is selected on her own form.
			name:     "a plain Doula who is the only Doula is marked",
			signedIn: []fixture.StaffOption{},
			seed:     func(e *env) { e.Colleague(fixture.Owner()) },
			want:     []string{caller}, onlyDoula: true,
		},
		{
			name:     "a contractor Doula counts as another Doula",
			signedIn: []fixture.StaffOption{fixture.Founder()},
			seed:     func(e *env) { e.Colleague(fixture.Contractor()) },
			want:     []string{caller},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(append([]fixture.StaffOption{fixture.StaffName(caller)}, tc.signedIn...)...))
			if tc.seed != nil {
				tc.seed(e)
			}

			got := e.readDoulas(e.Staff)
			if !slices.Equal(names(got.Items), tc.want) {
				t.Fatalf("items = %v, want %v", names(got.Items), tc.want)
			}
			if got.CallerIsOnlyDoula != tc.onlyDoula {
				t.Fatalf("callerIsOnlyDoula = %v, want %v", got.CallerIsOnlyDoula, tc.onlyDoula)
			}
		})
	}
}

// TestDoulasHandler_AnOwnerOrAdminWhoIsAContractorDoulaIsListed is #1625
// on the form's list. An Owner who is a contractor Doula reads herself
// first, and an Admin who is a contractor Doula is beside her: each holds
// a role that makes her a part of the Practice, so each is attached
// without an Offer. The contractor Doula who holds neither role is still
// not listed.
func TestDoulasHandler_AnOwnerOrAdminWhoIsAContractorDoulaIsListed(t *testing.T) {
	const ownerName, adminName = "Deborah Ridge", "Maya Okonkwo"
	e := newEnv(t, signedInAs(fixture.StaffName(ownerName), fixture.Owner(), fixture.Doula(), fixture.Contractor()))
	admin := e.Colleague(fixture.StaffName(adminName), fixture.Admin(), fixture.Doula(), fixture.Contractor())
	e.Colleague(fixture.StaffName("Lena Vasquez"), fixture.Contractor())

	for _, caller := range []struct {
		who  fixture.StaffRecord
		name string
		want []string
	}{
		{e.Staff, "owner", []string{ownerName, adminName}},
		{admin, "admin", []string{adminName, ownerName}},
	} {
		got := e.readDoulas(caller.who)
		if !slices.Equal(names(got.Items), caller.want) {
			t.Fatalf("%s items = %v, want %v", caller.name, names(got.Items), caller.want)
		}
		if got.CallerIsOnlyDoula {
			t.Fatalf("%s callerIsOnlyDoula = true, want false with other Doulas at the Practice", caller.name)
		}
	}
}

// TestDoulasHandler_RefusesAContractor proves a contractor Doula, who
// originates nothing (ADR-0017), is refused the list as she is refused
// the ask.
func TestDoulasHandler_RefusesAContractor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	e.Colleague(fixture.Owner())

	reqtest.ExpectStatus(t, http.MethodGet, e.PracticeURL("/engagement-request-doulas"), http.StatusForbidden, e.SignedIn)
}
