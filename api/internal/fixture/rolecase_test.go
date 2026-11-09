package fixture_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

const (
	employeeType   = "employee"
	contractorType = "contractor"
)

func TestRoleCases_SeedTheStaffMemberTheirNameSays(t *testing.T) {
	cases := []struct {
		role           fixture.RoleCase
		wantRoles      []string
		wantEmployment string
	}{
		{fixture.OwnerCase, []string{fixture.RoleOwner}, employeeType},
		{fixture.AdminCase, []string{fixture.RoleAdmin}, employeeType},
		{fixture.EmployeeDoulaCase, []string{fixture.RoleDoula}, employeeType},
		{fixture.ContractorDoulaCase, []string{fixture.RoleDoula}, contractorType},
		{fixture.NoRoleCase, []string{}, employeeType},
		{fixture.OwnerAndDoulaCase, []string{fixture.RoleOwner, fixture.RoleDoula}, employeeType},
	}
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	for _, tc := range cases {
		t.Run(tc.role.Name, func(t *testing.T) {
			s := fixture.Staff(t, db, p, tc.role.Opts...)
			if !slices.Equal(s.Roles, tc.wantRoles) || s.Employment != tc.wantEmployment {
				t.Fatalf("roles = %v, employment = %q, want %v and %q", s.Roles, s.Employment, tc.wantRoles, tc.wantEmployment)
			}
		})
	}
}
