package testdb_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestSeedStaffAtPractice proves SeedStaffAtPractice's Staff row and
// practice_memberships row land with the roles and employment type
// passed in -- every package composing seed helpers on top of this one
// relies on that.
func TestSeedStaffAtPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Test Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "seed-test-staff", []string{"owner", "doula"}, "contractor")

	var name, email string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT name, email FROM staff WHERE id = $1`, staffID,
	).Scan(&name, &email); err != nil {
		t.Fatalf("read seeded staff: %v", err)
	}
	if name != "Test Staff seed-test-staff" {
		t.Fatalf("name = %q, want derived from identityUID", name)
	}
	if email != "seed-test-staff@example.com" {
		t.Fatalf("email = %q, want derived from identityUID", email)
	}

	var roles, employmentType string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT roles::text, employment_type FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&roles, &employmentType); err != nil {
		t.Fatalf("read seeded membership: %v", err)
	}
	if roles != "{owner,doula}" {
		t.Fatalf("roles = %q, want {owner,doula}", roles)
	}
	if employmentType != "contractor" {
		t.Fatalf("employment_type = %q, want contractor", employmentType)
	}
}

// TestSeedNamedStaffAtPractice proves the display name passed in lands on
// the Staff row unchanged, for a test that asserts a specific name comes
// back.
func TestSeedNamedStaffAtPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Named Seed Test Practice")
	staffID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "named-seed-test-staff", "Jamie Doula", []string{"doula"}, "employee")

	var name string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT name FROM staff WHERE id = $1`, staffID,
	).Scan(&name); err != nil {
		t.Fatalf("read seeded staff: %v", err)
	}
	if name != "Jamie Doula" {
		t.Fatalf("name = %q, want %q", name, "Jamie Doula")
	}
}

// TestSeedPortalAccount proves the portal_accounts row lands with the
// identifier and sign-in address passed in -- every package seeding an
// accepted client_portal_users row relies on this existing first, or the
// identity_uid foreign key (#616) refuses the insert.
func TestSeedPortalAccount(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedPortalAccount(t, db, "portal_seed-test", "seed-test@example.com")

	var signInAddress string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT sign_in_address FROM portal_accounts WHERE identifier = $1`, "portal_seed-test",
	).Scan(&signInAddress); err != nil {
		t.Fatalf("read seeded portal account: %v", err)
	}
	if signInAddress != "seed-test@example.com" {
		t.Fatalf("sign_in_address = %q, want %q", signInAddress, "seed-test@example.com")
	}
}

// TestAttachPortalUser proves the client_portal_users row lands pointed
// at the given Portal Account and Client -- the shape a multi-Practice
// Portal Account's second (and later) row takes (#309, ADR-0015).
func TestAttachPortalUser(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Attach Test Practice")
	testdb.SeedPortalAccount(t, db, "portal_attach-test", "attach-test@example.com")

	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Attach Test Client', 'client@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}

	testdb.AttachPortalUser(t, db, "portal_attach-test", clientID)

	var identityUID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT identity_uid FROM client_portal_users WHERE client_id = $1`, clientID,
	).Scan(&identityUID); err != nil {
		t.Fatalf("read attached client_portal_users row: %v", err)
	}
	if identityUID != "portal_attach-test" {
		t.Fatalf("identity_uid = %q, want %q", identityUID, "portal_attach-test")
	}
}
