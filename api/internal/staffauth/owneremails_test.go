package staffauth_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func TestOwnerEmails_NamesEveryOwnerAndNobodyElse(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Owner Emails Practice")
	testdb.SeedStaffAtPractice(t, db, practiceID, "oe-owner", []string{ownerRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, "oe-coowner", []string{ownerRole, adminRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, "oe-doula", []string{doulaRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Owner Emails Other Practice")
	testdb.SeedStaffAtPractice(t, db, otherPracticeID, "oe-elsewhere", []string{ownerRole}, "employee")

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	got, err := staffauth.OwnerEmails(t.Context(), tx, practiceID)
	if err != nil {
		t.Fatalf("OwnerEmails: %v", err)
	}
	slices.Sort(got)
	want := []string{"oe-coowner@example.com", "oe-owner@example.com"}
	if !slices.Equal(got, want) {
		t.Fatalf("OwnerEmails = %v, want %v", got, want)
	}
}
