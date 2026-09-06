package testdb

import (
	"strings"
	"testing"
)

// SeedPractice inserts a bare Practice row using the superuser Admin
// connection.
func SeedPractice(t *testing.T, db *DB, name string) (practiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ($1) RETURNING id`, name,
	).Scan(&practiceID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed practice %q: %v", name, err)
	}
	return practiceID
}

// SeedStaffAtPractice inserts a Staff row bound to identityUID and a
// practice_memberships row granting roles at practiceID as
// employmentType, using the superuser Admin connection (which bypasses
// RLS) so fixture setup isn't gated by the policies under test. The Staff
// row is named "Test Staff "+identityUID and emailed
// identityUID+"@example.com" -- both derived from the one value every
// caller already picks uniquely -- so two Staff seeded in the same test
// never collide.
func SeedStaffAtPractice(t *testing.T, db *DB, practiceID, identityUID string, roles []string, employmentType string) (staffID string) {
	t.Helper()
	return SeedNamedStaffAtPractice(t, db, practiceID, identityUID, "Test Staff "+identityUID, roles, employmentType)
}

// SeedNamedStaffAtPractice is SeedStaffAtPractice with an explicit
// display name, for a test that asserts a specific name comes back (e.g.
// a Message's sender).
func SeedNamedStaffAtPractice(t *testing.T, db *DB, practiceID, identityUID, name string, roles []string, employmentType string) (staffID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, $2, $3, 'NY') RETURNING id`,
		identityUID, name, identityUID+"@example.com",
	).Scan(&staffID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed staff: %v", err)
	}

	literal := "{" + strings.Join(roles, ",") + "}"
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, $3::practice_role[], $4)`,
		practiceID, staffID, literal, employmentType,
	); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed membership: %v", err)
	}
	return staffID
}

// SeedPortalAccount inserts a portal_accounts row using the superuser
// Admin connection. client_portal_users.identity_uid (#616) carries a
// foreign key to portal_accounts.identifier, so any fixture that seeds a
// client_portal_users row with identity_uid set needs a matching row
// here first, or the insert fails.
func SeedPortalAccount(t *testing.T, db *DB, identifier, signInAddress string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO portal_accounts (identifier, sign_in_address) VALUES ($1, $2)`,
		identifier, signInAddress,
	); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed portal account %q: %v", identifier, err)
	}
}

// AttachPortalUser links an already-seeded Portal Account (identifier) to
// clientID via a new client_portal_users row, using the superuser Admin
// connection. For the second (and later) client_portal_users row a
// multi-Practice Portal Account holds (#309, ADR-0015) -- the first row
// is SeedPortalAccount's own caller's job, since that call also mints the
// Portal Account itself and a second SeedPortalAccount call for the same
// identifier would collide on portal_accounts' own primary key.
func AttachPortalUser(t *testing.T, db *DB, identifier, clientID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		identifier, clientID,
	); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: attach portal user %q to client %q: %v", identifier, clientID, err)
	}
}
