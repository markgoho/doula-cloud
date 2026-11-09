package staffauth_test

import (
	"context"
	"database/sql"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

type reachCase struct {
	name string
	who  fixture.StaffRecord
	want bool
}

// requireReach asks each case's Reader whether it can reach a record, in a
// transaction scoped to the Practice.
func requireReach(t *testing.T, db *testdb.DB, practice fixture.PracticeRecord, cases []reachCase,
	can func(context.Context, staffauth.Reader, *sql.Tx) (bool, error),
) {
	t.Helper()
	tx := db.BeginAs(t, testdb.Practice(practice.ID))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := staffauth.NewReader(tc.who.ID, tc.who.Roles, tc.who.Employment)
			got, err := can(t.Context(), reader, tx)
			if err != nil {
				t.Fatalf("can access: %v", err)
			}
			if got != tc.want {
				t.Fatalf("can access = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestReader_CanAccessEngagement covers every ADR-0008 read-table cell
// this method decides: Owner and Admin reach every Engagement, an
// employee Doula reaches every Engagement at the Practice (#227's
// ambient grant), and a contractor Doula reaches only an Engagement she
// holds an open, granted attachment on -- an accrued-only or ended
// attachment never reaches (#228).
func TestReader_CanAccessEngagement(t *testing.T) {
	db := testdb.New(t)
	practice := fixture.Practice(t, db)
	engagementID := fixture.Engagement(t, db, practice).ID
	staff := func(opts ...fixture.StaffOption) fixture.StaffRecord { return fixture.Staff(t, db, practice, opts...) }
	contractor := func(origin string, ended bool) fixture.StaffRecord {
		who := staff(fixture.Contractor())
		if origin != "" {
			testdb.SeedAttachment(t, db, engagementID, who.ID, origin, ended)
		}
		return who
	}

	requireReach(t, db, practice, []reachCase{
		{ownerRole, staff(fixture.Roles(ownerRole)), true},
		{adminRole, staff(fixture.Roles(adminRole)), true},
		{"employee doula, ambient", staff(), true},
		{"contractor, no attachment", contractor("", false), false},
		{"contractor, accrued only", contractor("accrued", false), false},
		{"contractor, granted but ended", contractor("granted", true), false},
		{"contractor, granted and open", contractor("granted", false), true},
	}, func(ctx context.Context, r staffauth.Reader, tx *sql.Tx) (bool, error) {
		return r.CanAccessEngagement(ctx, tx, engagementID)
	})
}

// TestReader_CanAccessClient mirrors TestReader_CanAccessEngagement for
// the Client-scoped ADR-0017 rule: attachment reaches through any of the
// Client's Engagements, not just one named one.
func TestReader_CanAccessClient(t *testing.T) {
	db := testdb.New(t)
	practice := fixture.Practice(t, db)
	engagement := fixture.Engagement(t, db, practice)
	attached := fixture.Staff(t, db, practice, fixture.Contractor())
	testdb.SeedAttachment(t, db, engagement.ID, attached.ID, "granted", false)

	requireReach(t, db, practice, []reachCase{
		{ownerRole, fixture.Staff(t, db, practice, fixture.Roles(ownerRole)), true},
		{"contractor, no attachment", fixture.Staff(t, db, practice, fixture.Contractor()), false},
		{"contractor, granted and open", attached, true},
	}, func(ctx context.Context, r staffauth.Reader, tx *sql.Tx) (bool, error) {
		return r.CanAccessClient(ctx, tx, engagement.Client.ID)
	})
}

// TestReader_RoleAndEmploymentTypePredicates confirms the axis a Reader
// carries besides roles: an employee reader (Owner and Admin included,
// #227's "employee means inside the business") is not a contractor, and
// that
// IsAmbientContractor/IsOwnerOrAdmin -- the two methods that replace the
// predicate copies #835 found at nine call sites -- agree with Has and
// IsContractor on every role/employment-type combination the codebase
// cares about. Pure: staffauth.NewReader builds a Reader directly, no
// database round trip needed to prove what Reader's own methods decide.
func TestReader_RoleAndEmploymentTypePredicates(t *testing.T) {
	cases := []struct {
		name                  string
		roles                 []string
		employmentType        string
		wantContractor        bool
		wantOwnerOrAdmin      bool
		wantAmbientContractor bool
	}{
		{"owner, employee", []string{ownerRole}, employeeType, false, true, false},
		{"admin, employee", []string{adminRole}, employeeType, false, true, false},
		{"employee doula, ambient", []string{doulaRole}, employeeType, false, false, false},
		{"plain contractor doula", []string{doulaRole}, contractorType, true, false, true},
		{"owner who also holds contractor employment type", []string{ownerRole}, contractorType, true, true, false},
		{"admin who also holds contractor employment type", []string{adminRole, doulaRole}, contractorType, true, true, false},
		{"zero-role employee", []string{}, employeeType, false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := staffauth.NewReader("staff-id", tc.roles, tc.employmentType)
			if got := reader.EmploymentType(); got != tc.employmentType {
				t.Errorf("EmploymentType() = %q, want %q", got, tc.employmentType)
			}
			if got := reader.IsContractor(); got != tc.wantContractor {
				t.Errorf("IsContractor() = %v, want %v", got, tc.wantContractor)
			}
			if got := reader.IsOwnerOrAdmin(); got != tc.wantOwnerOrAdmin {
				t.Errorf("IsOwnerOrAdmin() = %v, want %v", got, tc.wantOwnerOrAdmin)
			}
			if got := reader.IsAmbientContractor(); got != tc.wantAmbientContractor {
				t.Errorf("IsAmbientContractor() = %v, want %v", got, tc.wantAmbientContractor)
			}
		})
	}
}

// TestReader_Roles covers the accessor #501's practiceSessionHandler
// reads to hand the frontend both axes off one Reader, on both a
// populated and an empty roles slice -- nil in, "[]" out, never "null".
func TestReader_Roles(t *testing.T) {
	if roles := staffauth.NewReader("staff-id", []string{doulaRole}, employeeType).Roles(); len(roles) != 1 || roles[0] != doulaRole {
		t.Fatalf("Roles() = %v, want [doula]", roles)
	}
	if roles := staffauth.NewReader("staff-id", nil, employeeType).Roles(); len(roles) != 0 {
		t.Fatalf("Roles() = %v, want empty", roles)
	}
}
