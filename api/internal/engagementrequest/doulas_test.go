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
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

func doulasURL(base, practiceID string) string {
	return base + "/api/practices/" + practiceID + "/engagement-request-doulas"
}

// readDoulas reads the Start work form's list as who.
func readDoulas(t *testing.T, db *testdb.DB, practiceID string, who fixture.StaffRecord) engagementrequest.DoulasResponse {
	t.Helper()
	srv, session := newServer(t, db, who, &tasknudge.FakeEnqueuer{})
	defer srv.Close()
	outResp := reqtest.Get(t, doulasURL(srv.URL, practiceID), reqtest.Session(session))
	defer outResp.Body.Close()
	out := reqtest.DecodeStatus[engagementrequest.DoulasResponse](t, outResp, http.StatusOK)
	return out
}

func names(list []engagementrequest.RequestDoula) []string {
	out := make([]string, 0, len(list))
	for _, d := range list {
		out = append(out, d.Name)
	}
	return out
}

// TestDoulasHandler_TheOnlyDoulaIsListedAndMarked is the one-Doula
// Practice: a solo Owner reads herself and the fact that selects her
// answer when the form opens.
func TestDoulasHandler_TheOnlyDoulaIsListedAndMarked(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	owner := fixture.Staff(t, db, p, fixture.Founder())

	got := readDoulas(t, db, p.ID, owner)
	if len(got.Items) != 1 || got.Items[0].StaffID != owner.ID {
		t.Fatalf("items = %+v, want the Owner alone", got.Items)
	}
	if !got.CallerIsOnlyDoula {
		t.Fatal("callerIsOnlyDoula = false, want true at a Practice of one Doula")
	}
}

