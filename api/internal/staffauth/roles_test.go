package staffauth_test

import (
	"testing"

	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestRoles exercises staffauth.Roles directly -- the function main.go's
// practiceSessionHandler calls so the frontend can gate Owner-only UI
// (like the invite link) on the caller's actual roles.
func TestRoles(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Roles Fn Test")
	doulaID := seedStaff(t, db, "roles-fn-doula")
	seedMembership(t, db, practiceID, doulaID) // seeds '{doula}'

	zeroRoleID := seedStaff(t, db, "roles-fn-zero")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{}', 'employee')`,
		practiceID, zeroRoleID,
	); err != nil {
		t.Fatalf("seed zero-role membership: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	roles, err := staffauth.Roles(t.Context(), tx, practiceID, doulaID)
	if err != nil {
		t.Fatalf("Roles(doula): %v", err)
	}
	if len(roles) != 1 || roles[0] != doulaRole {
		t.Fatalf("Roles(doula) = %v, want [doula]", roles)
	}

	zeroRoles, err := staffauth.Roles(t.Context(), tx, practiceID, zeroRoleID)
	if err != nil {
		t.Fatalf("Roles(zero-role): %v", err)
	}
	if len(zeroRoles) != 0 {
		t.Fatalf("Roles(zero-role) = %v, want empty", zeroRoles)
	}
}
