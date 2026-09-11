package staffauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
)

// selfStaff is the answer to "which Staff person is calling?" for a
// route that runs before any Practice is known -- the caller's own
// `staff` row, resolved from the identity her credential names.
//
// It is a value, not an id, for the reason SessionHandler and
// UpdateWorkStateHandler each used to re-read the same row: the columns
// a pre-Practice route wants are all on it, so one resolution answers
// the question and hands over what the handler came for. A route that
// holds one of these has already passed the guard; a route that does not
// hold one has nothing to proceed with.
type selfStaff struct {
	ID                  string
	Name                string
	Email               string
	WorkState           string
	WorkStateReportedAt time.Time
	LastPracticeID      sql.NullString
	// ActivityStampStale answers Middleware's #1197 question ("does
	// last_active_at need refreshing?") with the database's own now(),
	// not Go's: this repo's dormancy tests fast-forward Postgres's clock
	// through internal/simclock's sim.now() shim (a role's search_path
	// resolves an unqualified now() to it ahead of pg_catalog.now()), so
	// staleness decided by time.Now() in Go would drift from whatever
	// day billing/dormancy.go's own now()-based reads believe it is.
	ActivityStampStale bool
	DeletedAt          sql.NullTime
}

// resolveSelf is this package's one owner of the pre-Practice
// self-resolution, and the only place either half of it is written.
//
// The two halves are one act. Setting app.current_identity_uid is what
// opens staff_self_visibility (00006) -- the policy that admits the
// caller's own row in the window where app.current_practice_id is unset,
// which is every route mountSessionRoutes registers. Resolving the row
// is what proves the identity behind the credential is a Staff person at
// all. A handler that set the variable without resolving would leave a
// policy satisfied for a row nobody checked exists; one that resolved
// without setting it would be reading through some other policy's
// window and would miss the RLS the pair is for. #1024 is what the first
// of those costs: the one handler in the family that skipped the pair
// minted a token and queued mail for a caller with no `staff` row at
// all, and nothing failed until #892 added a send-time recheck.
//
// found=false is not an error: it is the family's own refusal
// condition, which requireSelf below turns into the one answer every
// pre-Practice route gives. Two callers read it directly instead, for
// different reasons: Middleware, because a Practice-scoped route answers
// this condition differently (see the comment there), and
// FinishEnrollmentHandler, because it runs inside sessionmint's step and
// has no http.ResponseWriter to refuse through.
func resolveSelf(ctx context.Context, tx *sql.Tx, identityUID string) (selfStaff, bool, error) {
	return querySelf(ctx, tx, identityUID, false)
}

// lockSelfRow is resolveSelf with the row locked FOR UPDATE, for the one
// act that has to serialize against a concurrent copy of itself: login
// deletion, where the lock is what makes the already-deleted check a
// gate rather than a guess.
func lockSelfRow(ctx context.Context, tx *sql.Tx, identityUID string) (selfStaff, bool, error) {
	return querySelf(ctx, tx, identityUID, true)
}

func querySelf(ctx context.Context, tx *sql.Tx, identityUID string, forUpdate bool) (selfStaff, bool, error) {
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return selfStaff{}, false, fmt.Errorf("staffauth: set current identity uid: %w", err)
	}

	query := `SELECT id, name, email, work_state, work_state_reported_at, last_practice_id,
	                 last_active_at IS NULL OR last_active_at < now() - interval '1 day',
	                 deleted_at
	            FROM staff WHERE identity_uid = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}

	var self selfStaff
	err := tx.QueryRowContext(ctx, query, identityUID).Scan(
		&self.ID, &self.Name, &self.Email, &self.WorkState,
		&self.WorkStateReportedAt, &self.LastPracticeID, &self.ActivityStampStale, &self.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return selfStaff{}, false, nil
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return selfStaff{}, false, fmt.Errorf("staffauth: resolve staff: %w", err)
	}
	return self, true, nil
}

// requireSelf is the pre-Practice family's whole preamble, in one call:
// resolve the caller's own row, or write the family's refusal and stop.
// It is the ok-bool idiom RequireOwner and RequireConfirmed use, and it
// is what a route in mountSessionRoutes reaches for rather than writing
// the three steps out again.
//
// # Which status a pre-Practice route answers
//
// 404, carrying MsgNoMatchingStaffAccount. That is recorded here because
// it used to differ between copies -- the two /api/staff/mfa routes
// answered 403 for the identical condition. 404 is the right one and the
// 403s moved to it: docs/api-design.md admits a 403 only with a code
// from apierr.ForbiddenCodes, and "no matching staff account" is none of
// the three kinds those name -- not a role refusal, not a Practice-level
// condition, not a step the reader can take and retry. A bare 403
// therefore renders on the app's error page as "your role does not have
// permission", which is not what happened; the resource this caller
// asked about is her own Staff account, and it does not exist. It is
// also the answer the app's no-practice screen and its sign-in specs
// already read.
//
// selfResolvingRoutes in this package's tests is the enumerated form of
// that rule: a route mounted under /api/staff/ that neither reaches this
// function nor is excused in writing fails the build.
func requireSelf(w http.ResponseWriter, r *http.Request, tx *sql.Tx, identityUID string) (selfStaff, bool) {
	self, found, err := resolveSelf(r.Context(), tx, identityUID)
	return refuseUnlessSelf(w, self, found, err)
}

// refuseUnlessSelf is the refusal half on its own, taking a resolution's
// three return values straight through. It exists because
// lockOwnStaffRow needs the same refusal over lockSelfRow's result, and
// the status and the message belong in one place even more than the
// lookup does -- that pair is what the whole family is agreeing on.
// Splitting it this way rather than threading a forUpdate bool through
// requireSelf keeps the lock out of the name every ordinary route reads.
func refuseUnlessSelf(w http.ResponseWriter, self selfStaff, found bool, err error) (selfStaff, bool) {
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return selfStaff{}, false
	}
	if !found {
		apierr.WriteError(w, MsgNoMatchingStaffAccount, http.StatusNotFound)
		return selfStaff{}, false
	}
	return self, true
}
