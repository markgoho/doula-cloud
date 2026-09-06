package staffauth

import "slices"

// Reader is an unforgeable claim of a caller's roles and employment type
// at the Practice a request is scoped to. staffauth.Middleware is the
// only production constructor: it runs the one practice_memberships
// query a request needs and places the resulting Reader on the request
// context, reached through ReaderFrom. NewReader exists alongside it so
// a test can build a Reader for an arbitrary role/employment-type
// combination directly, without seeding a Practice and a membership row
// just to prove what one of Reader's own methods decides.
type Reader struct {
	staffID        string
	roles          []string
	employmentType string
}

// NewReader constructs a Reader directly from roles and employmentType,
// with no database access -- the shape a test needs, not a handler.
// staffauth.Middleware builds its own Reader the same way, from the one
// row its request-scoped query already read.
func NewReader(staffID string, roles []string, employmentType string) Reader {
	return Reader{staffID: staffID, roles: roles, employmentType: employmentType}
}

// Has reports whether the Reader's caller holds role.
func (r Reader) Has(role string) bool {
	return slices.Contains(r.roles, role)
}

// Roles reports every role the Reader's caller holds at the resolved
// Practice, never nil -- so a caller that JSON-encodes it (the practice
// session endpoint) sends "[]" rather than "null" for a Staff member with
// no roles.
func (r Reader) Roles() []string {
	if r.roles == nil {
		return []string{}
	}
	return r.roles
}

// IsContractor reports whether the Reader's caller's membership at the
// resolved Practice is employment_type = 'contractor' -- the axis
// ADR-0008 gates ambient reach on. False for 'employee', including every
// Owner and Admin membership today (#227: employee means inside the
// business, not on a payroll).
func (r Reader) IsContractor() bool {
	return r.employmentType == "contractor"
}

// IsOwnerOrAdmin reports whether the Reader's caller holds the owner or
// admin role -- ADR-0008's other ambient-reach population, alongside an
// employee Doula. Replaces the "Has(owner) || Has(admin)" predicate that
// used to be copied at each call site that needed it.
func (r Reader) IsOwnerOrAdmin() bool {
	return r.Has("owner") || r.Has("admin")
}

// IsAmbientContractor reports whether the Reader's caller is a plain
// contractor Doula -- employment_type contractor, holding neither the
// owner nor admin role -- the population ADR-0008 confines to what she
// is attached to, rather than granting the Practice-wide ambient reach an
// owner, admin, or employee Doula all hold. Replaces the "contractor and
// not owner and not admin" predicate that used to be copied at each call
// site that needed it.
func (r Reader) IsAmbientContractor() bool {
	return r.IsContractor() && !r.IsOwnerOrAdmin()
}
