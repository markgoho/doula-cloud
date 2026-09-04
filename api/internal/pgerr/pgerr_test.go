package pgerr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"doula-cloud/api/internal/pgerr"
)

func uniqueViolation(constraint string) error {
	return &pgconn.PgError{Code: "23505", ConstraintName: constraint}
}

func TestIsUniqueViolation(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"a unique violation", uniqueViolation("staff_identity_uid_key"), true},
		// Wrapped, because every caller reaches this through at least one
		// fmt.Errorf on the way up.
		{"a wrapped unique violation", fmt.Errorf("insert staff: %w", uniqueViolation("staff_identity_uid_key")), true},
		// A different SQLSTATE must not read as a duplicate: 23503 is a
		// foreign-key violation, which means the opposite -- the row it
		// points at is missing, not already there.
		{"a foreign key violation", &pgconn.PgError{Code: "23503"}, false},
		{"an ordinary error", errors.New("connection refused"), false},
		{"no error at all", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pgerr.IsUniqueViolation(tc.err); got != tc.want {
				t.Errorf("IsUniqueViolation(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsUniqueViolationOn(t *testing.T) {
	const slugIndex = "practice_websites_slug_key"

	for _, tc := range []struct {
		name       string
		err        error
		constraint string
		want       bool
	}{
		{"the named constraint", uniqueViolation(slugIndex), slugIndex, true},
		{"the named constraint, wrapped", fmt.Errorf("upsert: %w", uniqueViolation(slugIndex)), slugIndex, true},
		// The distinction the narrow form exists for: #443's upsert
		// collides on practice_id when it is working, and on the slug index
		// when another Practice holds that address.
		{"a different constraint", uniqueViolation("practice_websites_pkey"), slugIndex, false},
		{"a different SQLSTATE on the same constraint", &pgconn.PgError{Code: "23503", ConstraintName: slugIndex}, slugIndex, false},
		{"an ordinary error", errors.New("connection refused"), slugIndex, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pgerr.IsUniqueViolationOn(tc.err, tc.constraint); got != tc.want {
				t.Errorf("IsUniqueViolationOn(%v, %q) = %v, want %v", tc.err, tc.constraint, got, tc.want)
			}
		})
	}
}
