package fixture

// RoleCase is one Staff member who may or may not do a thing, for a test
// that ranges over ADR-0015's role table.
type RoleCase struct {
	Name string
	Opts []StaffOption
}

// The rows of ADR-0015's role table a test ranges over.
var (
	OwnerCase           = RoleCase{"owner", []StaffOption{Owner()}}
	AdminCase           = RoleCase{"admin", []StaffOption{Admin()}}
	EmployeeDoulaCase   = RoleCase{"employee doula", []StaffOption{Doula()}}
	ContractorDoulaCase = RoleCase{"contractor doula", []StaffOption{Doula(), Contractor()}}
	NoRoleCase          = RoleCase{"no role", []StaffOption{NoRole()}}
	OwnerAndDoulaCase   = RoleCase{"owner and doula", []StaffOption{Owner(), Doula()}}
)
