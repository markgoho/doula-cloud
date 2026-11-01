package staffauth_test

import (
	"testing"

	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// #2434: the read POST /api/session hands back, the same one
// GET /api/staff/session answers.
func TestResolveSession_ReadsTheCallersStaffSession(t *testing.T) {
	db := testdb.New(t)
	const uid = "uid-resolve-session"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{"owner"}, "employee")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	resp, found, err := staffauth.ResolveSession(t.Context(), tx, uid, true)
	if err != nil {
		t.Fatalf("ResolveSession: %v", err)
	}
	if !found {
		t.Fatal("found = false, want the seeded Staff row")
	}
	if resp.StaffID != staffID {
		t.Errorf("staffId = %q, want %q", resp.StaffID, staffID)
	}
	if len(resp.Memberships) != 1 || resp.Memberships[0].PracticeID != practiceID {
		t.Errorf("memberships = %+v, want one at %s", resp.Memberships, practiceID)
	}
	if !resp.SecondFactor {
		t.Error("secondFactor = false, want the value passed in")
	}
}

func TestResolveSession_FindsNothingForAnIdentityWithNoStaffRow(t *testing.T) {
	db := testdb.New(t)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, found, err := staffauth.ResolveSession(t.Context(), tx, "uid-nobody", false)
	if err != nil {
		t.Fatalf("ResolveSession: %v", err)
	}
	if found {
		t.Error("found = true, want false for an identity with no Staff row")
	}
}
