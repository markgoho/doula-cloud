package staffauth

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
)

// Reader is an unforgeable claim of a caller's roles and employment type
// for the current request. Construct one only via ResolveReader -- a
// query function that wants ADR-0008-gated data takes a Reader parameter
// instead of a bare context/tx, so calling it without having gone
// through role resolution is a compile error, not a missed check.
type Reader struct {
	staffID        string
	roles          []string
	employmentType string
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

// ResolveReader loads the caller's roles and employment_type for
// practiceID/staffID -- the one place a Reader can be constructed. Must
// run downstream of Middleware, which is what makes practiceID/staffID
// trustworthy inputs here.
func ResolveReader(ctx context.Context, tx *sql.Tx, practiceID, staffID string) (Reader, error) {
	var rolesCSV, employmentType string
	err := tx.QueryRowContext(ctx,
		`SELECT array_to_string(roles, ','), employment_type::text FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&rolesCSV, &employmentType)
	// coverage:ignore reason: DB query failure, not exercised by unit tests -- Middleware already confirmed this membership exists
	if err != nil {
		return Reader{}, fmt.Errorf("staffauth: resolve reader: %w", err)
	}
	var roles []string
	if rolesCSV != "" {
		roles = splitCSV(rolesCSV)
	}
	return Reader{staffID: staffID, roles: roles, employmentType: employmentType}, nil
}

// splitCSV splits a comma-joined string, avoiding a strings import for
// this single trivial use.
func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}
