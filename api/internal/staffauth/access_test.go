package staffauth_test

import (
	"testing"

	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func seedContractorMembership(t *testing.T, db *testdb.DB, practiceID, staffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}', 'contractor')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed contractor membership: %v", err)
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
	practiceID := seedPractice(t, db, "Access Test Practice")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	ownerID := seedStaff(t, db, "access-owner")
	seedMembershipWithRoles(t, db, practiceID, ownerID, "{owner}")

	adminID := seedStaff(t, db, "access-admin")
	seedMembershipWithRoles(t, db, practiceID, adminID, "{admin}")

	employeeID := seedStaff(t, db, "access-employee")
	seedMembershipWithRoles(t, db, practiceID, employeeID, "{doula}")

	unattachedContractorID := seedStaff(t, db, "access-contractor-unattached")
	seedContractorMembership(t, db, practiceID, unattachedContractorID)

	accruedContractorID := seedStaff(t, db, "access-contractor-accrued")
	seedContractorMembership(t, db, practiceID, accruedContractorID)
	testdb.SeedAttachment(t, db, engagementID, accruedContractorID, "accrued", false)

	endedContractorID := seedStaff(t, db, "access-contractor-ended")
	seedContractorMembership(t, db, practiceID, endedContractorID)
	testdb.SeedAttachment(t, db, engagementID, endedContractorID, "granted", true)

	grantedContractorID := seedStaff(t, db, "access-contractor-granted")
	seedContractorMembership(t, db, practiceID, grantedContractorID)
	testdb.SeedAttachment(t, db, engagementID, grantedContractorID, "granted", false)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}

	cases := []struct {
		name           string
		staffID        string
		roles          []string
		employmentType string
		want           bool
	}{
		{ownerRole, ownerID, []string{ownerRole}, employeeType, true},
		{adminRole, adminID, []string{adminRole}, employeeType, true},
		{"employee doula, ambient", employeeID, []string{doulaRole}, employeeType, true},
		{"contractor, no attachment", unattachedContractorID, []string{doulaRole}, contractorType, false},
		{"contractor, accrued only", accruedContractorID, []string{doulaRole}, contractorType, false},
		{"contractor, granted but ended", endedContractorID, []string{doulaRole}, contractorType, false},
		{"contractor, granted and open", grantedContractorID, []string{doulaRole}, contractorType, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := staffauth.NewReader(tc.staffID, tc.roles, tc.employmentType)
			got, err := reader.CanAccessEngagement(t.Context(), tx, engagementID)
			if err != nil {
				t.Fatalf("CanAccessEngagement: %v", err)
			}
			if got != tc.want {
				t.Fatalf("CanAccessEngagement() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestReader_CanAccessClient mirrors TestReader_CanAccessEngagement for
// the Client-scoped ADR-0017 rule: attachment reaches through any of the
// Client's Engagements, not just one named one.
func TestReader_CanAccessClient(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Client Access Test Practice")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT client_id FROM engagements WHERE id = $1`, engagementID).Scan(&clientID); err != nil {
		t.Fatalf("read client id: %v", err)
	}

	ownerID := seedStaff(t, db, "client-access-owner")
	seedMembershipWithRoles(t, db, practiceID, ownerID, "{owner}")

	unattachedContractorID := seedStaff(t, db, "client-access-contractor-unattached")
	seedContractorMembership(t, db, practiceID, unattachedContractorID)

	attachedContractorID := seedStaff(t, db, "client-access-contractor-attached")
	seedContractorMembership(t, db, practiceID, attachedContractorID)
	testdb.SeedAttachment(t, db, engagementID, attachedContractorID, "granted", false)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}

	cases := []struct {
		name           string
		staffID        string
		roles          []string
		employmentType string
		want           bool
	}{
		{ownerRole, ownerID, []string{ownerRole}, employeeType, true},
		{"contractor, no attachment", unattachedContractorID, []string{doulaRole}, contractorType, false},
		{"contractor, granted and open", attachedContractorID, []string{doulaRole}, contractorType, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := staffauth.NewReader(tc.staffID, tc.roles, tc.employmentType)
			got, err := reader.CanAccessClient(t.Context(), tx, clientID)
			if err != nil {
				t.Fatalf("CanAccessClient: %v", err)
			}
			if got != tc.want {
				t.Fatalf("CanAccessClient() = %v, want %v", got, tc.want)
			}
		})
	}
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
