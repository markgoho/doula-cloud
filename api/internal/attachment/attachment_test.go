package attachment_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/attachment"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	doulaRole      = "doula"
	ownerRole      = "owner"
	employeeType   = "employee"
	contractorType = "contractor"
)

// TestWhyNotAttachable is the whole rule: a Member here, with the Doula
// role, an employee. The checks run in that order, so a person who is
// neither a Doula nor an employee is told about the role.
func TestWhyNotAttachable(t *testing.T) {
	cases := []struct {
		name string
		m    attachment.Membership
		want attachment.NotAttachable
	}{
		{"an employee doula", attachment.Membership{Exists: true, IsDoula: true, EmploymentType: employeeType}, attachment.Attachable},
		{"no membership here", attachment.Membership{}, attachment.NotAtPractice},
		{"staff with no doula role", attachment.Membership{Exists: true, EmploymentType: employeeType}, attachment.NotADoula},
		{"a contractor doula", attachment.Membership{Exists: true, IsDoula: true, EmploymentType: contractorType}, attachment.IsContractor},
		{"a contractor with no doula role", attachment.Membership{Exists: true, EmploymentType: contractorType}, attachment.NotADoula},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.WhyNotAttachable(); got != tc.want {
				t.Fatalf("WhyNotAttachable() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestOfReader reads the caller's own Membership off the Reader the
// middleware resolved: the role and the Employment type, with no query.
// An Owner who is also an employee Doula is attachable, and an Owner who
// is a contractor Doula is not.
func TestOfReader(t *testing.T) {
	cases := []struct {
		name           string
		roles          []string
		employmentType string
		want           attachment.NotAttachable
	}{
		{"owner who is an employee doula", []string{ownerRole, doulaRole}, employeeType, attachment.Attachable},
		{"owner with no doula role", []string{ownerRole}, employeeType, attachment.NotADoula},
		{"owner who is a contractor doula", []string{ownerRole, doulaRole}, contractorType, attachment.IsContractor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := attachment.OfReader(staffauth.NewReader("staff-id", tc.roles, tc.employmentType))
			if !m.Exists {
				t.Fatal("Exists = false, want true: the middleware admits only a Member")
			}
			if got := m.WhyNotAttachable(); got != tc.want {
				t.Fatalf("WhyNotAttachable() = %q, want %q", got, tc.want)
			}
		})
	}
}

// beginPracticeTx opens an app-role transaction scoped to practiceID the
// way staffauth.Middleware scopes a request's own, so RLS behaves here as
// it does behind a route.
func beginPracticeTx(t *testing.T, db *testdb.DB, practiceID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	return tx
}

// TestReadMembership reads another person's Membership, and answers the
// zero Membership (no name, not attachable) for a person who holds none
// at this Practice.
func TestReadMembership(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Read Membership")
	doulaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "read-membership-doula", "Hana Kim", []string{doulaRole}, contractorType)
	otherPractice := testdb.SeedPractice(t, db, "Another Practice")
	strangerID := testdb.SeedStaffAtPractice(t, db, otherPractice, "read-membership-stranger", []string{doulaRole}, employeeType)
	tx := beginPracticeTx(t, db, practiceID)

	got, err := attachment.ReadMembership(t.Context(), tx, practiceID, doulaID)
	if err != nil {
		t.Fatalf("ReadMembership: %v", err)
	}
	want := attachment.Membership{Exists: true, IsDoula: true, EmploymentType: contractorType, Name: "Hana Kim"}
	if got != want {
		t.Fatalf("membership = %+v, want %+v", got, want)
	}

	stranger, err := attachment.ReadMembership(t.Context(), tx, practiceID, strangerID)
	if err != nil {
		t.Fatalf("ReadMembership (stranger): %v", err)
	}
	if stranger != (attachment.Membership{}) {
		t.Fatalf("stranger membership = %+v, want the zero Membership", stranger)
	}
	if reason := stranger.WhyNotAttachable(); reason != attachment.NotAtPractice {
		t.Fatalf("stranger reason = %q, want %q", reason, attachment.NotAtPractice)
	}
}

// TestGrant writes the two records together: a granted Attachment with
// no fee and the decider as attached_by, and one doula_attached activity
// entry whose actor is the decider and whose diff names the Doula.
func TestGrant(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Grant")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "grant-owner", []string{ownerRole}, employeeType)
	doulaID := testdb.SeedStaffAtPractice(t, db, practiceID, "grant-doula", []string{doulaRole}, employeeType)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	tx := beginPracticeTx(t, db, practiceID)

	if err := attachment.Grant(t.Context(), tx, practiceID, engagementID, doulaID, ownerID); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	var origin, attachedBy string
	var feeCents *int64
	var feeTerms *string
	if err := tx.QueryRowContext(t.Context(),
		`SELECT origin::text, attached_by::text, fee_amount_cents, fee_terms
		   FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NULL`,
		engagementID, doulaID,
	).Scan(&origin, &attachedBy, &feeCents, &feeTerms); err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if origin != "granted" || attachedBy != ownerID || feeCents != nil || feeTerms != nil {
		t.Fatalf("attachment = origin %q, attached_by %q, fee %v %v; want granted, the decider, no fee", origin, attachedBy, feeCents, feeTerms)
	}

	var entries int
	var actor, attached string
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*), max(actor_staff_id::text), max(diff ->> 'attachedStaffId') FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'doula_attached'`,
		engagementID,
	).Scan(&entries, &actor, &attached); err != nil {
		t.Fatalf("read activity: %v", err)
	}
	if entries != 1 || actor != ownerID || attached != doulaID {
		t.Fatalf("activity = %d entries, actor %q, attached %q; want one, the decider, the Doula", entries, actor, attached)
	}
}
