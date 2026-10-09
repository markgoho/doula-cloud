package engagementrequest_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/engagementrequest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

func doulasURL(base, practiceID string) string {
	return base + "/api/practices/" + practiceID + "/engagement-request-doulas"
}

// readDoulas reads the Start work form's list as uid.
func readDoulas(t *testing.T, db *testdb.DB, practiceID, uid string) engagementrequest.DoulasResponse {
	t.Helper()
	srv, session := newServer(t, db, uid, &tasknudge.FakeEnqueuer{})
	defer srv.Close()
	var out engagementrequest.DoulasResponse
	decode(t, get(t, doulasURL(srv.URL, practiceID), session), http.StatusOK, &out)
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
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "solo-owner", founderRoles, employeeType)

	got := readDoulas(t, db, practiceID, "solo-owner")
	if len(got.Items) != 1 || got.Items[0].StaffID != ownerID {
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
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	ownerID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "owner-1", "Renata Alvarez", founderRoles, employeeType)
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-z", "Zora Bell", []string{doulaRole}, employeeType)
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-a", "Amara Okafor", []string{doulaRole}, employeeType)
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, "contractor-1", "Lena Vasquez", []string{doulaRole}, contractorType)
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, "admin-1", "Priya Nair", []string{adminRole}, employeeType)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_invitations
		     (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, 'invited@example.com', '{doula}'::practice_role[], 'employee', $2, $3, $4)`,
		practiceID, staffauth.TokenDigest("invited-token"), ownerID, time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	got := readDoulas(t, db, practiceID, "owner-1")
	want := []string{"Renata Alvarez", "Zora Bell", "Amara Okafor"} // last name, then first name (#1537)
	if !slices.Equal(names(got.Items), want) {
		t.Fatalf("items = %v, want %v", names(got.Items), want)
	}
	if got.Items[0].StaffID != ownerID {
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
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, "admin-1", "Priya Nair", []string{adminRole}, employeeType)
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-1", "Hana Kim", []string{doulaRole}, employeeType)

	got := readDoulas(t, db, practiceID, "admin-1")
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
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	testdb.SeedStaffAtPractice(t, db, practiceID, "doula-2", []string{doulaRole}, employeeType)

	got := readDoulas(t, db, practiceID, "doula-1")
	if len(got.Items) != 1 || got.Items[0].StaffID != doulaID {
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
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	testdb.SeedStaffAtPractice(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)

	if got := readDoulas(t, db, practiceID, "doula-1"); !got.CallerIsOnlyDoula {
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
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	const ownerName, adminName = "Deborah Ridge", "Maya Okonkwo"
	ownerID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "owner-1", ownerName, []string{ownerRole, doulaRole}, contractorType)
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, "admin-1", adminName, []string{adminRole, doulaRole}, contractorType)
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, "contractor-1", "Lena Vasquez", []string{doulaRole}, contractorType)

	for _, uid := range []string{"owner-1", "admin-1"} {
		got := readDoulas(t, db, practiceID, uid)
		want := []string{ownerName, adminName}
		if uid == "admin-1" {
			want = []string{adminName, ownerName}
		}
		if !slices.Equal(names(got.Items), want) {
			t.Fatalf("%s items = %v, want %v", uid, names(got.Items), want)
		}
		if got.CallerIsOnlyDoula {
			t.Fatalf("%s callerIsOnlyDoula = true, want false with other Doulas at the Practice", uid)
		}
	}
	if got := readDoulas(t, db, practiceID, "owner-1"); got.Items[0].StaffID != ownerID {
		t.Fatalf("first item = %+v, want the person at the form", got.Items[0])
	}
}

// TestDoulasHandler_ASoloOwnerWhoIsAContractorIsSelected is the Practice
// of one that #1625 found: its only Owner is a contractor Doula. She is
// listed, and she is the only Doula, so her answer is selected when the
// form opens, as for each solo Owner.
func TestDoulasHandler_ASoloOwnerWhoIsAContractorIsSelected(t *testing.T) {
	db := testdb.New(t)
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "solo-owner", founderRoles, contractorType)

	got := readDoulas(t, db, practiceID, "solo-owner")
	if len(got.Items) != 1 || got.Items[0].StaffID != ownerID {
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
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "owner-1", founderRoles, employeeType)
	testdb.SeedContractorAtPractice(t, db, practiceID, "contractor-1")

	got := readDoulas(t, db, practiceID, "owner-1")
	if len(got.Items) != 1 || got.Items[0].StaffID != ownerID {
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
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "owner-1", []string{ownerRole}, employeeType)
	testdb.SeedContractorAtPractice(t, db, practiceID, "contractor-1")

	srv, session := newServer(t, db, "contractor-1", &tasknudge.FakeEnqueuer{})
	defer srv.Close()
	expectStatus(t, get(t, doulasURL(srv.URL, practiceID), session), http.StatusForbidden)
}
