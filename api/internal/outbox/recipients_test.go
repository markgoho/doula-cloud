package outbox_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

const roleOwner = "owner"

func TestRecipients_NameTheRoleAtThatPracticeAndNobodyElse(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Recipients Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "rc-owner", []string{roleOwner}, "employee")
	coownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "rc-coowner", []string{roleOwner, "admin"}, "employee")
	adminID := testdb.SeedStaffAtPractice(t, db, practiceID, "rc-admin", []string{"admin"}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, "rc-doula", []string{"doula"}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Recipients Other Practice")
	testdb.SeedStaffAtPractice(t, db, otherPracticeID, "rc-elsewhere", []string{roleOwner}, "employee")

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	cases := []struct {
		name       string
		resolve    func() ([]string, []string, error)
		wantIDs    []string
		wantEmails []string
	}{
		{
			"owners",
			func() ([]string, []string, error) { return outbox.OwnerRecipients(t.Context(), tx, practiceID) },
			[]string{ownerID, coownerID},
			[]string{"rc-owner@example.com", "rc-coowner@example.com"},
		},
		{
			"owners and admins",
			func() ([]string, []string, error) { return outbox.OwnerAndAdminRecipients(t.Context(), tx, practiceID) },
			[]string{ownerID, coownerID, adminID},
			[]string{"rc-owner@example.com", "rc-coowner@example.com", "rc-admin@example.com"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ids, emails, err := tc.resolve()
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			wantIDs := slices.Sorted(slices.Values(tc.wantIDs))
			if !slices.Equal(ids, wantIDs) {
				t.Fatalf("ids = %v, want %v ordered by id", ids, wantIDs)
			}
			if len(emails) != len(tc.wantEmails) {
				t.Fatalf("emails = %v, want %v", emails, tc.wantEmails)
			}
			for _, e := range tc.wantEmails {
				if !slices.Contains(emails, e) {
					t.Fatalf("emails = %v, want %q among them", emails, e)
				}
			}
		})
	}
}
