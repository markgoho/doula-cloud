// Package pgerr names the Postgres error conditions this codebase acts
// on, so a SQLSTATE code is written once rather than at every call site
// that has to tell "somebody already has this" apart from "the database
// broke".
//
// It exists because the same four lines had been copied five times over
// -- staffauth, plans, contracts, engagementrequest and portalinvite each
// carried an unexported isUniqueViolation, every one of them commented
// "mirroring" whichever package the author had copied it from. A chain of
// copies is what a Postgres detail with no home looks like: staffauth
// wrote the first, kept it unexported because nothing else needed it yet,
// and each package after that found the comment instead of the function.
//
// A leaf package on purpose -- it imports errors and pgconn and nothing
// of this codebase's own, so any package that touches the database can
// depend on it without a cycle.
package pgerr

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolation is Postgres's SQLSTATE for a unique constraint or index
// being violated: https://www.postgresql.org/docs/current/errcodes-appendix.html
const uniqueViolation = "23505"

// IsUniqueViolation reports whether err is a Postgres unique violation --
// the error an INSERT gets when the row it is adding already exists.
//
// Callers use it to turn a race into an ordinary answer: a second signup
// on one address is a 409 rather than a 500, and a concurrent
// insert-if-absent is a retry rather than a failure.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

// IsUniqueViolationOn reports whether err is a unique violation on one
// named constraint.
//
// The narrower question, for a statement that can collide on more than
// one index and means different things by each. #443's Practice Page
// upsert is the case: a conflict on the slug index is another Practice
// holding that address, which the caller retries with the next candidate,
// while a conflict on practice_id is the upsert doing its job.
func IsUniqueViolationOn(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == constraint
}