// TestDoulasHandler_AnOwnerReadsEveryEmployeeDoula is the Practice with
// several Doulas, as an Owner reads it: herself first, then each employee
// Doula by name; never a contractor, never a person with no Doula role,
// never a person whose Invitation is pending. Nothing is selected for
// her, because she is not the only Doula.
func TestDoulasHandler_AnOwnerReadsEveryEmployeeDoula(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	owner := fixture.Staff(t, db, p, fixture.StaffName("Renata Alvarez"), fixture.Founder())
	fixture.Staff(t, db, p, fixture.StaffName("Zora Bell"))
	fixture.Staff(t, db, p, fixture.StaffName("Amara Okafor"))
	fixture.Staff(t, db, p, fixture.StaffName("Lena Vasquez"), fixture.Contractor())
	fixture.Staff(t, db, p, fixture.StaffName("Priya Nair"), fixture.Admin())
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_invitations
		     (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, 'invited@example.com', '{doula}'::practice_role[], 'employee', $2, $3, $4)`,
		p.ID, staffauth.TokenDigest("invited-token"), owner.ID, time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	got := readDoulas(t, db, p.ID, owner)
	want := []string{"Renata Alvarez", "Zora Bell", "Amara Okafor"} // last name, then first name (#1537)
	if !slices.Equal(names(got.Items), want) {
		t.Fatalf("items = %v, want %v", names(got.Items), want)
	}
	if got.Items[0].StaffID != owner.ID {
		t.Fatalf("first item = %+v, want the person at the form", got.Items[0])
	}
	if got.CallerIsOnlyDoula {
		t.Fatal("callerIsOnlyDoula = true, want false with other Doulas at the Practice")
	}
}

// TestDoulasHandler_AnOwnerWithNoDoulaRoleIsNotListed proves the list
// holds no caller who cannot be named: an Admin who is no Doula reads her
// colleagues and not herself.
func TestDoulasHandler_AnOwnerWithNoDoulaRoleIsNotListed(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	admin := fixture.Staff(t, db, p, fixture.StaffName("Priya Nair"), fixture.Admin())
	fixture.Staff(t, db, p, fixture.StaffName("Hana Kim"))

	got := readDoulas(t, db, p.ID, admin)
	if !slices.Equal(names(got.Items), []string{"Hana Kim"}) {
		t.Fatalf("items = %v, want the one Doula and not the Admin", names(got.Items))
	}
	if got.CallerIsOnlyDoula {
		t.Fatal("callerIsOnlyDoula = true, want false: the person at the form is no Doula")
	}
}

// TestDoulasHandler_APlainDoulaReadsHerselfOnly is the plain Doula
// asker: ADR-0008 gives her no read of the roster, so she reads herself
// at a Practice of any size. She is still not marked the only Doula,
// which the server knows although her own list cannot show it. Read
// through the app_runtime connection, so the count is the one RLS allows
// her session and not the superuser's.
func TestDoulasHandler_APlainDoulaReadsHerselfOnly(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	fixture.Staff(t, db, p, fixture.Owner())
	doula := fixture.Staff(t, db, p)
	fixture.Staff(t, db, p)

	got := readDoulas(t, db, p.ID, doula)
	if len(got.Items) != 1 || got.Items[0].StaffID != doula.ID {
		t.Fatalf("items = %+v, want herself alone", got.Items)
	}
	if got.CallerIsOnlyDoula {
		t.Fatal("callerIsOnlyDoula = true, want false with a second Doula at the Practice")
	}
}

// TestDoulasHandler_APlainDoulaWhoIsTheOnlyDoulaIsMarked proves the mark
// does not depend on approval authority: the one employee Doula of an
// Owner who is no Doula is selected on her own form.
func TestDoulasHandler_APlainDoulaWhoIsTheOnlyDoulaIsMarked(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	fixture.Staff(t, db, p, fixture.Owner())
	doula := fixture.Staff(t, db, p)

	if got := readDoulas(t, db, p.ID, doula); !got.CallerIsOnlyDoula {
		t.Fatal("callerIsOnlyDoula = false, want true for the Practice's one Doula")
	}
}

// TestDoulasHandler_AnOwnerOrAdminWhoIsAContractorDoulaIsListed is #1625
// on the form's list. An Owner who is a contractor Doula reads herself
// first, and an Admin who is a contractor Doula is beside her: each holds
// a role that makes her a part of the Practice, so each is attached
// without an Offer. The contractor Doula who holds neither role is still
// not listed.
func TestDoulasHandler_AnOwnerOrAdminWhoIsAContractorDoulaIsListed(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	const ownerName, adminName = "Deborah Ridge", "Maya Okonkwo"
	owner := fixture.Staff(t, db, p, fixture.StaffName(ownerName), fixture.Owner(), fixture.Doula(), fixture.Contractor())
	admin := fixture.Staff(t, db, p, fixture.StaffName(adminName), fixture.Admin(), fixture.Doula(), fixture.Contractor())
	fixture.Staff(t, db, p, fixture.StaffName("Lena Vasquez"), fixture.Contractor())

	for _, caller := range []struct {
		who  fixture.StaffRecord
		name string
		want []string
	}{
		{owner, "owner", []string{ownerName, adminName}},
		{admin, "admin", []string{adminName, ownerName}},
	} {
		got := readDoulas(t, db, p.ID, caller.who)
		if !slices.Equal(names(got.Items), caller.want) {
			t.Fatalf("%s items = %v, want %v", caller.name, names(got.Items), caller.want)
		}
		if got.CallerIsOnlyDoula {
			t.Fatalf("%s callerIsOnlyDoula = true, want false with other Doulas at the Practice", caller.name)
		}
	}
	if got := readDoulas(t, db, p.ID, owner); got.Items[0].StaffID != owner.ID {
		t.Fatalf("first item = %+v, want the person at the form", got.Items[0])
	}
}

// TestDoulasHandler_ASoloOwnerWhoIsAContractorIsSelected is the Practice
// of one that #1625 found: its only Owner is a contractor Doula. She is
// listed, and she is the only Doula, so her answer is selected when the
// form opens, as for each solo Owner.
func TestDoulasHandler_ASoloOwnerWhoIsAContractorIsSelected(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	owner := fixture.Staff(t, db, p, fixture.Founder(), fixture.Contractor())

	got := readDoulas(t, db, p.ID, owner)
	if len(got.Items) != 1 || got.Items[0].StaffID != owner.ID {
		t.Fatalf("items = %+v, want the Owner alone", got.Items)
	}
	if !got.CallerIsOnlyDoula {
		t.Fatal("callerIsOnlyDoula = false, want true at a Practice of one Doula")
	}
}

// TestDoulasHandler_AContractorCountsAsAnotherDoula proves the reading
// DoulasResponse's own comment states: a contractor Doula is never
// listed, and her being at the Practice still stops the pre-selection.
func TestDoulasHandler_AContractorCountsAsAnotherDoula(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	owner := fixture.Staff(t, db, p, fixture.Founder())
	fixture.Staff(t, db, p, fixture.Contractor())

	got := readDoulas(t, db, p.ID, owner)
	if len(got.Items) != 1 || got.Items[0].StaffID != owner.ID {
		t.Fatalf("items = %+v, want the Owner alone", got.Items)
	}
	if got.CallerIsOnlyDoula {
		t.Fatal("callerIsOnlyDoula = true, want false with a contractor Doula at the Practice")
	}
}

// TestDoulasHandler_RefusesAContractor proves a contractor Doula, who
// originates nothing (ADR-0017), is refused the list as she is refused
// the ask.
func TestDoulasHandler_RefusesAContractor(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	fixture.Staff(t, db, p, fixture.Owner())
	contractor := fixture.Staff(t, db, p, fixture.Contractor())

	srv, session := newServer(t, db, contractor, &tasknudge.FakeEnqueuer{})
	defer srv.Close()
	resp := reqtest.Get(t, doulasURL(srv.URL, p.ID), reqtest.Session(session))
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusForbidden)
}
