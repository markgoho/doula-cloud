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

func seedAccessEngagement(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Access Test Client', 'access-client@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	var engagementID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return engagementID
}

func seedAttachment(t *testing.T, db *testdb.DB, engagementID, staffID, origin string, ended bool) {
	t.Helper()
	endedAt := "NULL"
	if ended {
		endedAt = "now()"
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, ended_at)
		 VALUES ($1, $2, $3::attachment_origin, $2, `+endedAt+`)`,
		engagementID, staffID, origin,
	); err != nil {
		t.Fatalf("seed attachment (origin=%s, ended=%v): %v", origin, ended, err)
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
	engagementID := seedAccessEngagement(t, db, practiceID)

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
	seedAttachment(t, db, engagementID, accruedContractorID, "accrued", false)

	endedContractorID := seedStaff(t, db, "access-contractor-ended")
	seedContractorMembership(t, db, practiceID, endedContractorID)
	seedAttachment(t, db, engagementID, endedContractorID, "granted", true)

	grantedContractorID := seedStaff(t, db, "access-contractor-granted")
	seedContractorMembership(t, db, practiceID, grantedContractorID)
	seedAttachment(t, db, engagementID, grantedContractorID, "granted", false)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}

	cases := []struct {
		name    string
		staffID string
		want    bool
	}{
		{ownerRole, ownerID, true},
		{adminRole, adminID, true},
		{"employee doula, ambient", employeeID, true},
		{"contractor, no attachment", unattachedContractorID, false},
		{"contractor, accrued only", accruedContractorID, false},
		{"contractor, granted but ended", endedContractorID, false},
		{"contractor, granted and open", grantedContractorID, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, err := staffauth.ResolveReader(t.Context(), tx, practiceID, tc.staffID)
			if err != nil {
				t.Fatalf("ResolveReader: %v", err)
			}
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
	engagementID := seedAccessEngagement(t, db, practiceID)
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
	seedAttachment(t, db, engagementID, attachedContractorID, "granted", false)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}

	cases := []struct {
		name    string
		staffID string
		want    bool
	}{
		{ownerRole, ownerID, true},
		{"contractor, no attachment", unattachedContractorID, false},
		{"contractor, granted and open", attachedContractorID, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, err := staffauth.ResolveReader(t.Context(), tx, practiceID, tc.staffID)
			if err != nil {
				t.Fatalf("ResolveReader: %v", err)
			}
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

// TestReader_IsContractor confirms the axis ResolveReader carries
// besides roles: an employee reader (Owner and Admin included, #227's
// "employee means inside the business") is not a contractor. It also
// covers Reader.Roles() -- the accessor #501's practiceSessionHandler
// reads to hand the frontend both axes off one Reader -- on both a
// populated and an empty roles array, since ResolveReader only sets its
// private slice when the DB's CSV is non-empty.
func TestReader_IsContractor(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Employment Type Test Practice")

	employeeID := seedStaff(t, db, "employment-employee")
	seedMembershipWithRoles(t, db, practiceID, employeeID, "{doula}")

	contractorID := seedStaff(t, db, "employment-contractor")
	seedContractorMembership(t, db, practiceID, contractorID)

	zeroRoleID := seedStaff(t, db, "employment-zero-role")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{}', 'employee')`,
		practiceID, zeroRoleID,
	); err != nil {
		t.Fatalf("seed zero-role membership: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}

	employeeReader, err := staffauth.ResolveReader(t.Context(), tx, practiceID, employeeID)
	if err != nil {
		t.Fatalf("ResolveReader employee: %v", err)
	}
	if employeeReader.IsContractor() {
		t.Fatal("employee reader reported IsContractor() = true")
	}
	if roles := employeeReader.Roles(); len(roles) != 1 || roles[0] != doulaRole {
		t.Fatalf("employeeReader.Roles() = %v, want [doula]", roles)
	}

	contractorReader, err := staffauth.ResolveReader(t.Context(), tx, practiceID, contractorID)
	if err != nil {
		t.Fatalf("ResolveReader contractor: %v", err)
	}
	if !contractorReader.IsContractor() {
		t.Fatal("contractor reader reported IsContractor() = false")
	}

	zeroRoleReader, err := staffauth.ResolveReader(t.Context(), tx, practiceID, zeroRoleID)
	if err != nil {
		t.Fatalf("ResolveReader zero-role: %v", err)
	}
	if roles := zeroRoleReader.Roles(); len(roles) != 0 {
		t.Fatalf("zeroRoleReader.Roles() = %v, want empty", roles)
	}
}
