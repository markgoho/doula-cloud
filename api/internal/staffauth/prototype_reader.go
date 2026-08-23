// PROTOTYPE -- throwaway. Shape B for #231: a check pushed down to the
// query layer, so an ungated query cannot compile.
//
// Reader is proof a caller's roles were actually resolved from the DB --
// its fields are unexported, so only ResolveReader (this package) can
// construct one. A query function that wants gated data takes a Reader
// parameter instead of a bare context/tx, so calling it without having
// gone through role resolution is a compile error, not a missed check.
//
// This alone still only gates whole functions (same shape as A, moved
// down one layer). What makes it different is what a caller does INSIDE
// the function: it can return one of several Go types depending on what
// Reader proves, e.g. contracts.ContractScope vs contracts.ContractFull.
// The handler doesn't decide what to redact -- it just encodes whatever
// type it got back. See contracts/prototype_view.go for the Contract
// case this is built to answer, and prototype_reader_test.go for why a
// brand-new endpoint can still bypass this (the finding this shape
// surfaces).
//
// employment_type does not exist in the schema yet -- #227 decided the
// model but its migration is still fog on map #225 ("All build work after
// ADR-0007"). So Reader carries only practice_role, the one axis that is
// real today; the contractor cell of ADR-0006's table is out of reach for
// this prototype and left to the ADR-0007 build phase.
package staffauth

import (
	"context"
	"database/sql"
	"fmt"
)

// Reader is an unforgeable claim of a caller's roles for the current
// request. Construct one only via ResolveReader.
type Reader struct {
	roles []string
}

// Has reports whether the Reader's caller holds role.
func (r Reader) Has(role string) bool {
	for _, have := range r.roles {
		if have == role {
			return true
		}
	}
	return false
}

// ResolveReader loads the caller's roles for practiceID/staffID -- the one
// place a Reader can be constructed. Must run downstream of Middleware,
// which is what makes practiceID/staffID trustworthy inputs here.
func ResolveReader(ctx context.Context, tx *sql.Tx, practiceID, staffID string) (Reader, error) {
	var rolesCSV string
	err := tx.QueryRowContext(ctx,
		`SELECT array_to_string(roles, ',') FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&rolesCSV)
	if err != nil {
		return Reader{}, fmt.Errorf("staffauth: resolve reader: %w", err)
	}
	var roles []string
	if rolesCSV != "" {
		roles = splitCSV(rolesCSV)
	}
	return Reader{roles: roles}, nil
}

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
